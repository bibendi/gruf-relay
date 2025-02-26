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
	ProxyPort           int           `yaml:"proxy_port"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval"`
	RubyServers         []RubyServer  `yaml:"ruby_servers"`
}

// RubyServer представляет конфигурацию одного Ruby GRPC сервера.
type RubyServer struct {
	Name    string   `yaml:"name"`
	Host    string   `yaml:"host"`
	Port    int      `yaml:"port"`
	Command []string `yaml:"command"`
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

// validateConfig выполняет валидацию конфигурации.
func validateConfig(cfg *Config) error {
	if cfg.ProxyPort <= 0 {
		return fmt.Errorf("proxy_port must be a positive integer")
	}

	if cfg.HealthCheckInterval <= 0 {
		return fmt.Errorf("health_check_interval must be a positive duration")
	}

	if len(cfg.RubyServers) == 0 {
		return fmt.Errorf("at least one ruby_server must be defined")
	}

	for _, server := range cfg.RubyServers {
		if server.Name == "" {
			return fmt.Errorf("ruby_server name cannot be empty")
		}
		if server.Host == "" {
			return fmt.Errorf("ruby_server host cannot be empty")
		}
		if server.Port <= 0 {
			return fmt.Errorf("ruby_server port must be a positive integer")
		}
		if len(server.Command) == 0 {
			return fmt.Errorf("ruby_server command cannot be empty")
		}
	}

	return nil
}
