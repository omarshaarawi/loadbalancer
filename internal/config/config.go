package config

import (
	"encoding/json"
	"os"
	"time"
)

type Config struct {
	Port         string        `json:"port"`
	ReadTimeout  time.Duration `json:"read_timeout"`
	WriteTimeout time.Duration `json:"write_timeout"`

	ProbeInterval    time.Duration `json:"probe_interval"`
	ProbeTimeout     time.Duration `json:"probe_timeout"`
	HealthCheckPath  string        `json:"health_check_path"`
	SelectionChoices int           `json:"selection_choices"`

	Servers []ServerConfig `json:"servers"`

	MetricsPort string `json:"metrics_port"`
}

type ServerConfig struct {
	ID      string `json:"id"`
	Address string `json:"address"`
	Weight  int    `json:"weight"`
}

func LoadConfig(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	config := &Config{}
	if err := json.NewDecoder(file).Decode(config); err != nil {
		return nil, err
	}

	if config.Port == "" {
		config.Port = "8080"
	}
	if config.ReadTimeout == 0 {
		config.ReadTimeout = 5 * time.Second
	}
	if config.WriteTimeout == 0 {
		config.WriteTimeout = 10 * time.Second
	}
	if config.ProbeInterval == 0 {
		config.ProbeInterval = time.Second
	}
	if config.ProbeTimeout == 0 {
		config.ProbeTimeout = 2 * time.Second
	}
	if config.HealthCheckPath == "" {
		config.HealthCheckPath = "/health"
	}
	if config.SelectionChoices == 0 {
		config.SelectionChoices = 2
	}

	return config, nil
}
