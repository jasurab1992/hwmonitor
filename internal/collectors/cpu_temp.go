//go:build windows

package collectors

import (
	"fmt"
	"log"

	"github.com/yusufpapurcu/wmi"
)

// msAcpiThermalZone represents a WMI MSAcpi_ThermalZoneTemperature instance.
type msAcpiThermalZone struct {
	InstanceName       string
	CurrentTemperature uint32
}

// CPUTempCollector collects CPU temperature via WMI thermal zone data.
type CPUTempCollector struct{}

func NewCPUTempCollector() *CPUTempCollector {
	return &CPUTempCollector{}
}

func (c *CPUTempCollector) Name() string {
	return "cpu_temp"
}

func (c *CPUTempCollector) Collect() ([]Metric, error) {
	var zones []msAcpiThermalZone
	query := "SELECT InstanceName, CurrentTemperature FROM MSAcpi_ThermalZoneTemperature"
	err := wmi.QueryNamespace(query, &zones, `root\wmi`)
	if err != nil {
		log.Printf("cpu_temp: WMI thermal zone query failed (admin rights may be required): %v", err)
		return nil, nil
	}

	var metrics []Metric
	for i, z := range zones {
		celsius := (float64(z.CurrentTemperature) - 2732.0) / 10.0
		metrics = append(metrics, Metric{
			Name:  "cpu_temperature_celsius",
			Value: celsius,
			Labels: map[string]string{
				"zone": fmt.Sprintf("TZ%02d", i),
			},
		})
	}

	return metrics, nil
}
