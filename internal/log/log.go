package log

import (
	"log/slog"
	"os"
	"sync"
)

var (
	logger *slog.Logger
	once   sync.Once
)

func initLogger(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	logger = slog.New(handler)
}

// Init initializes the logger with the specified verbosity level.
// If verbose is true, sets log level to Debug, otherwise to Info.
// Safe to call concurrently; only the first call takes effect.
func Init(verbose bool) {
	once.Do(func() {
		initLogger(verbose)
	})
}

func ensureInit() {
	once.Do(func() {
		initLogger(false)
	})
}

// Info logs an info level message.
func Info(msg string, args ...any) {
	ensureInit()
	logger.Info(msg, args...)
}

// Debug logs a debug level message.
func Debug(msg string, args ...any) {
	ensureInit()
	logger.Debug(msg, args...)
}

// Error logs an error level message.
func Error(msg string, args ...any) {
	ensureInit()
	logger.Error(msg, args...)
}

// Warn logs a warn level message.
func Warn(msg string, args ...any) {
	ensureInit()
	logger.Warn(msg, args...)
}
