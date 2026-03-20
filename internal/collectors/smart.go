//go:build windows

package collectors

import (
	"fmt"
	"log"
)

// SMARTCollector collects SMART health metrics for SATA/HDD drives via direct ATA PASS-THROUGH IOCTL.
type SMARTCollector struct{}

func NewSMARTCollector() *SMARTCollector { return &SMARTCollector{} }

func (s *SMARTCollector) Name() string { return "smart" }

func (s *SMARTCollector) Collect() ([]Metric, error) {
	drives := EnumeratePhysicalDrives()
	var metrics []Metric

	for _, d := range drives {
		if d.BusType == busTypeNvme {
			continue // handled by NVMeCollector
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

		// Power-on hours: SMART attr 0x09 (9).
		// Use only lower 32 bits — upper bytes may encode minutes/seconds on some drives.
		if a, ok := d.SmartAttrs[0x09]; ok && a.RawLo > 0 && a.RawLo < 1_000_000 {
			metrics = append(metrics, Metric{
				Name:   "smart_power_on_hours",
				Value:  float64(a.RawLo),
				Labels: copyLabels(labels),
			})
		}

		// Reallocated sectors: attr 0x05 (5) — raw value = count
		if a, ok := d.SmartAttrs[0x05]; ok {
			metrics = append(metrics, Metric{
				Name:   "smart_reallocated_sectors",
				Value:  float64(a.RawLo),
				Labels: copyLabels(labels),
			})
		}

		// SSD wear / life remaining.
		// Attr 0xE7 (231) = SSD Life Left: normalized value IS the percentage remaining.
		// Attr 0xE9 (233) = Media_Wearout_Indicator: normalized starts at 100, decreases.
		// Only emit if the normalized value is sane (1-100).
		for _, wearId := range []byte{0xE7, 0xE9} {
			if a, ok := d.SmartAttrs[wearId]; ok && a.Value > 0 && a.Value <= 100 {
				metrics = append(metrics, Metric{
					Name:   "smart_life_remaining_percent",
					Value:  float64(a.Value),
					Labels: copyLabels(labels),
				})
				break
			}
		}

		// Pending sectors: attr 0xC5 (197)
		if a, ok := d.SmartAttrs[0xC5]; ok {
			metrics = append(metrics, Metric{
				Name:   "smart_pending_sectors",
				Value:  float64(a.RawLo),
				Labels: copyLabels(labels),
			})
		}
	}

	if len(metrics) == 0 {
		log.Printf("smart: no SATA/HDD drives found or no SMART data available")
	}

	return metrics, nil
}
