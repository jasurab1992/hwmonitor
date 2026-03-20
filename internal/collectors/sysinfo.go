//go:build windows

package collectors

import (
	"fmt"

	"github.com/yusufpapurcu/wmi"
)

type win32Processor struct {
	Name                      string
	NumberOfCores             uint32
	NumberOfLogicalProcessors uint32
	L2CacheSize               uint32
	L3CacheSize               uint32
	CurrentClockSpeed         uint32
}

type win32BaseBoard struct {
	Manufacturer string
	Product      string
}

type win32BIOS struct {
	SMBIOSBIOSVersion string
	ReleaseDate       string
}

type win32PhysicalMemory struct {
	Speed            uint32
	SMBIOSMemoryType uint16
}

// SysInfoCollector collects static system information via WMI.
type SysInfoCollector struct{}

func NewSysInfoCollector() *SysInfoCollector {
	return &SysInfoCollector{}
}

func (s *SysInfoCollector) Name() string {
	return "sysinfo"
}

func (s *SysInfoCollector) Collect() ([]Metric, error) {
	labels := make(map[string]string)

	// CPU info
	var cpus []win32Processor
	if err := wmi.Query("SELECT Name, NumberOfCores, NumberOfLogicalProcessors, L2CacheSize, L3CacheSize, CurrentClockSpeed FROM Win32_Processor", &cpus); err == nil && len(cpus) > 0 {
		labels["cpu_name"] = cpus[0].Name
		labels["cpu_cores"] = fmt.Sprintf("%d", cpus[0].NumberOfCores)
		labels["cpu_threads"] = fmt.Sprintf("%d", cpus[0].NumberOfLogicalProcessors)
		labels["cpu_l2_cache_kb"] = fmt.Sprintf("%d", cpus[0].L2CacheSize)
		labels["cpu_l3_cache_kb"] = fmt.Sprintf("%d", cpus[0].L3CacheSize)
		labels["cpu_clock_mhz"] = fmt.Sprintf("%d", cpus[0].CurrentClockSpeed)
	}

	// Motherboard info
	var boards []win32BaseBoard
	if err := wmi.Query("SELECT Manufacturer, Product FROM Win32_BaseBoard", &boards); err == nil && len(boards) > 0 {
		labels["mobo"] = boards[0].Manufacturer + " " + boards[0].Product
	}

	// BIOS info
	var bios []win32BIOS
	if err := wmi.Query("SELECT SMBIOSBIOSVersion, ReleaseDate FROM Win32_BIOS", &bios); err == nil && len(bios) > 0 {
		labels["bios_version"] = bios[0].SMBIOSBIOSVersion
		labels["bios_date"] = bios[0].ReleaseDate
	}

	// RAM info
	var mem []win32PhysicalMemory
	if err := wmi.Query("SELECT Speed, SMBIOSMemoryType FROM Win32_PhysicalMemory", &mem); err == nil && len(mem) > 0 {
		ramType := "Unknown"
		switch mem[0].SMBIOSMemoryType {
		case 17:
			ramType = "DDR4"
		case 34:
			ramType = "DDR5"
		case 26:
			ramType = "DDR4" // some systems report 26 for DDR4
		case 24:
			ramType = "DDR3"
		case 20:
			ramType = "DDR"
		case 21:
			ramType = "DDR2"
		}
		labels["ram_type"] = ramType
		labels["ram_speed_mhz"] = fmt.Sprintf("%d", mem[0].Speed)
	}

	metrics := []Metric{
		{
			Name:   "system_info",
			Value:  1,
			Labels: labels,
		},
	}

	return metrics, nil
}
