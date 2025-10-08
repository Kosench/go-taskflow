package logger

import (
	"github.com/rs/zerolog"
	"time"
)

// Fields type for structured logging
type Fields map[string]interface{}

// LogLevel represents log level
type LogLevel string

const (
	// LevelTrace trace level
	LevelTrace LogLevel = "trace"
	// LevelDebug debug level
	LevelDebug LogLevel = "debug"
	// LevelInfo info level
	LevelInfo LogLevel = "info"
	// LevelWarn warn level
	LevelWarn LogLevel = "warn"
	// LevelError error level
	LevelError LogLevel = "error"
	// LevelFatal fatal level
	LevelFatal LogLevel = "fatal"
)

// Duration logs duration since start
func Duration(event *zerolog.Event, key string, start time.Time) *zerolog.Event {
	return event.Dur(key, time.Since(start))
}

// TrackTime tracks execution time of a function
func TrackTime(start time.Time, name string) {
	elapsed := time.Since(start)
	Get().Debug().
		Str("function", name).
		Dur("duration", elapsed).
		Msg("function execution time")
}

// Measure returns a function to measure execution time
// Usage: defer logger.Measure(time.Now(), "functionName")()
func Measure(start time.Time, name string) func() {
	return func() {
		Get().Debug().
			Str("function", name).
			Dur("duration", time.Since(start)).
			Msg("function completed")
	}
}

// LogRequest logs HTTP request details
func LogRequest(method, path string, statusCode int, duration time.Duration) {
	event := Get().Info()

	if statusCode >= 400 && statusCode < 500 {
		event = Get().Warn()
	} else if statusCode >= 500 {
		event = Get().Error()
	}

	event.
		Str("method", method).
		Str("path", path).
		Int("status_code", statusCode).
		Dur("duration", duration).
		Msg("http request")
}

// LogKafkaMessage logs Kafka message details
func LogKafkaMessage(topic string, partition int32, offset int64, key string, action string) {
	Get().Info().
		Str("topic", topic).
		Int32("partition", partition).
		Int64("offset", offset).
		Str("key", key).
		Str("action", action).
		Msg("kafka message")
}

// LogDatabaseQuery logs database query details
func LogDatabaseQuery(query string, duration time.Duration, rowsAffected int64) {
	event := Get().Debug()

	if duration > 1*time.Second {
		event = Get().Warn()
	}

	event.
		Str("query", query).
		Dur("duration", duration).
		Int64("rows_affected", rowsAffected).
		Msg("database query")
}

// LogTaskProcessing logs task processing details
func LogTaskProcessing(taskID string, taskType string, status string, duration time.Duration, err error) {
	event := Get().Info()

	if err != nil {
		event = Get().Error().Err(err)
	}

	event.
		Str("task_id", taskID).
		Str("task_type", taskType).
		Str("status", status).
		Dur("duration", duration).
		Msg("task processing")
}
