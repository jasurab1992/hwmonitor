//go:build windows

package collectors

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/yusufpapurcu/wmi"
)

// IPMICollector collects BMC/IPMI sensor data (temperatures, fans, voltages)
// via ipmiutil. Falls back to native Windows IPMI WMI provider if available.
type IPMICollector struct{}

func NewIPMICollector() *IPMICollector { return &IPMICollector{} }
func (c *IPMICollector) Name() string  { return "ipmi" }

var (
	ipmiutilOnce    sync.Once
	ipmiutilBin     string
	ipmiutilReady   bool
	ipmiutilTempBin string
)

func initIpmiutil() {
	ipmiutilOnce.Do(func() {
		// 1. Extract embedded binaries (when built with -tags embed_ipmiutil).
		//    ipmiutil.exe depends on several DLLs — all must live in the same
		//    directory so Windows DLL search finds them when launching the exe.
		const embedDir = "drivers/ipmiutil"
		if entries, err := ipmiutilFS.ReadDir(embedDir); err == nil && len(entries) > 0 {
			dir := filepath.Join(os.TempDir(), "hwmon_ipmiutil")
			if err := os.MkdirAll(dir, 0700); err == nil {
				allOK := true
				for _, e := range entries {
					data, err := ipmiutilFS.ReadFile(embedDir + "/" + e.Name())
					if err != nil {
						continue
					}
					if err := os.WriteFile(filepath.Join(dir, e.Name()), data, 0700); err != nil {
						allOK = false
						break
					}
				}
				if allOK {
					ipmiutilBin = filepath.Join(dir, "ipmiutil.exe")
					ipmiutilTempBin = dir
					ipmiutilReady = true
					log.Printf("ipmi: using embedded ipmiutil")
					return
				}
			}
		}

		// 2. Look next to the running executable (so ipmiutil.exe + DLLs can
		//    live in the same directory as hwmonitor.exe without PATH changes).
		if exe, err := os.Executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exe), "ipmiutil.exe")
			if _, err := os.Stat(candidate); err == nil {
				ipmiutilBin = candidate
				ipmiutilReady = true
				log.Printf("ipmi: ipmiutil found at %s", candidate)
				return
			}
		}

		// 3. Fall back to PATH and common install locations.
		candidates := []string{
			"ipmiutil",
			`C:\Program Files\ipmiutil\ipmiutil.exe`,
			`C:\Program Files (x86)\ipmiutil\ipmiutil.exe`,
			`C:\ipmiutil\ipmiutil.exe`,
		}
		for _, c := range candidates {
			if path, err := exec.LookPath(c); err == nil {
				ipmiutilBin = path
				ipmiutilReady = true
				log.Printf("ipmi: ipmiutil found at %s", path)
				return
			}
			if _, err := os.Stat(c); err == nil {
				ipmiutilBin = c
				ipmiutilReady = true
				log.Printf("ipmi: ipmiutil found at %s", c)
				return
			}
		}
		log.Printf("ipmi: ipmiutil not found — place ipmiutil.exe (+ ipmiutillib.dll) next to hwmonitor.exe for BMC sensor data")
	})
}

// CleanupIPMI removes the temporary extracted ipmiutil directory (if any).
func CleanupIPMI() {
	if ipmiutilTempBin != "" {
		os.RemoveAll(ipmiutilTempBin)
	}
}

// msIPMISensor is a WMI record from root\WMI MSIPMISensor (available when
// ipmidrv.sys is loaded on Windows Server — no external binary needed).
type msIPMISensor struct {
	SensorName     string
	SensorType     uint32
	CurrentReading uint32
	UnitType       uint32
	IsValid        bool
}

