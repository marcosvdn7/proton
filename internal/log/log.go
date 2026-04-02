package log

import (
	"log/slog"
	"os"
)

var logger *slog.Logger

// Init initializes the logger with the specified verbosity level.
// If verbose is true, sets log level to Debug, otherwise to Info.
func Init(verbose bool) {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})

	logger = slog.New(handler)
}

// Info logs an info level message
func Info(msg string, args ...any) {
	if logger == nil {
		Init(false) // Initialize with default settings if not already initialized
	}
	logger.Info(msg, args...)
}

// Debug logs a debug level message
func Debug(msg string, args ...any) {
	if logger == nil {
		Init(false) // Initialize with default settings if not already initialized
	}
	logger.Debug(msg, args...)
}

// Error logs an error level message
func Error(msg string, args ...any) {
	if logger == nil {
		Init(false) // Initialize with default settings if not already initialized
	}
	logger.Error(msg, args...)
}

// Warn logs a warn level message
func Warn(msg string, args ...any) {
	if logger == nil {
		Init(false) // Initialize with default settings if not already initialized
	}
	logger.Warn(msg, args...)
}
