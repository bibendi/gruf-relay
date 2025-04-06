package logger

import (
	"log"
	"log/slog"
	"os"
)

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func InitLogger(level, format string) {
	logLevel, ok := levelMap[level]
	if !ok {
		log.Fatalf("Invalid log level: %s", level)
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
		log.Fatalf("Invalid log format: %s", format)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	logger.Info("Initializing logger", slog.String("level", level), slog.String("format", format))
}
