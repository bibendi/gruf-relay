package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	content := []byte(`
log_level: info
log_format: json
host: "127.0.0.1"
port: 8081
health_check_interval: 10s
workers:
  count: 4
  start_port: 9001
  metrics_path: "/worker-metrics"
probes:
  enabled: true
  port: 5556
metrics:
  enabled: true
  metrics_port: 9395
  metrics_path: "/app-metrics"
`)

	tmpfile, err := os.CreateTemp("", "config-*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpfile.Name())

	_, err = tmpfile.Write(content)
	require.NoError(t, err)
	require.NoError(t, tmpfile.Close())

	cfg, err := LoadConfig(tmpfile.Name())
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "127.0.0.1", cfg.Host)
	assert.Equal(t, 8081, cfg.Port)
	assert.Equal(t, 4, cfg.Workers.Count)
	assert.Equal(t, 9001, cfg.Workers.StartPort)
	assert.Equal(t, "/worker-metrics", cfg.Workers.MetricsPath)
	assert.True(t, cfg.Probes.Enabled)
	assert.Equal(t, 5556, cfg.Probes.Port)
	assert.True(t, cfg.Metrics.Enabled)
	assert.Equal(t, 9395, cfg.Metrics.Port)
	assert.Equal(t, "/app-metrics", cfg.Metrics.Path)
}

func TestLoadConfigInvalidFile(t *testing.T) {
	_, err := LoadConfig("nonexistent.yaml")
	assert.Error(t, err)
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				Port:                8080,
				HealthCheckInterval: 5 * time.Second,
				Workers: struct {
					Count       int    `yaml:"count" env:"WORKERS_COUNT" env-default:"2"`
					StartPort   int    `yaml:"start_port" env:"WORKERS_START_PORT" env-default:"9000"`
					MetricsPath string `yaml:"metrics_path" env:"WORKERS_METRICS_PATH" env-default:"/metrics"`
				}{
					Count:     2,
					StartPort: 9000,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid port",
			config: Config{
				Port:                0,
				HealthCheckInterval: 5 * time.Second,
				Workers: struct {
					Count       int    `yaml:"count" env:"WORKERS_COUNT" env-default:"2"`
					StartPort   int    `yaml:"start_port" env:"WORKERS_START_PORT" env-default:"9000"`
					MetricsPath string `yaml:"metrics_path" env:"WORKERS_METRICS_PATH" env-default:"/metrics"`
				}{
					Count:     2,
					StartPort: 9000,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid health check interval",
			config: Config{
				Port:                8080,
				HealthCheckInterval: 0,
				Workers: struct {
					Count       int    `yaml:"count" env:"WORKERS_COUNT" env-default:"2"`
					StartPort   int    `yaml:"start_port" env:"WORKERS_START_PORT" env-default:"9000"`
					MetricsPath string `yaml:"metrics_path" env:"WORKERS_METRICS_PATH" env-default:"/metrics"`
				}{
					Count:     2,
					StartPort: 9000,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid workers count",
			config: Config{
				Port:                8080,
				HealthCheckInterval: 5 * time.Second,
				Workers: struct {
					Count       int    `yaml:"count" env:"WORKERS_COUNT" env-default:"2"`
					StartPort   int    `yaml:"start_port" env:"WORKERS_START_PORT" env-default:"9000"`
					MetricsPath string `yaml:"metrics_path" env:"WORKERS_METRICS_PATH" env-default:"/metrics"`
				}{
					Count:     0,
					StartPort: 9000,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid workers start port",
			config: Config{
				Port:                8080,
				HealthCheckInterval: 5 * time.Second,
				Workers: struct {
					Count       int    `yaml:"count" env:"WORKERS_COUNT" env-default:"2"`
					StartPort   int    `yaml:"start_port" env:"WORKERS_START_PORT" env-default:"9000"`
					MetricsPath string `yaml:"metrics_path" env:"WORKERS_METRICS_PATH" env-default:"/metrics"`
				}{
					Count:     2,
					StartPort: 0,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.validateConfig()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
