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
}

type win32BaseBoard struct {
	Manufacturer string
	Product      string
}

type win32BIOS struct {
	SMBIOSBIOSVersion string
}

type win32PhysicalMemory struct {
	Capacity         uint64
	Speed            uint32
	MemoryType       uint16
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

func decodeMemoryType(mt uint16) string {
	switch mt {
	case 20:
		return "DDR"
	case 21:
		return "DDR2"
	case 24:
		return "DDR3"
	case 26:
		return "DDR4"
	case 34:
		return "DDR5"
	default:
		return "Unknown"
	}
}

func (s *SysInfoCollector) Collect() ([]Metric, error) {
	var metrics []Metric

	// CPU info
	var cpus []win32Processor
	if err := wmi.Query("SELECT Name, NumberOfCores, NumberOfLogicalProcessors, L2CacheSize, L3CacheSize FROM Win32_Processor", &cpus); err == nil && len(cpus) > 0 {
		metrics = append(metrics,
			Metric{
				Name:   "sysinfo_cpu_cores",
				Value:  float64(cpus[0].NumberOfCores),
				Labels: map[string]string{"processor": cpus[0].Name},
			},
			Metric{
				Name:   "sysinfo_cpu_threads",
				Value:  float64(cpus[0].NumberOfLogicalProcessors),
				Labels: map[string]string{"processor": cpus[0].Name},
			},
			Metric{
				Name:  "sysinfo_cpu_l2_cache_kb",
				Value: float64(cpus[0].L2CacheSize),
			},
			Metric{
				Name:  "sysinfo_cpu_l3_cache_kb",
				Value: float64(cpus[0].L3CacheSize),
			},
		)
	}

	// Motherboard info
	var boards []win32BaseBoard
	if err := wmi.Query("SELECT Manufacturer, Product FROM Win32_BaseBoard", &boards); err == nil && len(boards) > 0 {
		metrics = append(metrics, Metric{
			Name:  "sysinfo_baseboard_info",
			Value: 1,
			Labels: map[string]string{
				"manufacturer": boards[0].Manufacturer,
				"product":      boards[0].Product,
			},
		})
	}

	// BIOS info
	var bios []win32BIOS
	if err := wmi.Query("SELECT SMBIOSBIOSVersion FROM Win32_BIOS", &bios); err == nil && len(bios) > 0 {
		metrics = append(metrics, Metric{
			Name:  "sysinfo_bios_info",
			Value: 1,
			Labels: map[string]string{
				"version": bios[0].SMBIOSBIOSVersion,
			},
		})
	}

	// RAM info
	var mem []win32PhysicalMemory
	if err := wmi.Query("SELECT Capacity, Speed, MemoryType, SMBIOSMemoryType FROM Win32_PhysicalMemory", &mem); err == nil {
		for i, m := range mem {
			memType := decodeMemoryType(m.MemoryType)
			if memType == "Unknown" && m.SMBIOSMemoryType > 0 {
				memType = decodeMemoryType(m.SMBIOSMemoryType)
			}
			metrics = append(metrics, Metric{
				Name:  "sysinfo_memory_module_bytes",
				Value: float64(m.Capacity),
				Labels: map[string]string{
					"slot":      fmt.Sprintf("%d", i),
					"speed_mhz": fmt.Sprintf("%d", m.Speed),
					"type":      memType,
				},
			})
		}
	}

	return metrics, nil
}
