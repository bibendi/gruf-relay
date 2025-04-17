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
	Describe("MustLoadConfig", func() {
		var originalConfigPath string
		var content []byte

		BeforeEach(func() {
			originalConfigPath = defaultConfigPath
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

		AfterEach(func() {
			defaultConfigPath = originalConfigPath
			AppConfig = nil
		})

		It("should load config from file", func() {
			tmpfile, err := os.CreateTemp("", "config-*.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpfile.Name())

			_, err = tmpfile.Write(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpfile.Close()).NotTo(HaveOccurred())

			defaultConfigPath = tmpfile.Name()
			cfg := MustLoadConfig()
			Expect(cfg).NotTo(BeNil())

			Expect(cfg.LogLevel).To(Equal("info"))
			Expect(cfg.LogFormat).To(Equal("json"))
			Expect(cfg.Host).To(Equal("127.0.0.1"))
			Expect(cfg.Port).To(Equal(8081))
			Expect(cfg.HealthCheckInterval).To(Equal(10 * time.Second))

			Expect(cfg.Workers.Count).To(Equal(4))
			Expect(cfg.Workers.StartPort).To(Equal(9001))
			Expect(cfg.Workers.MetricsPath).To(Equal("/worker-metrics"))

			Expect(cfg.Probes.Enabled).To(BeTrue())
			Expect(cfg.Probes.Port).To(Equal(5556))

			Expect(cfg.Metrics.Enabled).To(BeTrue())
			Expect(cfg.Metrics.Port).To(Equal(9395))
			Expect(cfg.Metrics.Path).To(Equal("/app-metrics"))
		})

		It("should panic if config file does not exist", func() {
			defaultConfigPath = "nonexistent_config.yaml"

			Expect(func() {
				MustLoadConfig()
			}).To(Panic())
		})

		It("should load config from env variable CONFIG_PATH", func() {
			tmpfile, err := os.CreateTemp("", "env-config-*.yaml")
			Expect(err).NotTo(HaveOccurred())
			defer os.Remove(tmpfile.Name())

			_, err = tmpfile.Write(content)
			Expect(err).NotTo(HaveOccurred())
			Expect(tmpfile.Close()).NotTo(HaveOccurred())
			os.Setenv("CONFIG_PATH", tmpfile.Name())
			defer os.Unsetenv("CONFIG_PATH")

			cfg := MustLoadConfig()
			Expect(cfg).NotTo(BeNil())
			Expect(cfg.LogLevel).To(Equal("info"))
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
