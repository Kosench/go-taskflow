package kafka

import (
	"encoding/json"
	"time"

	"github.com/IBM/sarama"
)

type TaskMessage struct {
	ID          string          `json:"id"`
	Type        string          `json:"type"`
	Priority    int             `json:"priority"`
	Payload     json.RawMessage `json:"payload"`
	Retries     int             `json:"retries"`
	MaxRetries  int             `json:"max_retries"`
	TraceID     string          `json:"trace_id,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	ScheduledAt *time.Time      `json:"scheduled_at,omitempty"`
}

// TaskResultMessage represents task result message
type TaskResultMessage struct {
	TaskID      string          `json:"task_id"`
	Status      string          `json:"status"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error,omitempty"`
	WorkerID    string          `json:"worker_id"`
	ProcessedAt time.Time       `json:"processed_at"`
	Duration    time.Duration   `json:"duration_ms"`
}

// RetryMessage represents retry message
type RetryMessage struct {
	TaskID      string    `json:"task_id"`
	Attempt     int       `json:"attempt"`
	MaxAttempts int       `json:"max_attempts"`
	Error       string    `json:"error"`
	NextRetryAt time.Time `json:"next_retry_at"`
}

// DLQMessage represents dead letter queue message
type DLQMessage struct {
	TaskID        string          `json:"task_id"`
	Type          string          `json:"type"`
	Payload       json.RawMessage `json:"payload"`
	Error         string          `json:"error"`
	FailedAt      time.Time       `json:"failed_at"`
	RetryCount    int             `json:"retry_count"`
	OriginalTopic string          `json:"original_topic"`
}

// MessageHeaders represents common Kafka headers
type MessageHeaders struct {
	TaskID        string
	TaskType      string
	TraceID       string
	CorrelationID string
	Timestamp     int64
}

// ParseHeaders parses Kafka headers
func ParseHeaders(headers []*sarama.RecordHeader) MessageHeaders {
	h := MessageHeaders{}

	for _, header := range headers {
		switch string(header.Key) {
		case "task_id":
			h.TaskID = string(header.Value)
		case "task_type":
			h.TaskType = string(header.Value)
		case "trace_id":
			h.TraceID = string(header.Value)
		case "correlation_id":
			h.CorrelationID = string(header.Value)
		case "timestamp":
			// Parse timestamp if needed
		}
	}

	return h
}
