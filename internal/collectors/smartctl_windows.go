//go:build windows

package collectors

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
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
	CriticalWarning      int `json:"critical_warning"`
	Temperature          int `json:"temperature"` // °C, already converted by smartctl
	AvailableSpare       int `json:"available_spare"`
	PercentageUsed       int `json:"percentage_used"`
	PowerOnHours         int `json:"power_on_hours"`
	MediaErrors          int `json:"media_errors"`
}

// ─── Discovery ────────────────────────────────────────────────────────────────

var (
	smartctlOnce  sync.Once
	smartctlBin   string
	smartctlReady bool
)

func initSmartctl() {
	smartctlOnce.Do(func() {
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

// ─── Enrichment ───────────────────────────────────────────────────────────────

// enrichWithSmartctl runs smartctl -j -A on the given drive and merges any
// missing data into info. It never overwrites data already obtained via IOCTL.
func enrichWithSmartctl(info *physicalDriveInfo) {
	initSmartctl()
	if !smartctlReady {
		return
	}

	// For NVMe drives that our IOCTL approach couldn't read, also pass -d nvme.
	args := []string{"-j", "-A", fmt.Sprintf("/dev/pd%d", info.Index)}
	if info.BusType == busTypeNvme && !info.NVMeHasData {
		args = append([]string{"-d", "nvme"}, args...)
	}

	out, _ := exec.Command(smartctlBin, args...).Output()
	// smartctl exits non-zero for warnings/unsupported features but still writes JSON.
	if len(out) == 0 {
		return
	}

	var result smartctlJSON
	if err := json.Unmarshal(out, &result); err != nil {
		return
	}

	// Merge ATA SMART attributes — fill in any attr not yet in our map.
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

	// Merge NVMe health — only if IOCTL didn't get it.
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

	// Temperature from smartctl's top-level field (covers NVMe and some SATA).
	if result.Temperature != nil && result.Temperature.Current > 0 && !info.HasTemp {
		t := float64(result.Temperature.Current)
		if t > 0 && t < 120 {
			info.TempC = t
			info.HasTemp = true
		}
	}
}
