//go:build windows

package collectors

import (
	"fmt"
	"log"
)

// SMARTCollector collects health metrics for SATA/HDD drives via smartctl.
type SMARTCollector struct{}

func NewSMARTCollector() *SMARTCollector { return &SMARTCollector{} }
func (s *SMARTCollector) Name() string   { return "smart" }

func (s *SMARTCollector) Collect() ([]Metric, error) {
	drives := EnumeratePhysicalDrives()
	var metrics []Metric

	for _, d := range drives {
		if d.BusType == busTypeNvme {
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
				Name:   "smart_temp_celsius",
				Value:  d.TempC,
				Labels: copyLabels(labels),
			})
		}
		if d.HasPowerOnHours {
			metrics = append(metrics, Metric{
				Name:   "smart_power_on_hours",
				Value:  float64(d.PowerOnHours),
				Labels: copyLabels(labels),
			})
		}
		if d.HasPercentUsed {
			metrics = append(metrics, Metric{
				Name:   "smart_life_remaining_percent",
				Value:  float64(100 - d.PercentUsed),
				Labels: copyLabels(labels),
			})
		}
		if d.HasSpareAvail {
			metrics = append(metrics, Metric{
				Name:   "smart_spare_available_percent",
				Value:  float64(d.SpareAvail),
				Labels: copyLabels(labels),
			})
		}
		if d.HasReallocated {
			metrics = append(metrics, Metric{
				Name:   "smart_reallocated_sectors",
				Value:  float64(d.ReallocatedSectors),
				Labels: copyLabels(labels),
			})
		}
		if d.HasPending {
			metrics = append(metrics, Metric{
				Name:   "smart_pending_sectors",
				Value:  float64(d.PendingSectors),
				Labels: copyLabels(labels),
			})
		}
	}

	if len(metrics) == 0 {
		log.Printf("smart: no SATA/HDD drives found or no SMART data available")
	}
	return metrics, nil
}
