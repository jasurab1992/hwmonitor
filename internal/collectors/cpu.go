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

	// Total CPU usage (average across all cores)
	totalPercent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get total CPU usage: %w", err)
	}
	if len(totalPercent) > 0 {
		metrics = append(metrics, Metric{
			Name:  "cpu_usage_percent",
			Value: totalPercent[0],
		})
	}

	// Per-core CPU usage
	perCore, err := cpu.Percent(time.Second, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get per-core CPU usage: %w", err)
	}
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
