package producer

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProducer_getTopicByPriority(t *testing.T) {
	cfg := &config.KafkaConfig{
		Topics: config.TopicsConfig{
			TasksHigh:   "tasks-high",
			TasksNormal: "tasks-normal",
			TasksLow:    "tasks-low",
		},
	}

	p := &Producer{config: cfg}

	tests := []struct {
		priority domain.Priority
		expected string
	}{
		{domain.PriorityCritical, "tasks-high"},
		{domain.PriorityHigh, "tasks-high"},
		{domain.PriorityNormal, "tasks-normal"},
		{domain.PriorityLow, "tasks-low"},
	}

	for _, tt := range tests {
		t.Run(tt.priority.String(), func(t *testing.T) {
			topic := p.getTopicByPriority(tt.priority)
			assert.Equal(t, tt.expected, topic)
		})
	}
}

func TestProducer_prepareTaskMessage(t *testing.T) {
	cfg := &config.KafkaConfig{
		Topics: config.TopicsConfig{
			TasksNormal: "tasks-normal",
		},
	}

	log := logger.New(logger.Config{Level: "debug"})
	p := &Producer{config: cfg, logger: log}

	task := &domain.Task{
		ID:        "test-123",
		Type:      domain.TaskTypeImageResize,
		Priority:  domain.PriorityNormal,
		Payload:   []byte(`{"width": 100}`),
		TraceID:   "trace-456",
		CreatedAt: time.Now(),
	}

	msg, err := p.prepareTaskMessage(task, "tasks-normal")
	require.NoError(t, err)

	assert.Equal(t, "tasks-normal", msg.Topic)
	assert.Equal(t, "test-123", string(msg.Key.(sarama.StringEncoder)))

	// Check headers
	foundTaskID := false
	foundTaskType := false
	for _, header := range msg.Headers {
		if string(header.Key) == "task_id" {
			foundTaskID = true
			assert.Equal(t, "test-123", string(header.Value))
		}
		if string(header.Key) == "task_type" {
			foundTaskType = true
			assert.Equal(t, string(domain.TaskTypeImageResize), string(header.Value))
		}
	}

	assert.True(t, foundTaskID, "task_id header not found")
	assert.True(t, foundTaskType, "task_type header not found")
}

func TestTaskMessage_Marshal(t *testing.T) {
	now := time.Now()
	msg := messages.TaskMessage{
		ID:         "test-123",
		Type:       "image_resize",
		Priority:   1,
		Payload:    []byte(`{"width": 100}`),
		Retries:    0,
		MaxRetries: 3,
		TraceID:    "trace-456",
		CreatedAt:  now,
	}

	data, err := json.Marshal(msg)
	require.NoError(t, err)

	var decoded messages.TaskMessage
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, msg.ID, decoded.ID)
	assert.Equal(t, msg.Type, decoded.Type)
	assert.Equal(t, msg.Priority, decoded.Priority)
}

// Integration test (requires Kafka)
func TestProducer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	cfg := &config.KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topics: config.TopicsConfig{
			TasksHigh:   "tasks-high",
			TasksNormal: "tasks-normal",
			TasksLow:    "tasks-low",
		},
		Producer: config.ProducerConfig{
			RequiredAcks:    1,
			Compression:     "snappy",
			MaxMessageBytes: 1000000,
			FlushFrequency:  100 * time.Millisecond,
			Idempotent:      false,
			RetryMax:        3,
			RetryBackoff:    100 * time.Millisecond,
		},
	}

	log := logger.New(logger.Config{Level: "debug"})

	producer, err := NewProducer(cfg, log)
	require.NoError(t, err)
	defer producer.Close()

	// Send test task
	task := &domain.Task{
		ID:         "integration-test-123",
		Type:       domain.TaskTypeImageResize,
		Priority:   domain.PriorityNormal,
		Payload:    []byte(`{"test": true}`),
		CreatedAt:  time.Now(),
		MaxRetries: 3,
	}

	ctx := context.Background()
	err = producer.SendTask(ctx, task)
	assert.NoError(t, err)

	// Give some time for async send
	time.Sleep(500 * time.Millisecond)

	stats := producer.GetStats()
	assert.Greater(t, stats.MessagesSent, int64(0))
}
