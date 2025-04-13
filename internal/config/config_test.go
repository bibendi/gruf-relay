package config

import (
	"os"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Config Suite")
}

var _ = Describe("Config", func() {
	Describe("LoadConfig", func() {
		var content []byte

		BeforeEach(func() {
			content = []byte(`
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
		})

		It("should load config from file", func() {
			tmpfile, err := os.CreateTemp("", "config-*.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpfile.Name())

			_, err = tmpfile.Write(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpfile.Close()).NotTo(HaveOccurred())

			cfg, err := LoadConfig(tmpfile.Name())
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).NotTo(BeNil())
		})

		It("should return error for invalid file", func() {
			_, err := LoadConfig("nonexistent.yaml")
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("ValidateConfig", func() {
		var config Config

		BeforeEach(func() {
			// Initialize with a valid configuration
			config = Config{
				Port:                8080,
				HealthCheckInterval: 5 * time.Second,
				Workers: Workers{
					Count:     2,
					StartPort: 9000,
				},
			}
		})

		Context("valid config", func() {
			It("should not return an error", func() {
				Expect(config.validateConfig()).NotTo(HaveOccurred())
			})
		})

		DescribeTable("invalid config",
			func(setup func(config *Config), valid bool) {
				config := &Config{
					Port:                8080,
					HealthCheckInterval: 1,
					Workers: Workers{
						Count:     2,
						StartPort: 9000,
					},
				}
				setup(config)
				err := config.validateConfig()
				if valid {
					Expect(err).ToNot(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
				}
			},
			Entry("invalid port", func(config *Config) { config.Port = 0 }, false),
			Entry("invalid health check interval", func(config *Config) { config.HealthCheckInterval = 0 }, false),
			Entry("invalid workers count", func(config *Config) { config.Workers.Count = 0 }, false),
			Entry("invalid workers start port", func(config *Config) { config.Workers.StartPort = 0 }, false),
		)
	})
})
