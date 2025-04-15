package logger

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/lmittmann/tint"
)

var AppLogger Logger

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func init() {
	AppLogger = newLogger()
	slog.SetDefault(AppLogger.(*slog.Logger))
}

type Logger interface {
	With(args ...any) *slog.Logger
	WithGroup(name string) *slog.Logger
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	Log(ctx context.Context, level slog.Level, msg string, args ...any)
	LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr)
	Handler() slog.Handler
}

func newLogger() Logger {
	level := config.AppConfig.LogLevel
	logLevel, ok := levelMap[level]
	if !ok {
		panic(fmt.Sprintf("Invalid log level: %s", level))
	}

	handlerOpts := &slog.HandlerOptions{
		Level: logLevel,
	}

	format := config.AppConfig.LogFormat
	var handler slog.Handler
	switch format {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	case "pretty":
		handler = tint.NewHandler(os.Stdout, nil)
	default:
		panic(fmt.Sprintf("Invalid log format: %s", format))
	}

	logger := slog.New(handler)

	return logger
}

func Debug(msg string, args ...any) {
	AppLogger.Log(context.Background(), slog.LevelDebug, msg, args...)
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	AppLogger.Log(ctx, slog.LevelDebug, msg, args...)
}

// Info calls [Logger.Info] on the default logger.
func Info(msg string, args ...any) {
	AppLogger.Log(context.Background(), slog.LevelInfo, msg, args...)
}

// InfoContext calls [Logger.InfoContext] on the default logger.
func InfoContext(ctx context.Context, msg string, args ...any) {
	AppLogger.Log(ctx, slog.LevelInfo, msg, args...)
}

// Warn calls [Logger.Warn] on the default logger.
func Warn(msg string, args ...any) {
	AppLogger.Log(context.Background(), slog.LevelWarn, msg, args...)
}

// WarnContext calls [Logger.WarnContext] on the default logger.
func WarnContext(ctx context.Context, msg string, args ...any) {
	AppLogger.Log(ctx, slog.LevelWarn, msg, args...)
}

// Error calls [Logger.Error] on the default logger.
func Error(msg string, args ...any) {
	AppLogger.Log(context.Background(), slog.LevelError, msg, args...)
}

// ErrorContext calls [Logger.ErrorContext] on the default logger.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	AppLogger.Log(ctx, slog.LevelError, msg, args...)
}

// Log calls [Logger.Log] on the default logger.
func Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	AppLogger.Log(ctx, level, msg, args...)
}

// LogAttrs calls [Logger.LogAttrs] on the default logger.
func LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	AppLogger.LogAttrs(ctx, level, msg, attrs...)
}