// collectViaWMI queries the Windows IPMI WMI provider directly.
// Returns nil if the provider is unavailable (no ipmidrv.sys or not a server).
func collectViaWMI() []Metric {
	var sensors []msIPMISensor
	if err := wmi.QueryNamespace(
		"SELECT SensorName, SensorType, CurrentReading, UnitType, IsValid FROM MSIPMISensor",
		&sensors, `root\WMI`,
	); err != nil {
		return nil
	}
	var metrics []Metric
	for _, s := range sensors {
		if !s.IsValid {
			continue
		}
		// SensorType 1 = Temperature, UnitType 1 = degrees C
		if s.SensorType != 1 || s.UnitType != 1 {
			continue
		}
		val := float64(s.CurrentReading)
		if val < -50 || val > 200 {
			continue
		}
		metrics = append(metrics, Metric{
			Name:   "ipmi_temperature_celsius",
			Value:  val,
			Labels: map[string]string{"sensor": strings.TrimSpace(s.SensorName)},
		})
	}
	return metrics
}

func (c *IPMICollector) Collect() ([]Metric, error) {
	// First try native WMI IPMI provider (works on Windows Server with
	// ipmidrv.sys — no external binary needed).
	if wmiMetrics := collectViaWMI(); len(wmiMetrics) > 0 {
		return wmiMetrics, nil
	}

	// Fall back to ipmiutil if available.
	initIpmiutil()
	if !ipmiutilReady {
		return nil, nil
	}
	return runIpmiutil()
}

// runIpmiutil runs `ipmiutil sensor` and parses all sensor readings.
//
// ipmiutil sensor output format:
//
//	SDR Full 01 01 20 a 01 snum 06 Sys_Temp1 = 1c OK 28.00 degrees C
//	SDR Full 01 01 22 a 01 snum 16 FAN2      = 0b OK 880.00 RPM
//	SDR Full 01 01 23 a 01 snum 30 P12V      = 02 OK 12.00 Volts
func runIpmiutil() ([]Metric, error) {
	out, err := exec.Command(ipmiutilBin, "sensor").Output()
	if err != nil && len(out) == 0 {
		return nil, err
	}

	var metrics []Metric
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		name, unit, value, ok := parseIpmiutilLine(scanner.Text())
		if !ok {
			continue
		}
		switch {
		case strings.Contains(unit, "degrees C"):
			// Skip control/offset registers — not real temperatures.
			if strings.Contains(strings.ToLower(name), "tcontrol") {
				continue
			}
			if value >= -50 && value <= 200 {
				metrics = append(metrics, Metric{
					Name:   "ipmi_temperature_celsius",
					Value:  value,
					Labels: map[string]string{"sensor": name},
				})
			}
		case strings.Contains(unit, "RPM"):
			if value >= 0 {
				metrics = append(metrics, Metric{
					Name:   "ipmi_fan_rpm",
					Value:  value,
					Labels: map[string]string{"sensor": name},
				})
			}
		case strings.Contains(unit, "Volts"):
			metrics = append(metrics, Metric{
				Name:   "ipmi_voltage_volts",
				Value:  value,
				Labels: map[string]string{"sensor": name},
			})
		}
	}
	return metrics, nil
}

// parseIpmiutilLine extracts (name, unit, value) from one ipmiutil sensor line.
// Expected format: ... <name> = <hex> <status> <float> <units...>
func parseIpmiutilLine(line string) (name, unit string, value float64, valid bool) {
	// Work with ASCII only — ipmiutil may embed non-printable bytes in names.
	line = sanitizeASCII(line)

	eqIdx := strings.Index(line, " = ")
	if eqIdx < 0 {
		return
	}
	// Sensor name is the last word before " = "
	prefixFields := strings.Fields(line[:eqIdx])
	if len(prefixFields) == 0 {
		return
	}
	name = prefixFields[len(prefixFields)-1]
	if name == "" {
		return
	}

	// After " = ": hex_reading status float_value units...
	rest := strings.Fields(line[eqIdx+3:])
	if len(rest) < 4 {
		return
	}
	// rest[0] = hex reading, rest[1] = status, rest[2] = value, rest[3+] = units
	if !strings.EqualFold(rest[1], "OK") {
		return
	}
	val, err := strconv.ParseFloat(rest[2], 64)
	if err != nil {
		return
	}
	return name, strings.Join(rest[3:], " "), val, true
}

// sanitizeASCII strips non-printable and non-ASCII bytes from s,
// keeping spaces and printable ASCII (0x20–0x7E).
func sanitizeASCII(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 && c <= 0x7E {
			b = append(b, c)
		}
	}
	return string(b)
}
