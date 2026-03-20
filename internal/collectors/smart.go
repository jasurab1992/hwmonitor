//go:build windows

package collectors

import (
	"fmt"
	"log"

	"github.com/yusufpapurcu/wmi"
)

// SMARTCollector collects SMART health metrics for SATA/HDD drives via WMI MSFT_StorageReliabilityCounter.
type SMARTCollector struct{}

func NewSMARTCollector() *SMARTCollector {
	return &SMARTCollector{}
}

func (s *SMARTCollector) Name() string {
	return "smart"
}

func (s *SMARTCollector) Collect() ([]Metric, error) {
	var disks []msftPhysicalDisk
	q := wmi.CreateQuery(&disks, "")
	if err := wmi.QueryNamespace(q, &disks, `root\Microsoft\Windows\Storage`); err != nil {
		log.Printf("smart: failed to query MSFT_PhysicalDisk: %v", err)
		return nil, nil
	}

	var counters []msftStorageReliabilityCounter
	qc := wmi.CreateQuery(&counters, "")
	if err := wmi.QueryNamespace(qc, &counters, `root\Microsoft\Windows\Storage`); err != nil {
		log.Printf("smart: failed to query MSFT_StorageReliabilityCounter: %v", err)
		return nil, nil
	}

	counterMap := make(map[string]msftStorageReliabilityCounter)
	for _, c := range counters {
		counterMap[c.DeviceId] = c
	}

	var metrics []Metric
	for _, disk := range disks {
		// Only non-NVMe drives (SATA, ATA, SAS, etc.)
		if disk.BusType == busTypeNVMe {
			continue
		}

		c, ok := counterMap[disk.DeviceId]
		if !ok {
			continue
		}

		label := fmt.Sprintf("%s (Drive%s)", disk.FriendlyName, disk.DeviceId)
		labels := map[string]string{
			"device": label,
			"id":     disk.DeviceId,
		}

		if c.Temperature > 0 {
			metrics = append(metrics, Metric{
				Name:   "smart_temp_celsius",
				Value:  float64(c.Temperature),
				Labels: copyLabels(labels),
			})
		}
		if c.Wear > 0 {
			metrics = append(metrics, Metric{
				Name:   "smart_percentage_used",
				Value:  float64(c.Wear),
				Labels: copyLabels(labels),
			})
		}
		metrics = append(metrics, Metric{
			Name:   "smart_power_on_hours",
			Value:  float64(c.PowerOnHours),
			Labels: copyLabels(labels),
		})
		metrics = append(metrics, Metric{
			Name:   "smart_read_errors_total",
			Value:  float64(c.ReadErrorsTotal),
			Labels: copyLabels(labels),
		})
		metrics = append(metrics, Metric{
			Name:   "smart_write_errors_total",
			Value:  float64(c.WriteErrorsTotal),
			Labels: copyLabels(labels),
		})
	}

	if len(metrics) == 0 {
		log.Printf("smart: no SATA/HDD drives found via WMI (may require admin)")
	}

	return metrics, nil
}
