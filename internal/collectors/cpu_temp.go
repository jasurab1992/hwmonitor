//go:build windows

package collectors

import (
	"fmt"
	"log"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"github.com/yusufpapurcu/wmi"
)

const (
	ring0DLL = "WinRing0x64.dll"

	// Intel MSRs
	msrIA32ThermStatus = 0x19C // per-core: bit31=valid, bits22:16=digital readout below TjMax
	msrTempTarget      = 0x1A2 // bits23:16 = TjMax

	// AMD Zen MSR (family 17h+)
	msrAMDZenTemp = 0xC0010299 // bits31:21 = Tctl * 8 (0.125°C units)
)

type cpuVendor int

const (
	vendorUnknown cpuVendor = iota
	vendorIntel
	vendorAMD
)

// CPUTempCollector reads per-core CPU temperatures via WinRing0 Ring0 driver.
// Falls back to WMI ACPI thermal zones if WinRing0x64.dll is not available.
type CPUTempCollector struct {
	ring0OK bool
	vendor  cpuVendor
	tjmax   uint32
	rdmsr   *syscall.Proc
}

// Reuse WMI structs declared in nvme.go (msAcpiThermalZone, perfThermalZone)
// to avoid redeclaration — defined here only.

type acpiThermalZone struct {
	InstanceName       string
	CurrentTemperature uint32
}

type perfThermalData struct {
	Name        string
	Temperature uint32
}

func NewCPUTempCollector() *CPUTempCollector {
	c := &CPUTempCollector{}
	c.ring0OK = c.initRing0()
	return c
}

func (c *CPUTempCollector) Name() string { return "cpu_temp" }

func (c *CPUTempCollector) initRing0() bool {
	dll, err := syscall.LoadDLL(ring0DLL)
	if err != nil {
		log.Printf("cpu_temp: WinRing0x64.dll not found — ACPI fallback active (%v)", err)
		return false
	}

	initFn, err1 := dll.FindProc("InitializeOls")
	_, err2 := dll.FindProc("DeinitializeOls")
	rdmsr, err3 := dll.FindProc("ReadMsrTx")
	if err1 != nil || err2 != nil || err3 != nil {
		log.Printf("cpu_temp: WinRing0 DLL missing required exports")
		dll.Release()
		return false
	}

	ret, _, _ := initFn.Call()
	if ret == 0 {
		log.Printf("cpu_temp: WinRing0 InitializeOls failed — run as Administrator")
		dll.Release()
		return false
	}

	c.rdmsr = rdmsr
	c.vendor = detectCPUVendor()

	if c.vendor == vendorIntel {
		c.tjmax = c.readTjMax()
		log.Printf("cpu_temp: Ring0 ready — Intel, TjMax=%d°C", c.tjmax)
	} else if c.vendor == vendorAMD {
		log.Printf("cpu_temp: Ring0 ready — AMD Zen")
	}

	return true
}

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

// readTjMax reads MSR_TEMPERATURE_TARGET on core 0 to get the CPU's thermal junction max.
func (c *CPUTempCollector) readTjMax() uint32 {
	var eax, edx uint32
	ok, _, _ := c.rdmsr.Call(
		uintptr(msrTempTarget),
		uintptr(unsafe.Pointer(&eax)),
		uintptr(unsafe.Pointer(&edx)),
		uintptr(1), // affinity: logical CPU 0
	)
	if ok != 0 {
		tjmax := (eax >> 16) & 0xFF
		if tjmax >= 80 && tjmax <= 115 {
			return tjmax
		}
	}
	return 100 // safe default for modern Intel
}

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

// collectIntel reads IA32_THERM_STATUS MSR for each logical CPU.
// Temperature = TjMax - DigitalReadout (bits 22:16).
func (c *CPUTempCollector) collectIntel() ([]Metric, error) {
	numCPU := runtime.NumCPU()
	var metrics []Metric
	var maxTemp float64

	for i := 0; i < numCPU && i < 64; i++ {
		var eax, edx uint32
		ok, _, _ := c.rdmsr.Call(
			uintptr(msrIA32ThermStatus),
			uintptr(unsafe.Pointer(&eax)),
			uintptr(unsafe.Pointer(&edx)),
			uintptr(1)<<i,
		)
		if ok == 0 {
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
		// Prepend CPU Package = hottest core
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

// collectAMD reads MSR C0010299h for AMD Zen (Ryzen/EPYC family 17h+).
// Tctl = bits31:21 / 8.0 (degrees Celsius).
func (c *CPUTempCollector) collectAMD() ([]Metric, error) {
	var eax, edx uint32
	ok, _, _ := c.rdmsr.Call(
		uintptr(msrAMDZenTemp),
		uintptr(unsafe.Pointer(&eax)),
		uintptr(unsafe.Pointer(&edx)),
		uintptr(1),
	)
	if ok == 0 {
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

// collectAcpi reads WMI thermal zones — no driver required, admin optional.
func (c *CPUTempCollector) collectAcpi() ([]Metric, error) {
	// Primary: MSAcpi_ThermalZoneTemperature (requires admin)
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

	// Fallback: Win32_PerfFormattedData_Counters_ThermalZoneInformation (no admin needed)
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
