package log

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/bibendi/gruf-relay/internal/config"
	"github.com/lmittmann/tint"
)

var defaultLogger Logger

type LogFormat string

const (
	LogFormatJSON   LogFormat = "json"
	LogFormatText   LogFormat = "text"
	LogFormatPretty LogFormat = "pretty"
)

var formatMap = map[string]LogFormat{
	"json":   LogFormatJSON,
	"text":   LogFormatText,
	"pretty": LogFormatPretty,
}

var levelMap = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

func MustInitLogger(logCfg config.Log) Logger {
	level := logCfg.Level
	logLevel, ok := levelMap[level]
	if !ok {
		panic(fmt.Sprintf("Invalid log level: %s", level))
	}

	format := logCfg.Format
	logFormat, ok := formatMap[format]
	if !ok {
		panic(fmt.Sprintf("Invalid log format: %s", format))
	}
	logger, err := newLogger(os.Stdout, logLevel, logFormat)
	if err != nil {
		panic(err)
	}
	defaultLogger = logger
	slog.SetDefault(logger)
	return logger
}

var mu sync.Mutex

func DefaultLogger() Logger {
	if defaultLogger == nil {
		mu.Lock()
		defer mu.Unlock()
		if defaultLogger != nil {
			return defaultLogger
		}
		MustInitLogger(config.DefaultConfig().Log)
	}
	return defaultLogger
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

func newLogger(w io.Writer, level slog.Level, format LogFormat) (*slog.Logger, error) {
	handlerOpts := &slog.HandlerOptions{
		Level: level,
	}

	if w == nil {
		w = os.Stdout
	}

	var handler slog.Handler
	switch format {
	case LogFormatJSON:
		handler = slog.NewJSONHandler(w, handlerOpts)
	case LogFormatText:
		handler = slog.NewTextHandler(w, handlerOpts)
	case LogFormatPretty:
		handler = tint.NewHandler(w, nil)
	default:
		return nil, fmt.Errorf("invalid log format: %s", format)
	}

	logger := slog.New(handler)
	return logger, nil
}

func With(args ...any) Logger {
	return DefaultLogger().With(args...)
}

func WithGroup(name string) Logger {
	return DefaultLogger().WithGroup(name)
}

func Debug(msg string, args ...any) {
	DefaultLogger().Log(context.Background(), slog.LevelDebug, msg, args...)
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	DefaultLogger().Log(ctx, slog.LevelDebug, msg, args...)
}

// Info calls [Logger.Info] on the default logger.
func Info(msg string, args ...any) {
	DefaultLogger().Log(context.Background(), slog.LevelInfo, msg, args...)
}

// InfoContext calls [Logger.InfoContext] on the default logger.
func InfoContext(ctx context.Context, msg string, args ...any) {
	DefaultLogger().Log(ctx, slog.LevelInfo, msg, args...)
}

// Warn calls [Logger.Warn] on the default logger.
func Warn(msg string, args ...any) {
	DefaultLogger().Log(context.Background(), slog.LevelWarn, msg, args...)
}

// WarnContext calls [Logger.WarnContext] on the default logger.
func WarnContext(ctx context.Context, msg string, args ...any) {
	DefaultLogger().Log(ctx, slog.LevelWarn, msg, args...)
}

// Error calls [Logger.Error] on the default logger.
func Error(msg string, args ...any) {
	DefaultLogger().Log(context.Background(), slog.LevelError, msg, args...)
}

// ErrorContext calls [Logger.ErrorContext] on the default logger.
func ErrorContext(ctx context.Context, msg string, args ...any) {
	DefaultLogger().Log(ctx, slog.LevelError, msg, args...)
}

// Log calls [Logger.Log] on the default logger.
func Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	DefaultLogger().Log(ctx, level, msg, args...)
}

// LogAttrs calls [Logger.LogAttrs] on the default logger.
func LogAttrs(ctx context.Context, level slog.Level, msg string, attrs ...slog.Attr) {
	DefaultLogger().LogAttrs(ctx, level, msg, attrs...)
}
