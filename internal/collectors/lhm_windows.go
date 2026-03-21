//go:build windows

package collectors

import (
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

type lhmBridgeSensor struct {
	Name       string  `json:"Name"`
	Type       string  `json:"Type"`
	Hardware   string  `json:"Hardware"`
	HwType     string  `json:"HwType"`
	Identifier string  `json:"Identifier"`
	Value      float64 `json:"Value"`
}

var (
	lhmOnce    sync.Once
	lhmBin     string
	lhmReady   bool
	lhmTempBin string

	lhmCacheMu   sync.Mutex
	lhmCache     []lhmBridgeSensor
	lhmCacheTime time.Time
	lhmCacheTTL  = 30 * time.Second
)

func initLHM() {
	lhmOnce.Do(func() {
		if len(lhmEmbedded) > 0 {
			tmp := filepath.Join(os.TempDir(), "hwmon_lhm_bridge.exe")
			if err := os.WriteFile(tmp, lhmEmbedded, 0700); err == nil {
				lhmBin = tmp
				lhmTempBin = tmp
				lhmReady = true
				log.Printf("lhm: using embedded lhm_bridge")
				return
			}
		}
		for _, c := range []string{
			"lhm_bridge",
			`C:\Program Files\lhm_bridge\lhm_bridge.exe`,
		} {
			if _, err := os.Stat(c); err == nil {
				lhmBin = c
				lhmReady = true
				log.Printf("lhm: lhm_bridge found at %s", c)
				return
			}
		}
	})
}

func CleanupLHM() {
	if lhmTempBin != "" {
		os.Remove(lhmTempBin)
	}
}

func collectLHMSensors() []lhmBridgeSensor {
	initLHM()
	if !lhmReady {
		return nil
	}

	lhmCacheMu.Lock()
	defer lhmCacheMu.Unlock()

	if lhmCache != nil && time.Since(lhmCacheTime) < lhmCacheTTL {
		return lhmCache
	}

	cmd := exec.Command(lhmBin)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.Output()
	if err != nil && len(out) == 0 {
		log.Printf("lhm: lhm_bridge failed: %v", err)
		return lhmCache // return stale cache on error rather than nothing
	}
	var sensors []lhmBridgeSensor
	if err := json.Unmarshal(out, &sensors); err != nil {
		log.Printf("lhm: JSON parse error: %v", err)
		return lhmCache
	}
	lhmCache = sensors
	lhmCacheTime = time.Now()
	return sensors
}

// LHMCollector collects hardware sensor data via the embedded lhm_bridge.exe
// (LibreHardwareMonitor). Provides voltages, fan speeds, and temperatures
// from SuperIO chips, CPU package, GPU, etc.
type LHMCollector struct{}

func NewLHMCollector() *LHMCollector { return &LHMCollector{} }
func (l *LHMCollector) Name() string  { return "lhm" }

func (l *LHMCollector) Collect() ([]Metric, error) {
	sensors := collectLHMSensors()
	if len(sensors) == 0 {
		return nil, nil
	}

	var metrics []Metric
	for _, s := range sensors {
		var name string
		switch s.Type {
		case "Temperature":
			name = "lhm_temperature_celsius"
		case "Voltage":
			name = "lhm_voltage_volts"
		case "Fan":
			name = "lhm_fan_rpm"
		case "Load":
			name = "lhm_load_percent"
		case "Power":
			name = "lhm_power_watts"
		case "Clock":
			name = "lhm_clock_mhz"
		default:
			continue
		}
		metrics = append(metrics, Metric{
			Name:  name,
			Value: s.Value,
			Labels: map[string]string{
				"name":       s.Name,
				"hardware":   s.Hardware,
				"hw_type":    s.HwType,
				"identifier": s.Identifier,
			},
		})
	}
	return metrics, nil
}
