package collectors

import (
	"fmt"
	"runtime"
	"sync"

	"github.com/shirou/gopsutil/v3/cpu"
)

// CPUCollector collects CPU usage, per-core usage, core count, and frequency.
type CPUCollector struct {
	mu sync.Mutex
}

// NewCPUCollector creates a new CPUCollector.
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{}
}

func (c *CPUCollector) Name() string {
	return "cpu"
}

func (c *CPUCollector) Collect() ([]Metric, error) {
	// Serialize access: gopsutil uses a global cache for interval=0 deltas.
	// Without a mutex, overlapping calls (UI polls at 1s, Collect takes ~0ms)
	// corrupt the shared state and produce bogus 100% readings.
	c.mu.Lock()
	defer c.mu.Unlock()

	var metrics []Metric

	// interval=0: non-blocking, returns delta since last call.
	// Avoids the 1-second blocking sleep that causes concurrent-call races.
	perCore, err := cpu.Percent(0, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get CPU usage: %w", err)
	}

	// Clamp: gopsutil occasionally returns slightly out-of-range values.
	for i, pct := range perCore {
		if pct < 0 {
			perCore[i] = 0
		} else if pct > 100 {
			perCore[i] = 100
		}
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
