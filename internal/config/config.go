package config

import (
	"fmt"
	"log"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	Host                string        `yaml:"host" env:"HOST" env-default:"0.0.0.0"`
	Port                int           `yaml:"port" env:"PORT" env-default:"8080"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" env:"HEALTH_CHECK_INTERVAL" env-default:"5s"`
	Workers             struct {
		Count     int `yaml:"count" env:"WORKERS_COUNT" env-default:"2"`
		StartPort int `yaml:"start_port" env:"WORKERS_START_PORT" env-default:"9000"`
	}
}

func LoadConfig(filename string) (*Config, error) {
	var config Config

	if err := cleanenv.ReadConfig(filename, &config); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := config.validateConfig(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	log.Printf("Configuration loaded: %+v", config)

	return &config, nil
}

func (c *Config) validateConfig() error {
	if c.Port <= 0 {
		return fmt.Errorf("port must be a positive integer")
	}

	if c.HealthCheckInterval <= 0 {
		return fmt.Errorf("health_check_interval must be a positive duration")
	}

	if c.Workers.Count <= 0 {
		return fmt.Errorf("workers count must be a positive integer")
	}

	if c.Workers.StartPort <= 0 {
		return fmt.Errorf("workers start_port must be a positive integer")
	}

	return nil
}
