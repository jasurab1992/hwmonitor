//go:build windows

package collectors

// CPU temperature via Ring0 — talks directly to WinRing0 kernel driver
// without the DLL layer. Requires WinRing0x64.sys installed as a service
// (or WinRing0x64.dll present — DLL auto-installs the driver).
//
// Driver source: https://github.com/GermanAizek/WinRing0 (GPL-3.0)
// Device path:   \\.\WinRing0_1_2_0
//
// Falls back → MSAcpi_ThermalZoneTemperature WMI
//           → Win32_PerfFormattedData_Counters_ThermalZoneInformation

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows"
)

var (
	modKernel32             = syscall.NewLazyDLL("kernel32.dll")
	procSetThreadAffinityMask = modKernel32.NewProc("SetThreadAffinityMask")
)

// ─── WinRing0 IOCTL codes ─────────────────────────────────────────────────
// CTL_CODE(OLS_TYPE=40000, func, METHOD_BUFFERED=0, FILE_ANY_ACCESS=0)
// = (40000 << 16) | (func << 2)
const (
	olsDevicePath   = `\\.\WinRing0_1_2_0`
	ioctlReadMsr    = 0x9C402084 // CTL_CODE(40000, 0x821, 0, 0)
	ioctlReadIOPort = 0x9C402CCC // CTL_CODE(40000, 0x831, 0, 0)  (unused here)
)

// msrIn is the input buffer for IOCTL_OLS_READ_MSR.
type msrIn struct {
	Register uint32
}

// msrOut is the output buffer: low 32 bits = EAX, high 32 bits = EDX.
type msrOut struct {
	Value uint64
}

// ─── Intel / AMD MSR addresses ────────────────────────────────────────────
const (
	msrIA32ThermStatus = 0x19C     // Intel: per-LP temp offset below TjMax
	msrTempTarget      = 0x1A2     // Intel: bits 23:16 = TjMax
	msrAMDZenTemp      = 0xC0010299 // AMD Zen: bits 31:21 = Tctl*8 (0.125°C)
)

type cpuVendor int

const (
	vendorUnknown cpuVendor = iota
	vendorIntel
	vendorAMD
)

// ─── Collector ────────────────────────────────────────────────────────────

type CPUTempCollector struct {
	handle  windows.Handle
	ring0OK bool
	vendor  cpuVendor
	tjmax   uint32
}

func NewCPUTempCollector() *CPUTempCollector {
	c := &CPUTempCollector{}
	c.ring0OK = c.openDriver()
	return c
}

func (c *CPUTempCollector) Name() string { return "cpu_temp" }

// openDriver opens \\.\WinRing0_1_2_0 and reads TjMax for Intel.
func (c *CPUTempCollector) openDriver() bool {
	path, err := syscall.UTF16PtrFromString(olsDevicePath)
	if err != nil {
		return false
	}
	h, err := windows.CreateFile(
		path,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		0, nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		log.Printf("cpu_temp: WinRing0 driver not found (%v) — ACPI fallback", err)
		return false
	}

	c.handle = h
	c.vendor = detectCPUVendor()

	if c.vendor == vendorIntel {
		c.tjmax = c.readTjMax()
		log.Printf("cpu_temp: Ring0 ready — Intel, TjMax=%d°C", c.tjmax)
	} else if c.vendor == vendorAMD {
		log.Printf("cpu_temp: Ring0 ready — AMD Zen")
	}
	return true
}

// readMSR sends IOCTL_OLS_READ_MSR to the WinRing0 driver.
// Returns (eax, edx, ok).
func (c *CPUTempCollector) readMSR(reg uint32) (eax, edx uint32, ok bool) {
	in := msrIn{Register: reg}
	var out msrOut
	var returned uint32
	err := windows.DeviceIoControl(
		c.handle,
		ioctlReadMsr,
		(*byte)(unsafe.Pointer(&in)),
		uint32(unsafe.Sizeof(in)),
		(*byte)(unsafe.Pointer(&out)),
		uint32(unsafe.Sizeof(out)),
		&returned,
		nil,
	)
	if err != nil {
		return 0, 0, false
	}
	return uint32(out.Value), uint32(out.Value >> 32), true
}

// readMSROnCore pins the current goroutine to the given logical CPU,
// reads the MSR, then restores the original affinity.
func (c *CPUTempCollector) readMSROnCore(reg uint32, logicalCPU int) (eax, edx uint32, ok bool) {
	if logicalCPU >= 64 {
		return 0, 0, false
	}

	// Lock goroutine to this OS thread so SetThreadAffinityMask has effect.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	mask := uintptr(1) << logicalCPU
	oldMask, _, _ := procSetThreadAffinityMask.Call(uintptr(windows.CurrentThread()), mask)
	if oldMask == 0 {
		return 0, 0, false
	}
	defer procSetThreadAffinityMask.Call(uintptr(windows.CurrentThread()), oldMask)

	return c.readMSR(reg)
}

