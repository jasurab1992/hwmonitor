//go:build windows

package collectors

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
)

// ─── smartctl JSON output structures ─────────────────────────────────────────

type smartctlJSON struct {
	NVMeHealthLog *struct {
		Temperature    int   `json:"temperature"`
		AvailableSpare int   `json:"available_spare"`
		PercentageUsed int   `json:"percentage_used"`
		PowerOnHours   int64 `json:"power_on_hours"`
		MediaErrors    int64 `json:"media_errors"`
	} `json:"nvme_smart_health_information_log"`

	ATASmartAttributes *struct {
		Table []struct {
			ID  int `json:"id"`
			Raw struct {
				Value int64 `json:"value"`
			} `json:"raw"`
		} `json:"table"`
	} `json:"ata_smart_attributes"`

	Temperature *struct {
		Current int `json:"current"`
	} `json:"temperature"`

	PowerOnTime *struct {
		Hours int64 `json:"hours"`
	} `json:"power_on_time"`

	// smartctl 7.3+: vendor-agnostic top-level endurance and spare fields
	EnduranceUsed *struct {
		CurrentPercent int `json:"current_percent"`
	} `json:"endurance_used"`

	SpareAvailable *struct {
		CurrentPercent int `json:"current_percent"`
	} `json:"spare_available"`
}

// ─── Discovery / lifecycle ───────────────────────────────────────────────────

var (
	smartctlOnce    sync.Once
	smartctlBin     string
	smartctlReady   bool
	smartctlTempBin string
)

func initSmartctl() {
	smartctlOnce.Do(func() {
		if len(smartctlEmbedded) > 0 {
			tmp := filepath.Join(os.TempDir(), "hwmon_smartctl.exe")
			if err := os.WriteFile(tmp, smartctlEmbedded, 0700); err == nil {
				smartctlBin = tmp
				smartctlTempBin = tmp
				smartctlReady = true
				log.Printf("disk: using embedded smartctl")
				return
			}
		}
		for _, c := range []string{
			"smartctl",
			`C:\Program Files\smartmontools\bin\smartctl.exe`,
			`C:\Program Files (x86)\smartmontools\bin\smartctl.exe`,
		} {
			if path, err := exec.LookPath(c); err == nil {
				smartctlBin = path
				smartctlReady = true
				log.Printf("disk: smartctl found at %s", path)
				return
			}
			if _, err := os.Stat(c); err == nil {
				smartctlBin = c
				smartctlReady = true
				log.Printf("disk: smartctl found at %s", c)
				return
			}
		}
		log.Printf("disk: smartctl not available — SMART data will be limited")
	})
}

// CleanupSmartctl removes the temporary extracted smartctl.exe (if any).
func CleanupSmartctl() {
	if smartctlTempBin != "" {
		os.Remove(smartctlTempBin)
	}
}

// ─── Primary SMART data collection ───────────────────────────────────────────

// collectSmartData runs smartctl -j -A and populates all health fields in info.
// This is the sole source of SMART data — no IOCTL SMART fallbacks needed.
func collectSmartData(info *physicalDriveInfo) {
	initSmartctl()
	if !smartctlReady {
		return
	}

	args := []string{"-j", "-A", fmt.Sprintf("/dev/pd%d", info.Index)}
	if info.BusType == busTypeNvme {
		args = append([]string{"-d", "nvme"}, args...)
	}

	cmd := exec.Command(smartctlBin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, _ := cmd.Output()
	if len(out) == 0 {
		return
	}

	var result smartctlJSON
	if err := json.Unmarshal(out, &result); err != nil {
		return
	}

	// ── NVMe ──────────────────────────────────────────────────────────────────
	if h := result.NVMeHealthLog; h != nil {
		info.PercentUsed = h.PercentageUsed
		info.HasPercentUsed = true
		info.SpareAvail = h.AvailableSpare
		info.HasSpareAvail = true
		info.PowerOnHours = uint64(h.PowerOnHours)
		info.HasPowerOnHours = true
		info.MediaErrors = uint64(h.MediaErrors)
		info.HasMediaErrors = true
		if h.Temperature > 0 && !info.HasTemp {
			info.TempC = float64(h.Temperature)
			info.HasTemp = true
		}
	}

	// ── SATA/HDD top-level fields ─────────────────────────────────────────────

	// endurance_used: vendor-agnostic % worn (Samsung, WD, Crucial, etc.)
	if e := result.EnduranceUsed; e != nil && !info.HasPercentUsed {
		info.PercentUsed = e.CurrentPercent
		info.HasPercentUsed = true
	}

	// spare_available: reserved spare blocks remaining
	if s := result.SpareAvailable; s != nil && !info.HasSpareAvail {
		info.SpareAvail = s.CurrentPercent
		info.HasSpareAvail = true
	}

	// power_on_time: top-level field, more reliable than attr 9 raw
	if p := result.PowerOnTime; p != nil {
		info.PowerOnHours = uint64(p.Hours)
		info.HasPowerOnHours = true
	}

	// ── ATA attributes — only specific IDs we care about ─────────────────────
	if result.ATASmartAttributes != nil {
		attrRaw := make(map[int]int64, len(result.ATASmartAttributes.Table))
		for _, a := range result.ATASmartAttributes.Table {
			attrRaw[a.ID] = a.Raw.Value
		}

		// Attr 9 — Power On Hours (fallback if power_on_time absent)
		if !info.HasPowerOnHours {
			if v, ok := attrRaw[9]; ok && v > 0 && v < 1_000_000 {
				info.PowerOnHours = uint64(v)
				info.HasPowerOnHours = true
			}
		}

		// Attr 5 — Reallocated Sector Count
		if v, ok := attrRaw[5]; ok {
			info.ReallocatedSectors = uint32(v)
			info.HasReallocated = true
		}

		// Attr 197 — Current Pending Sector Count
		if v, ok := attrRaw[197]; ok {
			info.PendingSectors = uint32(v)
			info.HasPending = true
		}
	}

	// ── Temperature fallback ──────────────────────────────────────────────────
	if t := result.Temperature; t != nil && !info.HasTemp {
		c := float64(t.Current)
		if c > -20 && c < 120 {
			info.TempC = c
			info.HasTemp = true
		}
	}
}
