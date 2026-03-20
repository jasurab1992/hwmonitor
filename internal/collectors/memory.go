package collectors

import (
	"fmt"

	"github.com/shirou/gopsutil/v3/mem"
)

// MemoryCollector collects RAM and swap usage metrics.
type MemoryCollector struct{}

// NewMemoryCollector creates a new MemoryCollector.
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{}
}

func (m *MemoryCollector) Name() string {
	return "memory"
}

func (m *MemoryCollector) Collect() ([]Metric, error) {
	var metrics []Metric

	vm, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get virtual memory stats: %w", err)
	}

	metrics = append(metrics,
		Metric{Name: "memory_total_bytes", Value: float64(vm.Total)},
		Metric{Name: "memory_used_bytes", Value: float64(vm.Used)},
		Metric{Name: "memory_available_bytes", Value: float64(vm.Available)},
		Metric{Name: "memory_usage_percent", Value: vm.UsedPercent},
	)

	sw, err := mem.SwapMemory()
	if err != nil {
		return metrics, fmt.Errorf("failed to get swap memory stats: %w", err)
	}

	metrics = append(metrics,
		Metric{Name: "swap_total_bytes", Value: float64(sw.Total)},
		Metric{Name: "swap_used_bytes", Value: float64(sw.Used)},
		Metric{Name: "swap_usage_percent", Value: sw.UsedPercent},
	)

	return metrics, nil
}
