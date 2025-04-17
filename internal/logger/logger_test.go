package logger

import (
	"bytes"
	"log/slog"
	"os"
	"testing"

	"github.com/bibendi/gruf-relay/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLogger(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Logger Suite")
}

var _ = Describe("Logger", func() {
	Describe("MustInitLogger", func() {
		var (
			originalConfig *config.Config
		)

		BeforeEach(func() {
			originalConfig = config.AppConfig
			DeferCleanup(func() {
				config.AppConfig = originalConfig
			})
		})

		It("should panic on invalid log level", func() {
			config.AppConfig = &config.Config{LogLevel: "invalid"}
			Expect(func() { MustInitLogger() }).To(Panic())
		})

		It("should panic on invalid log format", func() {
			config.AppConfig = &config.Config{LogFormat: "invalid"}
			Expect(func() { MustInitLogger() }).To(Panic())
		})

		It("should return a Logger instance", func() {
			config.AppConfig = &config.Config{LogLevel: "debug", LogFormat: "json"}
			l := MustInitLogger()
			Expect(l).NotTo(BeNil())
		})

		It("should set the global AppLogger and slog default", func() {
			config.AppConfig = &config.Config{LogLevel: "info", LogFormat: "text"}
			l := MustInitLogger()
			Expect(AppLogger).To(Equal(l))
		})
	})

	Describe("newLogger", func() {
		It("should create a JSON logger", func() {
			buffer := &bytes.Buffer{}
			l, err := newLogger(buffer, slog.LevelInfo, LogFormatJSON)
			Expect(err).NotTo(HaveOccurred())
			Expect(l).NotTo(BeNil())

			l.Info("test message", "key", "value")
			output := buffer.String()
			Expect(output).To(ContainSubstring(`"msg":"test message"`))
			Expect(output).To(ContainSubstring(`"key":"value"`))
		})

		It("should create a Text logger", func() {
			buffer := &bytes.Buffer{}
			l, err := newLogger(buffer, slog.LevelInfo, LogFormatText)
			Expect(err).NotTo(HaveOccurred())
			Expect(l).NotTo(BeNil())

			l.Info("test message", "key", "value")
			output := buffer.String()
			Expect(output).To(ContainSubstring(`msg="test message"`))
			Expect(output).To(ContainSubstring("key=value"))
		})

		It("should create a Pretty logger", func() {
			buffer := &bytes.Buffer{}
			l, err := newLogger(buffer, slog.LevelInfo, LogFormatPretty)
			Expect(err).NotTo(HaveOccurred())
			Expect(l).NotTo(BeNil())

			l.Info("test message", "key", "value")
			output := buffer.String()
			Expect(output).To(ContainSubstring("test message"))
			Expect(output).To(ContainSubstring("key"))
			Expect(output).To(ContainSubstring("value"))
		})

		It("should return an error for an invalid log format", func() {
			_, err := newLogger(os.Stdout, slog.LevelInfo, LogFormat("invalid"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid log format"))
		})
	})
})
