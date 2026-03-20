package collectors

import (
	"fmt"

	"github.com/shirou/gopsutil/v3/disk"
)

// DiskCollector collects disk usage and I/O metrics.
type DiskCollector struct{}

// NewDiskCollector creates a new DiskCollector.
func NewDiskCollector() *DiskCollector {
	return &DiskCollector{}
}

func (d *DiskCollector) Name() string {
	return "disk"
}

func (d *DiskCollector) Collect() ([]Metric, error) {
	var metrics []Metric

	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk partitions: %w", err)
	}

	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}

		labels := map[string]string{
			"device":     p.Device,
			"mountpoint": p.Mountpoint,
		}

		metrics = append(metrics,
			Metric{Name: "disk_total_bytes", Value: float64(usage.Total), Labels: copyLabels(labels)},
			Metric{Name: "disk_used_bytes", Value: float64(usage.Used), Labels: copyLabels(labels)},
			Metric{Name: "disk_free_bytes", Value: float64(usage.Free), Labels: copyLabels(labels)},
			Metric{Name: "disk_usage_percent", Value: usage.UsedPercent, Labels: copyLabels(labels)},
		)
	}

	ioCounters, err := disk.IOCounters()
	if err != nil {
		return metrics, fmt.Errorf("failed to get disk I/O counters: %w", err)
	}

	for device, io := range ioCounters {
		labels := map[string]string{"device": device}

		metrics = append(metrics,
			Metric{Name: "disk_read_bytes_total", Value: float64(io.ReadBytes), Labels: copyLabels(labels)},
			Metric{Name: "disk_write_bytes_total", Value: float64(io.WriteBytes), Labels: copyLabels(labels)},
			Metric{Name: "disk_read_count_total", Value: float64(io.ReadCount), Labels: copyLabels(labels)},
			Metric{Name: "disk_write_count_total", Value: float64(io.WriteCount), Labels: copyLabels(labels)},
		)
	}

	return metrics, nil
}

func copyLabels(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
