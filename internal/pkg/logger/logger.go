package logger

import (
	"context"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Logger struct {
	*zerolog.Logger
}

type Config struct {
	Level      string
	Format     string // json, console
	Output     string // stdout, stderr, file
	FilePath   string
	AddCaller  bool
	StackTrace bool
}

var (
	// Global logger instance
	globalLogger *Logger
)

// New creates a new logger instance
func New(cfg Config) *Logger {
	var output io.Writer
	// Setup output
	switch strings.ToLower(cfg.Output) {
	case "stderr":
		output = os.Stderr
	case "file":
		if cfg.FilePath == "" {
			cfg.FilePath = "app.log"
		}
		// Create log directory if not exists
		dir := filepath.Dir(cfg.FilePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			panic(fmt.Errorf("failed to create log directory: %w", err))
		}

		file, err := os.OpenFile(cfg.FilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(fmt.Errorf("failed to open log file: %w", err))
		}
		output = file
	default:
		output = os.Stdout
	}

	// Setup format
	if strings.ToLower(cfg.Format) == "console" {
		output = zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	// Configure zerolog
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// Create logger
	zl := zerolog.New(output).With().Timestamp().Logger()

	// Set log level
	level := parseLevel(cfg.Level)
	zl = zl.Level(level)

	// Add caller information
	if cfg.AddCaller {
		zl = zl.With().Caller().Logger()
	}

	logger := &Logger{
		Logger: &zl,
	}

	// Set global logger
	globalLogger = logger

	return logger
}

// parseLevel converts string level to zerolog level
func parseLevel(level string) zerolog.Level {
	switch strings.ToLower(level) {
	case "trace":
		return zerolog.TraceLevel
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn", "warning":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	case "disabled":
		return zerolog.Disabled
	default:
		return zerolog.InfoLevel
	}
}

// WithContext returns a logger with context
func (l *Logger) WithContext(ctx context.Context) context.Context {
	return l.Logger.WithContext(ctx)
}

// FromContext gets logger from context
func FromContext(ctx context.Context) *Logger {
	zl := zerolog.Ctx(ctx)
	if zl.GetLevel() == zerolog.Disabled {
		return globalLogger
	}
	return &Logger{Logger: zl}
}

// WithField adds a field to logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	zl := l.With().Interface(key, value).Logger()
	return &Logger{Logger: &zl}
}

// WithFields adds multiple fields to logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zl := l.With().Fields(fields).Logger()
	return &Logger{Logger: &zl}
}

// WithError adds error to logger
func (l *Logger) WithError(err error) *Logger {
	zl := l.With().Err(err).Logger()
	return &Logger{Logger: &zl}
}

// Global logger functions

// Get returns global logger
func Get() *Logger {
	if globalLogger == nil {
		// Create default logger if not initialized
		globalLogger = New(Config{
			Level:  "info",
			Format: "json",
			Output: "stdout",
		})
	}
	return globalLogger
}

// Debug logs debug level message
func Debug(msg string) {
	Get().Debug().Msg(msg)
}

// Info logs info level message
func Info(msg string) {
	Get().Info().Msg(msg)
}

// Warn logs warn level message
func Warn(msg string) {
	Get().Warn().Msg(msg)
}

// Error logs error level message
func Error(msg string) {
	Get().Error().Msg(msg)
}

// Fatal logs fatal level message and exits
func Fatal(msg string) {
	Get().Fatal().Msg(msg)
}

// Debugf logs formatted debug level message
func Debugf(format string, v ...interface{}) {
	Get().Debug().Msgf(format, v...)
}

// Infof logs formatted info level message
func Infof(format string, v ...interface{}) {
	Get().Info().Msgf(format, v...)
}

// Warnf logs formatted warn level message
func Warnf(format string, v ...interface{}) {
	Get().Warn().Msgf(format, v...)
}

// Errorf logs formatted error level message
func Errorf(format string, v ...interface{}) {
	Get().Error().Msgf(format, v...)
}

// Fatalf logs formatted fatal level message and exits
func Fatalf(format string, v ...interface{}) {
	Get().Fatal().Msgf(format, v...)
}
