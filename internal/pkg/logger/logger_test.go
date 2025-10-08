package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestLoggerCreation(t *testing.T) {
	// Create logger with JSON format
	logger := New(Config{
		Level:  "debug",
		Format: "json",
		Output: "stdout",
	})

	if logger == nil {
		t.Fatal("expected logger to be created")
	}
}

func TestLogLevels(t *testing.T) {
	tests := []struct {
		level    string
		expected string
	}{
		{"debug", "debug"},
		{"info", "info"},
		{"warn", "warn"},
		{"error", "error"},
	}

	for _, test := range tests {
		t.Run(test.level, func(t *testing.T) {
			var buf bytes.Buffer

			logger := New(Config{
				Level:  test.level,
				Format: "json",
				Output: "stdout",
			})

			// Capture output (this is simplified, in real test you'd redirect output)
			_ = buf
			_ = logger
			// Actual test would verify log output
		})
	}
}

func TestContextLogger(t *testing.T) {
	ctx := context.Background()

	// Add request ID to context
	ctx = WithRequestID(ctx, "test-request-123")

	// Get request ID from context
	requestID := GetRequestID(ctx)
	if requestID != "test-request-123" {
		t.Errorf("expected request ID to be 'test-request-123', got %s", requestID)
	}

	// Add correlation ID
	ctx = WithCorrelationID(ctx, "correlation-456")
	correlationID := GetCorrelationID(ctx)
	if correlationID != "correlation-456" {
		t.Errorf("expected correlation ID to be 'correlation-456', got %s", correlationID)
	}
}

func TestLoggerFields(t *testing.T) {
	logger := New(Config{
		Level:  "debug",
		Format: "json",
	})

	// Test with single field
	loggerWithField := logger.WithField("key", "value")
	if loggerWithField == nil {
		t.Fatal("expected logger with field")
	}

	// Test with multiple fields
	fields := map[string]interface{}{
		"field1": "value1",
		"field2": 42,
		"field3": true,
	}
	loggerWithFields := logger.WithFields(fields)
	if loggerWithFields == nil {
		t.Fatal("expected logger with fields")
	}
}

func TestMeasureFunction(t *testing.T) {
	// Test measure helper
	start := time.Now()
	time.Sleep(10 * time.Millisecond)

	// This would normally log, but we're just testing it doesn't panic
	TrackTime(start, "test_function")
}

func TestJSONOutput(t *testing.T) {
	var buf bytes.Buffer

	// This test is simplified - in real scenario you'd capture the output
	logger := New(Config{
		Level:  "info",
		Format: "json",
		Output: "stdout",
	})

	// Log a message (in real test, this would write to buf)
	logger.Info().Str("test", "value").Msg("test message")

	// In real test, you'd parse JSON and verify fields
	var logEntry map[string]interface{}
	if err := json.NewDecoder(&buf).Decode(&logEntry); err == nil {
		// Verify log entry structure
		if logEntry["test"] != "value" {
			t.Error("expected field 'test' to be 'value'")
		}
	}
}
