package collectors

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
)

// smartctlOutput represents the relevant parts of smartctl JSON output.
type smartctlOutput struct {
	Device struct {
		Name string `json:"name"`
	} `json:"device"`
	NVMeSmartHealthInformationLog struct {
		Temperature         int   `json:"temperature"`
		AvailableSpare      int   `json:"available_spare"`
		PercentageUsed      int   `json:"percentage_used"`
		PowerOnHours        int64 `json:"power_on_hours"`
		PowerCycles         int64 `json:"power_cycles"`
		UnsafeShutdowns     int64 `json:"unsafe_shutdowns"`
		MediaErrors         int64 `json:"media_errors"`
	} `json:"nvme_smart_health_information_log"`
}

// NVMeCollector collects NVMe SMART health metrics via smartctl.
type NVMeCollector struct{}

// NewNVMeCollector creates a new NVMeCollector.
func NewNVMeCollector() *NVMeCollector {
	return &NVMeCollector{}
}

func (n *NVMeCollector) Name() string {
	return "nvme"
}

func (n *NVMeCollector) Collect() ([]Metric, error) {
	// Check if smartctl is available
	smartctlPath, err := exec.LookPath("smartctl")
	if err != nil {
		log.Printf("smartctl not found, skipping NVMe metrics: %v", err)
		return nil, nil
	}

	devices, err := discoverNVMeDevices(smartctlPath)
	if err != nil {
		log.Printf("failed to discover NVMe devices: %v", err)
		return nil, nil
	}

	var metrics []Metric
	for _, device := range devices {
		m, err := collectNVMeDevice(smartctlPath, device)
		if err != nil {
			log.Printf("failed to collect NVMe metrics for %s: %v", device, err)
			continue
		}
		metrics = append(metrics, m...)
	}

	return metrics, nil
}

// discoverNVMeDevices uses smartctl --scan -j to find NVMe devices.
func discoverNVMeDevices(smartctlPath string) ([]string, error) {
	out, err := exec.Command(smartctlPath, "--scan", "-j").Output()
	if err != nil {
		return nil, fmt.Errorf("smartctl --scan failed: %w", err)
	}

	var scanResult struct {
		Devices []struct {
			Name     string `json:"name"`
			InfoName string `json:"info_name"`
			Protocol string `json:"protocol"`
		} `json:"devices"`
	}

	if err := json.Unmarshal(out, &scanResult); err != nil {
		return nil, fmt.Errorf("failed to parse smartctl scan output: %w", err)
	}

	var devices []string
	for _, d := range scanResult.Devices {
		if strings.EqualFold(d.Protocol, "NVMe") {
			devices = append(devices, d.Name)
		}
	}

	return devices, nil
}

// collectNVMeDevice runs smartctl -a on a single device and parses SMART data.
func collectNVMeDevice(smartctlPath, device string) ([]Metric, error) {
	out, err := exec.Command(smartctlPath, "-a", device, "-j").Output()
	if err != nil {
		// smartctl may return non-zero exit codes even on success
		if len(out) == 0 {
			return nil, fmt.Errorf("smartctl -a %s failed: %w", device, err)
		}
	}

	var data smartctlOutput
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("failed to parse smartctl output for %s: %w", device, err)
	}

	labels := map[string]string{"device": device}
	smart := data.NVMeSmartHealthInformationLog

	metrics := []Metric{
		{Name: "nvme_temperature_celsius", Value: float64(smart.Temperature), Labels: copyLabels(labels)},
		{Name: "nvme_available_spare_percent", Value: float64(smart.AvailableSpare), Labels: copyLabels(labels)},
		{Name: "nvme_percentage_used", Value: float64(smart.PercentageUsed), Labels: copyLabels(labels)},
		{Name: "nvme_power_on_hours", Value: float64(smart.PowerOnHours), Labels: copyLabels(labels)},
		{Name: "nvme_power_cycles", Value: float64(smart.PowerCycles), Labels: copyLabels(labels)},
		{Name: "nvme_unsafe_shutdowns", Value: float64(smart.UnsafeShutdowns), Labels: copyLabels(labels)},
		{Name: "nvme_media_errors", Value: float64(smart.MediaErrors), Labels: copyLabels(labels)},
	}

	return metrics, nil
}
