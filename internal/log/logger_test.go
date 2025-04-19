package log

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
		var logCfg config.Log

		BeforeEach(func() {
			logCfg = config.Log{
				Level:  "info",
				Format: "text",
			}

			DeferCleanup(func() {
				MustInitLogger(config.DefaultConfig().Log)
			})
		})

		It("should panic on invalid log level", func() {
			logCfg.Level = "invalid"
			Expect(func() { MustInitLogger(logCfg) }).To(Panic())
		})

		It("should panic on invalid log format", func() {
			logCfg.Format = "invalid"
			Expect(func() { MustInitLogger(logCfg) }).To(Panic())
		})

		It("should return a Logger instance", func() {
			Expect(MustInitLogger(logCfg)).NotTo(BeNil())
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
