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

// perfThermalZone represents a WMI Win32_PerfFormattedData_Counters_ThermalZoneInformation instance.
type perfThermalZone struct {
	Name        string
	Temperature uint32
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
	// Try primary source: MSAcpi_ThermalZoneTemperature (root\WMI)
	var zones []msAcpiThermalZone
	query := "SELECT InstanceName, CurrentTemperature FROM MSAcpi_ThermalZoneTemperature"
	err := wmi.QueryNamespace(query, &zones, `root\wmi`)
	if err == nil && len(zones) > 0 {
		var metrics []Metric
		for i, z := range zones {
			celsius := (float64(z.CurrentTemperature)/10.0 - 273.15)
			metrics = append(metrics, Metric{
				Name:  "cpu_temp_celsius",
				Value: celsius,
				Labels: map[string]string{
					"zone":   fmt.Sprintf("TZ%02d", i),
					"source": "MSAcpi",
				},
			})
		}
		return metrics, nil
	}

	log.Printf("cpu_temp: MSAcpi query failed, trying PerfCounter fallback: %v", err)

	// Fallback: Win32_PerfFormattedData_Counters_ThermalZoneInformation (root\CIMV2)
	var perfZones []perfThermalZone
	query2 := "SELECT Name, Temperature FROM Win32_PerfFormattedData_Counters_ThermalZoneInformation"
	err2 := wmi.Query(query2, &perfZones)
	if err2 == nil && len(perfZones) > 0 {
		var metrics []Metric
		for i, z := range perfZones {
			celsius := float64(z.Temperature) - 273.15
			zone := z.Name
			if zone == "" {
				zone = fmt.Sprintf("TZ%02d", i)
			}
			metrics = append(metrics, Metric{
				Name:  "cpu_temp_celsius",
				Value: celsius,
				Labels: map[string]string{
					"zone":   zone,
					"source": "PerfCounter",
				},
			})
		}
		return metrics, nil
	}

	log.Printf("cpu_temp: PerfCounter fallback also failed: %v", err2)
	return nil, nil
}
