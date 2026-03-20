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

		// SSD wear / life remaining — check vendor-specific attrs in priority order.
		// Value field = normalized (1-100), where 100 = new, 0/invalid = skip.
		//   0xE7 (231) = SSD Life Left              — WD, Kingston, Toshiba, many others
		//   0xE9 (233) = Media_Wearout_Indicator     — Intel, some others
		//   0xB4 (180) = Unused Reserved Block Count — Samsung (Value = % remaining)
		//   0xCA (202) = Percent Lifetime Remaining  — Micron / Crucial
		//   0xD1 (209) = Remaining Life Percentage   — SandForce controllers
		//   0xAA (170) = Available Reserved Space    — Intel Optane
		for _, wearId := range []byte{0xE7, 0xE9, 0xB4, 0xCA, 0xD1, 0xAA} {
			a, ok := d.SmartAttrs[wearId]
			if !ok {
				continue
			}
			// Value is normalized life-remaining (100 = new, 0 = end-of-life).
			// Accept 0 only if RawLo is also 0 (Samsung reports Value=0 on fresh drives
			// for some firmware versions where 0 means "no wear accumulated yet").
			// Reject values >100 (drive uses 0-253 scale for a different purpose).
			if a.Value > 100 {
				continue
			}
			lifeRemaining := float64(a.Value)
			if a.Value == 0 && a.RawLo == 0 {
				lifeRemaining = 100 // fresh drive, attr present but zero-initialized
			} else if a.Value == 0 {
				continue // Value=0 with non-zero raw → genuinely worn out or wrong attr
			}
			metrics = append(metrics, Metric{
				Name:   "smart_life_remaining_percent",
				Value:  lifeRemaining,
				Labels: copyLabels(labels),
			})
			break
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
