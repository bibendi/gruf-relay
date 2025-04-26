package config

import (
	"fmt"
	"os"
	"time"

	"github.com/ilyakaznacheev/cleanenv"
)

var (
	defaultConfigPath = "gruf-relay.yml"
	defaultConfig     *Config
)

func MustLoadConfig() *Config {
	cfgPath, ok := os.LookupEnv("CONFIG_PATH")
	if !ok {
		cfgPath = defaultConfigPath
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %s", err))
	}
	defaultConfig = cfg
	return cfg
}

func DefaultConfig() *Config {
	if defaultConfig == nil {
		return MustLoadConfig()
	}
	return defaultConfig
}

type Config struct {
	Log         Log
	Server      Server
	Workers     Workers
	HealthCheck HealthCheck `yaml:"health_check"`
	Probes      Probes
	Metrics     Metrics
}

type Log struct {
	Level  string `yaml:"level" env:"LOG_LEVEL" env-default:"debug"`
	Format string `yaml:"format" env:"LOG_FORMAT" env-default:"json"`
}

type Server struct {
	Host string `yaml:"host" env:"SERVER_HOST" env-default:"0.0.0.0"`
	Port int    `yaml:"port" env:"SERVER_PORT" env-default:"8080"`
}

type Workers struct {
	Count       int    `yaml:"count" env:"WORKERS_COUNT" env-default:"2"`
	StartPort   int    `yaml:"start_port" env:"WORKERS_START_PORT" env-default:"9000"`
	MetricsPath string `yaml:"metrics_path" env:"WORKERS_METRICS_PATH" env-default:"/metrics"`
}

type HealthCheck struct {
	Interval time.Duration `yaml:"interval" env:"HEALTH_CHECK_INTERVAL" env-default:"5s"`
	Timeout  time.Duration `yaml:"timeout" env:"HEALTH_CHECK_TIMEOUT" env-default:"3s"`
}

type Probes struct {
	Enabled bool `yaml:"enabled" env:"PROBES_ENABLED" env-default:"true"`
	Port    int  `yaml:"port" env:"PROBES_PORT" env-default:"5555"`
}

type Metrics struct {
	Enabled  bool          `yaml:"enabled" env:"METRICS_ENABLED" env-default:"true"`
	Port     int           `yaml:"port" env:"METRICS_PORT" env-default:"9394"`
	Path     string        `yaml:"path" env:"METRICS_PATH" env-default:"/metrics"`
	Interval time.Duration `yaml:"interval" env:"METRICS_INTERVAL" env-default:"5s"`
}

func loadConfig(filename string) (*Config, error) {
	var config Config

	if _, err := os.Stat(filename); err != nil {
		if err := cleanenv.ReadEnv(&config); err != nil {
			return nil, fmt.Errorf("failed to read environment variables: %w", err)
		}
	} else {
		if err := cleanenv.ReadConfig(filename, &config); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	if err := config.validateConfig(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

func (c *Config) validateConfig() error {
	if c.Server.Port <= 0 {
		return fmt.Errorf("port must be a positive integer")
	}

	if c.HealthCheck.Interval <= 0 {
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
