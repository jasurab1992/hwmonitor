package collectors

import (
	"fmt"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

// CPUCollector collects CPU usage, per-core usage, core count, and frequency.
type CPUCollector struct{}

// NewCPUCollector creates a new CPUCollector.
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{}
}

func (c *CPUCollector) Name() string {
	return "cpu"
}

func (c *CPUCollector) Collect() ([]Metric, error) {
	var metrics []Metric

	// Single call for per-core usage, then compute total as average
	perCore, err := cpu.Percent(time.Second, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU usage: %w", err)
	}

	// Compute total as average of all cores
	if len(perCore) > 0 {
		var sum float64
		for _, pct := range perCore {
			sum += pct
		}
		metrics = append(metrics, Metric{
			Name:  "cpu_usage_percent",
			Value: sum / float64(len(perCore)),
		})
	}

	// Per-core metrics
	for i, pct := range perCore {
		metrics = append(metrics, Metric{
			Name:  "cpu_core_usage_percent",
			Value: pct,
			Labels: map[string]string{
				"core": fmt.Sprintf("%d", i),
			},
		})
	}

	// Number of logical cores
	metrics = append(metrics, Metric{
		Name:  "cpu_cores_total",
		Value: float64(runtime.NumCPU()),
	})

	// CPU frequency
	infos, err := cpu.Info()
	if err != nil {
		return metrics, fmt.Errorf("failed to get CPU info: %w", err)
	}
	if len(infos) > 0 {
		metrics = append(metrics, Metric{
			Name:  "cpu_frequency_mhz",
			Value: infos[0].Mhz,
		})
	}

	return metrics, nil
}
