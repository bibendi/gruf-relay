package logger

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/dsl/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Logger", func() {
	Describe("NewLogger", func() {
		var (
			oldStdout *os.File
			r         *os.File
			w         *os.File
		)

		BeforeEach(func() {
			// Redirect stdout to capture log output
			oldStdout = os.Stdout
			r, w, _ = os.Pipe()
			os.Stdout = w
		})

		AfterEach(func() {
			// Restore stdout
			w.Close()
			os.Stdout = oldStdout
			// Reset default logger to avoid interference
			slog.SetDefault(slog.Default())
		})

		table.DescribeTable("Logger configurations",
			func(level string, format string, expectedJSON bool, shouldPanic bool) {
				if shouldPanic {
					Expect(func() { NewLogger(level, format) }).Should(Panic())
				} else {
					logger := NewLogger(level, format)
					logger.Log(context.Background(), slog.LevelInfo, "Test log message")

					var buf bytes.Buffer
					buf.ReadFrom(r)
					output := buf.String()

					Expect(output).ShouldNot(BeEmpty(), "Expected log output")
					Expect(strings.Contains(output, "Test log message")).Should(BeTrue(), "Expected log message in output")
					Expect(isJSON(output)).Should(Equal(expectedJSON), fmt.Sprintf("Expected JSON format: %v", expectedJSON))
				}
			},
			table.Entry("Valid JSON Debug", "debug", "json", true, false),
			table.Entry("Valid Text Info", "info", "text", false, false),
			table.Entry("Valid JSON Warn", "warn", "json", true, false),
			table.Entry("Valid Text Error", "error", "text", false, false),
			table.Entry("Invalid Level", "invalid", "json", false, true),
			table.Entry("Invalid Format", "info", "invalid", false, true),
		)
	})
})

func isJSON(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}\n")
}
