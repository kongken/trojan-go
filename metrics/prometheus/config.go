package prometheus

import "github.com/p4gefau1t/trojan-go/config"

const Name = "METRICS_PROMETHEUS"

// Config holds the configuration for Prometheus metrics
type Config struct {
	Metrics MetricsConfig `json:"metrics" yaml:"metrics"`
}

// MetricsConfig holds specific metrics settings
type MetricsConfig struct {
	Enabled bool   `json:"enabled" yaml:"enabled"`
	Host    string `json:"host" yaml:"host"`
	Port    int    `json:"port" yaml:"port"`
	Path    string `json:"path" yaml:"path"`
}

func init() {
	config.RegisterConfigCreator(Name, func() interface{} {
		return &Config{
			Metrics: MetricsConfig{
				Enabled: false,
				Host:    "127.0.0.1",
				Port:    9100,
				Path:    "/metrics",
			},
		}
	})
}
