package exporter

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"hwmonitor/internal/collectors"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Exporter collects metrics from all registered collectors and serves them via HTTP.
type Exporter struct {
	collectors []collectors.Collector
	port       int
	descs      map[string]*prometheus.Desc
}

// NewExporter creates a new Prometheus Exporter.
func NewExporter(colls []collectors.Collector, port int) *Exporter {
	return &Exporter{
		collectors: colls,
		port:       port,
		descs:      make(map[string]*prometheus.Desc),
	}
}

// Describe implements prometheus.Collector. We use DescribeByCollect since
// metric names are dynamic.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	// Use DescribeByCollect approach — no static descriptors needed.
}

// Collect implements prometheus.Collector. It calls Collect() on all registered
// collectors and converts the results to Prometheus metrics.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	for _, c := range e.collectors {
		metrics, err := c.Collect()
		if err != nil {
			log.Printf("collector %s error: %v", c.Name(), err)
			continue
		}

		for _, m := range metrics {
			labelNames := make([]string, 0, len(m.Labels))
			labelValues := make([]string, 0, len(m.Labels))
			for k, v := range m.Labels {
				labelNames = append(labelNames, k)
				labelValues = append(labelValues, v)
			}

			desc := prometheus.NewDesc(
				prometheus.BuildFQName("hwmonitor", "", m.Name),
				m.Name,
				labelNames,
				nil,
			)

			valueType := prometheus.GaugeValue
			if strings.HasSuffix(m.Name, "_total") {
				valueType = prometheus.CounterValue
			}

			metric, err := prometheus.NewConstMetric(desc, valueType, m.Value, labelValues...)
			if err != nil {
				log.Printf("failed to create metric %s: %v", m.Name, err)
				continue
			}
			ch <- metric
		}
	}
}

// Start registers the exporter and starts the HTTP server on the configured port.
// This method blocks until the server exits.
func (e *Exporter) Start() error {
	registry := prometheus.NewRegistry()
	registry.MustRegister(e)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	addr := fmt.Sprintf(":%d", e.port)
	log.Printf("Prometheus exporter listening on %s/metrics", addr)
	return http.ListenAndServe(addr, mux)
}