func (c *CPUTempCollector) readTjMax() uint32 {
	eax, _, ok := c.readMSROnCore(msrTempTarget, 0)
	if ok {
		tj := (eax >> 16) & 0xFF
		if tj >= 80 && tj <= 115 {
			return tj
		}
	}
	return 100 // safe default for modern Intel
}

// ─── Collect ──────────────────────────────────────────────────────────────

func (c *CPUTempCollector) Collect() ([]Metric, error) {
	if c.ring0OK {
		switch c.vendor {
		case vendorIntel:
			return c.collectIntel()
		case vendorAMD:
			return c.collectAMD()
		}
	}
	return c.collectAcpi()
}

// collectIntel reads IA32_THERM_STATUS on each logical CPU.
// Temperature = TjMax − DigitalReadout (bits 22:16 of EAX).
func (c *CPUTempCollector) collectIntel() ([]Metric, error) {
	numCPU := runtime.NumCPU()
	var metrics []Metric
	var maxTemp float64

	for i := 0; i < numCPU && i < 64; i++ {
		eax, _, ok := c.readMSROnCore(msrIA32ThermStatus, i)
		if !ok {
			continue
		}
		// Bit 31 = reading valid
		if (eax>>31)&1 == 0 {
			continue
		}
		dr := float64((eax >> 16) & 0x7F)
		temp := float64(c.tjmax) - dr
		if temp > maxTemp {
			maxTemp = temp
		}
		metrics = append(metrics, Metric{
			Name:  "cpu_temp_celsius",
			Value: temp,
			Labels: map[string]string{
				"zone":   fmt.Sprintf("Core #%d", i),
				"source": "Ring0",
			},
		})
	}

	if len(metrics) > 0 {
		// CPU Package = hottest core
		metrics = append([]Metric{{
			Name:  "cpu_temp_celsius",
			Value: maxTemp,
			Labels: map[string]string{
				"zone":   "CPU Package",
				"source": "Ring0",
			},
		}}, metrics...)
	}
	return metrics, nil
}

// collectAMD reads MSR C0010299h (Zen family 17h+).
// Tctl = bits 31:21 / 8.0 °C.
func (c *CPUTempCollector) collectAMD() ([]Metric, error) {
	eax, _, ok := c.readMSROnCore(msrAMDZenTemp, 0)
	if !ok {
		log.Printf("cpu_temp: AMD MSR read failed, falling back to ACPI")
		return c.collectAcpi()
	}
	tctl := float64((eax>>21)&0x7FF) / 8.0
	return []Metric{{
		Name:  "cpu_temp_celsius",
		Value: tctl,
		Labels: map[string]string{
			"zone":   "CPU Tctl",
			"source": "Ring0",
		},
	}}, nil
}

// ─── ACPI fallback ────────────────────────────────────────────────────────

type acpiThermalZone struct {
	InstanceName       string
	CurrentTemperature uint32
}

type perfThermalData struct {
	Name        string
	Temperature uint32
}

func (c *CPUTempCollector) collectAcpi() ([]Metric, error) {
	// Primary: MSAcpi_ThermalZoneTemperature (needs admin)
	var zones []acpiThermalZone
	if err := wmi.QueryNamespace(
		"SELECT InstanceName, CurrentTemperature FROM MSAcpi_ThermalZoneTemperature",
		&zones, `root\wmi`,
	); err == nil && len(zones) > 0 {
		var metrics []Metric
		for i, z := range zones {
			celsius := float64(z.CurrentTemperature)/10.0 - 273.15
			metrics = append(metrics, Metric{
				Name:  "cpu_temp_celsius",
				Value: celsius,
				Labels: map[string]string{
					"zone":   fmt.Sprintf("TZ%02d", i),
					"source": "ACPI",
				},
			})
		}
		return metrics, nil
	}

	// Fallback: Win32_PerfFormattedData_Counters_ThermalZoneInformation
	// Class name is always English regardless of Windows locale.
	var perf []perfThermalData
	if err := wmi.Query(
		"SELECT Name, Temperature FROM Win32_PerfFormattedData_Counters_ThermalZoneInformation",
		&perf,
	); err == nil && len(perf) > 0 {
		var metrics []Metric
		for i, z := range perf {
			celsius := float64(z.Temperature) - 273.15
			zone := z.Name
			if zone == "" {
				zone = fmt.Sprintf("TZ%02d", i)
			}
			metrics = append(metrics, Metric{
				Name:  "cpu_temp_celsius",
				Value: celsius,
				Labels: map[string]string{
					"zone":   zone,
					"source": "PerfCounter",
				},
			})
		}
		return metrics, nil
	}

	return nil, nil
}

// ─── Helpers ──────────────────────────────────────────────────────────────

func detectCPUVendor() cpuVendor {
	type proc struct{ Name string }
	var procs []proc
	if err := wmi.Query("SELECT Name FROM Win32_Processor", &procs); err == nil && len(procs) > 0 {
		n := strings.ToUpper(procs[0].Name)
		if strings.Contains(n, "INTEL") {
			return vendorIntel
		}
		if strings.Contains(n, "AMD") {
			return vendorAMD
		}
	}
	return vendorUnknown
}
