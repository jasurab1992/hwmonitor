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
)

// ─── smartctl JSON output structures ─────────────────────────────────────────

type smartctlJSON struct {
	ATASmartAttributes *struct {
		Table []smartctlAttrEntry `json:"table"`
	} `json:"ata_smart_attributes"`

	NVMeHealthLog *smartctlNVMeHealth `json:"nvme_smart_health_information_log"`

	Temperature *struct {
		Current int `json:"current"`
	} `json:"temperature"`
}

type smartctlAttrEntry struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value int    `json:"value"`
	Worst int    `json:"worst"`
	Raw   struct {
		Value int64  `json:"value"`
		Str   string `json:"string"`
	} `json:"raw"`
}

type smartctlNVMeHealth struct {
	Temperature    int `json:"temperature"`
	AvailableSpare int `json:"available_spare"`
	PercentageUsed int `json:"percentage_used"`
	PowerOnHours   int `json:"power_on_hours"`
	MediaErrors    int `json:"media_errors"`
}

// ─── Discovery / lifecycle ───────────────────────────────────────────────────

var (
	smartctlOnce    sync.Once
	smartctlBin     string
	smartctlReady   bool
	smartctlTempBin string // set when we extracted the embedded binary
)

func initSmartctl() {
	smartctlOnce.Do(func() {
		// 1. Extract embedded binary (when built with -tags embed_smartctl).
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

		// 2. Fall back to PATH and common install locations.
		candidates := []string{
			"smartctl",
			`C:\Program Files\smartmontools\bin\smartctl.exe`,
			`C:\Program Files (x86)\smartmontools\bin\smartctl.exe`,
		}
		for _, c := range candidates {
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
	})
}

// CleanupSmartctl removes the temporary extracted smartctl.exe (if any).
func CleanupSmartctl() {
	if smartctlTempBin != "" {
		os.Remove(smartctlTempBin)
	}
}

// ─── Enrichment ───────────────────────────────────────────────────────────────

// enrichWithSmartctl runs `smartctl -j -A /dev/pdN` and merges any missing
// data into info. Never overwrites data already obtained via IOCTL.
func enrichWithSmartctl(info *physicalDriveInfo) {
	initSmartctl()
	if !smartctlReady {
		return
	}

	args := []string{"-j", "-A", fmt.Sprintf("/dev/pd%d", info.Index)}
	if info.BusType == busTypeNvme && !info.NVMeHasData {
		args = append([]string{"-d", "nvme"}, args...)
	}

	out, _ := exec.Command(smartctlBin, args...).Output()
	// smartctl exits non-zero for warnings but still emits JSON.
	if len(out) == 0 {
		return
	}

	var result smartctlJSON
	if err := json.Unmarshal(out, &result); err != nil {
		return
	}

	// Merge ATA SMART attributes — add only those not already in our map.
	if result.ATASmartAttributes != nil {
		for _, a := range result.ATASmartAttributes.Table {
			id := byte(a.ID)
			if _, exists := info.SmartAttrs[id]; !exists {
				info.SmartAttrs[id] = smartAttr{
					Id:    id,
					Value: byte(a.Value),
					Worst: byte(a.Worst),
					RawLo: uint32(a.Raw.Value & 0xFFFFFFFF),
					RawHi: uint16((a.Raw.Value >> 32) & 0xFFFF),
				}
			}
		}
	}

	// Merge NVMe health — only if IOCTL returned nothing.
	if result.NVMeHealthLog != nil && !info.NVMeHasData {
		h := result.NVMeHealthLog
		info.NVMePercentUsed = byte(h.PercentageUsed)
		info.NVMeAvailableSpare = byte(h.AvailableSpare)
		info.NVMePowerOnHours = uint64(h.PowerOnHours)
		info.NVMeMediaErrors = uint64(h.MediaErrors)
		if h.Temperature > 0 && !info.HasTemp {
			info.TempC = float64(h.Temperature)
			info.HasTemp = true
		}
		info.NVMeHasData = true
	}

	// Temperature fallback.
	if result.Temperature != nil && result.Temperature.Current > 0 && !info.HasTemp {
		t := float64(result.Temperature.Current)
		if t > 0 && t < 120 {
			info.TempC = t
			info.HasTemp = true
		}
	}
}
