package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Host                string        `yaml:"host" env-default:"0.0.0.0"`
	Port                int           `yaml:"port" env-default:"8080"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
	Workers             WorkersConfig `yaml:"workers"`
}

type WorkersConfig struct {
	Count     int `yaml:"count" env-default:"2"`
	StartPort int `yaml:"start_port" env-default:"9000"`
}

func LoadConfig(filename string) (*Config, error) {
	yamlFile, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	config.setDefaultValues()
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

func (c *Config) setDefaultValues() {
	if c.HealthCheckInterval == 0 {
		c.HealthCheckInterval = 5 * time.Second
	}
	if c.Workers.Count == 0 {
		c.Workers.Count = 2
	}
	if c.Workers.StartPort == 0 {
		c.Workers.StartPort = 9000
	}
}

func validateConfig(cfg *Config) error {
	if cfg.Port <= 0 {
		return fmt.Errorf("port must be a positive integer")
	}

	if cfg.HealthCheckInterval <= 0 {
		return fmt.Errorf("health_check_interval must be a positive duration")
	}

	if cfg.Workers.Count <= 0 {
		return fmt.Errorf("workers count must be a positive integer")
	}

	if cfg.Workers.StartPort <= 0 {
		return fmt.Errorf("workers start_port must be a positive integer")
	}

	return nil
}
