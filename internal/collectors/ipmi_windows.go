//go:build windows

package collectors

import (
	"bufio"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
)

// IPMICollector collects BMC/IPMI sensor temperatures (inlet, outlet, ambient, etc.)
// via ipmitool. It is a no-op if ipmitool is not installed.
type IPMICollector struct{}

func NewIPMICollector() *IPMICollector { return &IPMICollector{} }
func (c *IPMICollector) Name() string  { return "ipmi" }

var (
	ipmitoolOnce  sync.Once
	ipmitoolBin   string
	ipmitoolReady bool
)

func initIpmitool() {
	ipmitoolOnce.Do(func() {
		candidates := []string{
			"ipmitool",
			`C:\Program Files\ipmitool\ipmitool.exe`,
			`C:\Program Files (x86)\ipmitool\ipmitool.exe`,
			`C:\ipmitool\ipmitool.exe`,
		}
		for _, c := range candidates {
			if path, err := exec.LookPath(c); err == nil {
				ipmitoolBin = path
				ipmitoolReady = true
				log.Printf("ipmi: ipmitool found at %s", path)
				return
			}
			if _, err := os.Stat(c); err == nil {
				ipmitoolBin = c
				ipmitoolReady = true
				log.Printf("ipmi: ipmitool found at %s", c)
				return
			}
		}
		log.Printf("ipmi: ipmitool not found — install for BMC/ambient temperatures")
	})
}

func (c *IPMICollector) Collect() ([]Metric, error) {
	initIpmitool()
	if !ipmitoolReady {
		return nil, nil
	}

	// Try Microsoft IPMI WMI interface first (built into Windows Server when
	// ipmidrv.sys is loaded — no extra driver install needed on most servers).
	metrics, _ := runIpmitool("-I", "ms", "sdr", "type", "Temperature")
	if len(metrics) == 0 {
		// Fall back to auto-detected interface.
		metrics, _ = runIpmitool("sdr", "type", "Temperature")
	}
	return metrics, nil
}

// runIpmitool runs ipmitool with the given args and parses temperature sensors.
// ipmitool `sdr type Temperature` output format:
//
//	Inlet Temp       | 28 degrees C  | ok
//	CPU Temp         | 40 degrees C  | ok
//	Inlet Temp       | no reading    | ns
func runIpmitool(args ...string) ([]Metric, error) {
	out, err := exec.Command(ipmitoolBin, args...).Output()
	if err != nil && len(out) == 0 {
		return nil, err
	}

	var metrics []Metric
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}

		name := strings.TrimSpace(parts[0])
		rawVal := strings.TrimSpace(parts[1])
		status := strings.TrimSpace(parts[2])

		// Skip sensors with no reading or not-ok status.
		if status != "ok" || strings.Contains(rawVal, "no reading") || strings.Contains(rawVal, "Disabled") {
			continue
		}

		// Parse "28 degrees C" or "28.00 degrees C".
		rawVal = strings.ReplaceAll(rawVal, "degrees C", "")
		rawVal = strings.TrimSpace(rawVal)
		val, err := strconv.ParseFloat(rawVal, 64)
		if err != nil || val < -50 || val > 200 {
			continue
		}

		metrics = append(metrics, Metric{
			Name:  "ipmi_temperature_celsius",
			Value: val,
			Labels: map[string]string{"sensor": name},
		})
	}
	return metrics, nil
}
