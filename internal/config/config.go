package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// CollectorsConfig controls which collectors are enabled.
type CollectorsConfig struct {
	CPU     bool `yaml:"cpu"`
	Memory  bool `yaml:"memory"`
	Disk    bool `yaml:"disk"`
	NVMe    bool `yaml:"nvme"`
	CPUTemp bool `yaml:"cpu_temp"`
	SMART   bool `yaml:"smart"`
	Network bool `yaml:"network"`
	SysInfo bool `yaml:"sysinfo"`
	Sensors bool `yaml:"sensors"`
}

// Config holds the application configuration.
type Config struct {
	PrometheusPort  int              `yaml:"prometheus_port"`
	CollectInterval time.Duration    `yaml:"collect_interval"`
	Collectors      CollectorsConfig `yaml:"collectors"`
}

// DefaultConfig returns a Config with default values.
func DefaultConfig() *Config {
	return &Config{
		PrometheusPort:  9100,
		CollectInterval: 5 * time.Second,
		Collectors: CollectorsConfig{
			CPU:     true,
			Memory:  true,
			Disk:    true,
			NVMe:    true,
			CPUTemp: true,
			SMART:   true,
			Network: true,
			SysInfo: true,
			Sensors: true,
		},
	}
}

// LoadConfig reads the YAML config from the given path.
// If the file does not exist, default values are returned.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
