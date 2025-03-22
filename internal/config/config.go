// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config представляет структуру конфигурации приложения.
type Config struct {
	Host                string        `yaml:"host" env-default:"0.0.0.0"`
	Port                int           `yaml:"port" env-default:"8080"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
	Workers             WorkersConfig `yaml:"workers"`
}

// WorkersConfig представляет конфигурацию воркеров (Ruby серверов).
type WorkersConfig struct {
	Count     int `yaml:"count"`
	StartPort int `yaml:"start_port"`
}

// LoadConfig загружает конфигурацию из YAML файла.
func LoadConfig(filename string) (*Config, error) {
	// 1. Чтение файла
	yamlFile, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 2. Разбор YAML
	var config Config
	config.setDefaultValues()
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 3. Валидация конфигурации (опционально, но рекомендуется)
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// setDefaultValues устанавливает значения по умолчанию для конфигурации.
func (c *Config) setDefaultValues() {
	// if c.Host == "" {
	// 	c.Host = "0.0.0.0"
	// }
	// if c.Port == 0 {
	// 	c.Port = 8080
	// }
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

// validateConfig выполняет валидацию конфигурации.
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
