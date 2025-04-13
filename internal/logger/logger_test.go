package logger

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewLogger(t *testing.T) {
	testCases := []struct {
		name           string
		level          string
		format         string
		expectedLevel  slog.Level
		expectedFormat string
		expectPanic    bool
	}{
		{
			name:           "Valid JSON Debug",
			level:          "debug",
			format:         "json",
			expectedLevel:  slog.LevelDebug,
			expectedFormat: "json",
			expectPanic:    false,
		},
		{
			name:           "Valid Text Info",
			level:          "info",
			format:         "text",
			expectedLevel:  slog.LevelInfo,
			expectedFormat: "text",
			expectPanic:    false,
		},
		{
			name:           "Valid JSON Warn",
			level:          "warn",
			format:         "json",
			expectedLevel:  slog.LevelWarn,
			expectedFormat: "json",
			expectPanic:    false,
		},
		{
			name:           "Valid Text Error",
			level:          "error",
			format:         "text",
			expectedLevel:  slog.LevelError,
			expectedFormat: "text",
			expectPanic:    false,
		},
		{
			name:           "Invalid Level",
			level:          "invalid",
			format:         "json",
			expectedLevel:  slog.LevelInfo,
			expectedFormat: "json",
			expectPanic:    true,
		},
		{
			name:           "Invalid Format",
			level:          "info",
			format:         "invalid",
			expectedLevel:  slog.LevelInfo,
			expectedFormat: "invalid",
			expectPanic:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.expectPanic {
				assert.Panics(t, func() {
					NewLogger(tc.level, tc.format)
				}, "Expected a panic")
			} else {
				// Redirect stdout to capture log output
				oldStdout := os.Stdout
				r, w, _ := os.Pipe()
				os.Stdout = w

				logger := NewLogger(tc.level, tc.format)
				logger.Log(context.Background(), tc.expectedLevel, "Test log message")

				w.Close()
				os.Stdout = oldStdout

				var buf bytes.Buffer
				buf.ReadFrom(r)
				output := buf.String()

				assert.NotEmpty(t, output, "Expected log output")
				assert.True(t, strings.Contains(output, "Test log message"), "Expected log message in output")

				switch tc.expectedFormat {
				case "json":
					assert.True(t, isJSON(output), "Expected JSON format")
				case "text":
					assert.True(t, !isJSON(output), "Expected Text format")
				}

				// Reset default logger back to nil to avoid interference with other tests
				slog.SetDefault(slog.Default())
			}
		})
	}
}

func isJSON(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}\n")
}
