//go:build windows

package collectors

import (
	"fmt"
	"log"

	"github.com/yusufpapurcu/wmi"
)

// msftStorageReliabilityCounter maps to MSFT_StorageReliabilityCounter WMI class.
type msftStorageReliabilityCounter struct {
	DeviceId    string
	Temperature uint8
	Wear        uint8
	ReadErrorsTotal uint64
	WriteErrorsTotal uint64
	PowerOnHours uint64
}

// msftPhysicalDisk maps to MSFT_PhysicalDisk WMI class.
type msftPhysicalDisk struct {
	DeviceId    string
	FriendlyName string
	BusType     uint16 // 17 = NVMe, 11 = SATA
	MediaType   uint16
	Size        uint64
}

const busTypeNVMe = 17

// NVMeCollector collects NVMe SMART health metrics via WMI MSFT_StorageReliabilityCounter.
type NVMeCollector struct{}

func NewNVMeCollector() *NVMeCollector {
	return &NVMeCollector{}
}

func (n *NVMeCollector) Name() string {
	return "nvme"
}

func (n *NVMeCollector) Collect() ([]Metric, error) {
	// Get all physical disks to find NVMe ones
	var disks []msftPhysicalDisk
	q := wmi.CreateQuery(&disks, "")
	if err := wmi.QueryNamespace(q, &disks, `root\Microsoft\Windows\Storage`); err != nil {
		log.Printf("nvme: failed to query MSFT_PhysicalDisk: %v", err)
		return nil, nil
	}

	// Get reliability counters
	var counters []msftStorageReliabilityCounter
	qc := wmi.CreateQuery(&counters, "")
	if err := wmi.QueryNamespace(qc, &counters, `root\Microsoft\Windows\Storage`); err != nil {
		log.Printf("nvme: failed to query MSFT_StorageReliabilityCounter: %v", err)
		return nil, nil
	}

	// Build map deviceId → counter
	counterMap := make(map[string]msftStorageReliabilityCounter)
	for _, c := range counters {
		counterMap[c.DeviceId] = c
	}

	var metrics []Metric
	for _, disk := range disks {
		if disk.BusType != busTypeNVMe {
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
				Name:   "nvme_temperature_celsius",
				Value:  float64(c.Temperature),
				Labels: copyLabels(labels),
			})
		}
		if c.Wear > 0 {
			metrics = append(metrics, Metric{
				Name:   "nvme_percentage_used",
				Value:  float64(c.Wear),
				Labels: copyLabels(labels),
			})
		}
		metrics = append(metrics, Metric{
			Name:   "nvme_power_on_hours",
			Value:  float64(c.PowerOnHours),
			Labels: copyLabels(labels),
		})
		metrics = append(metrics, Metric{
			Name:   "nvme_read_errors_total",
			Value:  float64(c.ReadErrorsTotal),
			Labels: copyLabels(labels),
		})
		metrics = append(metrics, Metric{
			Name:   "nvme_write_errors_total",
			Value:  float64(c.WriteErrorsTotal),
			Labels: copyLabels(labels),
		})
	}

	if len(metrics) == 0 {
		log.Printf("nvme: no NVMe drives found via WMI (may require admin)")
	}

	return metrics, nil
}
