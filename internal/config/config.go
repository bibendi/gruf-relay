package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

var AppConfig *Config

func MustLoadConfig() *Config {
	cfgPath, ok := os.LookupEnv("CONFIG_PATH")
	if !ok {
		cfgPath = "config/gruf-relay.yml"
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %s", err))
	}
	AppConfig = cfg
	return cfg
}

type Config struct {
	LogLevel            string        `yaml:"log_level" env:"LOG_LEVEL" env-default:"debug"`
	LogFormat           string        `yaml:"log_format" env:"LOG_FORMAT" env-default:"json"`
	Host                string        `yaml:"host" env:"HOST" env-default:"0.0.0.0"`
	Port                int           `yaml:"port" env:"PORT" env-default:"8080"`
	HealthCheckInterval time.Duration `yaml:"health_check_interval" env:"HEALTH_CHECK_INTERVAL" env-default:"5s"`
	Workers             Workers
	Probes              Probes
	Metrics             Metrics
}

type Workers struct {
	Count       int    `yaml:"count" env:"WORKERS_COUNT" env-default:"2"`
	StartPort   int    `yaml:"start_port" env:"WORKERS_START_PORT" env-default:"9000"`
	MetricsPath string `yaml:"metrics_path" env:"WORKERS_METRICS_PATH" env-default:"/metrics"`
}

type Probes struct {
	Enabled bool `yaml:"enabled" env:"PROBES_ENABLED" env-default:"true"`
	Port    int  `yaml:"port" env:"PROBES_PORT" env-default:"5555"`
}

type Metrics struct {
	Enabled bool   `yaml:"enabled" env:"METRICS_ENABLED" env-default:"true"`
	Port    int    `yaml:"metrics_port" env:"METRICS_PORT" env-default:"9394"`
	Path    string `yaml:"metrics_path" env:"METRICS_PATH" env-default:"/metrics"`
}

func loadConfig(filename string) (*Config, error) {
	var config Config

	if err := cleanenv.ReadConfig(filename, &config); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := config.validateConfig(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

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
