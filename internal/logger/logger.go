package logger

import (
	"fmt"
	"log/slog"
	"os"
)

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func NewLogger(level, format string) *slog.Logger {
	logLevel, ok := levelMap[level]
	if !ok {
		panic(fmt.Sprintf("Invalid log level: %s", level))
	}

	handlerOpts := &slog.HandlerOptions{
		Level: logLevel,
	}

	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	default:
		panic(fmt.Sprintf("Invalid log format: %s", format))
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	return logger
}
