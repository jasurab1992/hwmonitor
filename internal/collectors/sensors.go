//go:build windows

package collectors

import (
	"log"
	"strings"

	"github.com/yusufpapurcu/wmi"
)

// lhmSensor maps to the Sensor WMI class in LibreHardwareMonitor / OpenHardwareMonitor.
// SensorType values: "Temperature", "Voltage", "Fan", "Load", "Power", "Clock",
// "Control", "Level", "Factor", "Data", "SmallData", "Throughput", "Energy"
type lhmSensor struct {
	Name       string
	Identifier string
	SensorType string
	Parent     string
	Value      float32
}

// SensorsCollector reads hardware sensors from LibreHardwareMonitor or
// OpenHardwareMonitor WMI provider. Returns nil if neither is running.
type SensorsCollector struct{}

func NewSensorsCollector() *SensorsCollector {
	return &SensorsCollector{}
}

func (s *SensorsCollector) Name() string {
	return "sensors"
}

func (s *SensorsCollector) Collect() ([]Metric, error) {
	// Try LibreHardwareMonitor first (more actively maintained)
	if metrics, err := querySensors(`root\LibreHardwareMonitor`); err == nil && len(metrics) > 0 {
		return metrics, nil
	}

	// Try OpenHardwareMonitor
	if metrics, err := querySensors(`root\OpenHardwareMonitor`); err == nil && len(metrics) > 0 {
		return metrics, nil
	}

	log.Printf("sensors: LibreHardwareMonitor/OpenHardwareMonitor not found — install LHM as a service for voltages and per-core temps")
	return nil, nil
}

func querySensors(namespace string) ([]Metric, error) {
	var sensors []lhmSensor
	q := wmi.CreateQuery(&sensors, "")
	if err := wmi.QueryNamespace(q, &sensors, namespace); err != nil {
		return nil, err
	}

	var metrics []Metric
	for _, s := range sensors {
		var metricName string
		switch strings.ToLower(s.SensorType) {
		case "temperature":
			metricName = "sensor_temperature_celsius"
		case "voltage":
			metricName = "sensor_voltage_volts"
		case "fan":
			metricName = "sensor_fan_rpm"
		case "load":
			metricName = "sensor_load_percent"
		case "power":
			metricName = "sensor_power_watts"
		case "clock":
			metricName = "sensor_clock_mhz"
		default:
			continue
		}

		metrics = append(metrics, Metric{
			Name:  metricName,
			Value: float64(s.Value),
			Labels: map[string]string{
				"name":       s.Name,
				"identifier": s.Identifier,
				"type":       s.SensorType,
			},
		})
	}
	return metrics, nil
}
