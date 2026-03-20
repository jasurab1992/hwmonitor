package collectors

import (
	"strings"

	"github.com/shirou/gopsutil/v3/net"
)

// NetworkCollector collects per-interface network I/O metrics.
type NetworkCollector struct{}

func NewNetworkCollector() *NetworkCollector {
	return &NetworkCollector{}
}

func (n *NetworkCollector) Name() string {
	return "network"
}

func (n *NetworkCollector) Collect() ([]Metric, error) {
	counters, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}

	var metrics []Metric
	for _, c := range counters {
		// Skip loopback interfaces
		name := c.Name
		if name == "lo" || strings.Contains(strings.ToLower(name), "loopback") {
			continue
		}

		metrics = append(metrics,
			Metric{Name: "network_bytes_sent_total", Value: float64(c.BytesSent), Labels: map[string]string{"interface": name}},
			Metric{Name: "network_bytes_recv_total", Value: float64(c.BytesRecv), Labels: map[string]string{"interface": name}},
			Metric{Name: "network_packets_sent_total", Value: float64(c.PacketsSent), Labels: map[string]string{"interface": name}},
			Metric{Name: "network_packets_recv_total", Value: float64(c.PacketsRecv), Labels: map[string]string{"interface": name}},
			Metric{Name: "network_errors_in_total", Value: float64(c.Errin), Labels: map[string]string{"interface": name}},
			Metric{Name: "network_errors_out_total", Value: float64(c.Errout), Labels: map[string]string{"interface": name}},
		)
	}

	return metrics, nil
}
