package collectors

// Metric represents a single collected metric value with optional labels.
type Metric struct {
	Name   string
	Value  float64
	Labels map[string]string
}

// Collector is the interface that all metric collectors must implement.
type Collector interface {
	Name() string
	Collect() ([]Metric, error)
}
