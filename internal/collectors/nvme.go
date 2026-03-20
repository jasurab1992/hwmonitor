//go:build windows

package collectors

import (
	"fmt"
	"log"
)

const busTypeNVMe = busTypeNvme

// NVMeCollector collects NVMe health metrics via smartctl.
type NVMeCollector struct{}

func NewNVMeCollector() *NVMeCollector { return &NVMeCollector{} }
func (n *NVMeCollector) Name() string  { return "nvme" }

func (n *NVMeCollector) Collect() ([]Metric, error) {
	drives := EnumeratePhysicalDrives()
	var metrics []Metric

	for _, d := range drives {
		if d.BusType != busTypeNvme {
			continue
		}

		label := d.Model
		if label == "" {
			label = fmt.Sprintf("PhysicalDrive%d", d.Index)
		}
		labels := map[string]string{
			"device": label,
			"drive":  fmt.Sprintf("%d", d.Index),
		}

		if d.HasTemp {
			metrics = append(metrics, Metric{
				Name:   "nvme_temperature_celsius",
				Value:  d.TempC,
				Labels: copyLabels(labels),
			})
		}
		if d.HasPercentUsed {
			metrics = append(metrics, Metric{
				Name:   "nvme_percentage_used",
				Value:  float64(d.PercentUsed),
				Labels: copyLabels(labels),
			})
		}
		if d.HasSpareAvail {
			metrics = append(metrics, Metric{
				Name:   "nvme_available_spare_percent",
				Value:  float64(d.SpareAvail),
				Labels: copyLabels(labels),
			})
		}
		if d.HasPowerOnHours {
			metrics = append(metrics, Metric{
				Name:   "nvme_power_on_hours",
				Value:  float64(d.PowerOnHours),
				Labels: copyLabels(labels),
			})
		}
		if d.HasMediaErrors {
			metrics = append(metrics, Metric{
				Name:   "nvme_media_errors_total",
				Value:  float64(d.MediaErrors),
				Labels: copyLabels(labels),
			})
		}
	}

	if len(metrics) == 0 {
		log.Printf("nvme: no NVMe drives found or no data available")
	}
	return metrics, nil
}
