package converter

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/messages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskConverter_ToKafkaMessage(t *testing.T) {
	converter := NewTaskConverter()

	task := &domain.Task{
		ID:         "test-123",
		Type:       domain.TaskTypeImageResize,
		Priority:   domain.PriorityHigh,
		Payload:    []byte(`{"width": 100}`),
		Retries:    1,
		MaxRetries: 3,
		TraceID:    "trace-456",
		CreatedAt:  time.Now(),
	}

	msg, err := converter.ToKafkaMessage(task)
	require.NoError(t, err)

	assert.Equal(t, "test-123", msg.ID)
	assert.Equal(t, "image_resize", msg.Type)
	assert.Equal(t, 2, msg.Priority)
	assert.Equal(t, 1, msg.Retries)
	assert.Equal(t, 3, msg.MaxRetries)
	assert.Equal(t, "trace-456", msg.TraceID)
	assert.Equal(t, json.RawMessage(`{"width": 100}`), msg.Payload)
}

func TestTaskConverter_ToDomainTask(t *testing.T) {
	converter := NewTaskConverter()

	now := time.Now()
	msg := &messages.TaskMessage{
		ID:         "test-123",
		Type:       "send_email",
		Priority:   1,
		Payload:    json.RawMessage(`{"to": "test@example.com"}`),
		Retries:    2,
		MaxRetries: 5,
		TraceID:    "trace-789",
		CreatedAt:  now,
	}

	task, err := converter.ToDomainTask(msg)
	require.NoError(t, err)

	assert.Equal(t, "test-123", task.ID)
	assert.Equal(t, domain.TaskTypeSendEmail, task.Type)
	assert.Equal(t, domain.PriorityNormal, task.Priority)
	assert.Equal(t, []byte(`{"to": "test@example.com"}`), task.Payload)
	assert.Equal(t, 2, task.Retries)
	assert.Equal(t, 5, task.MaxRetries)
	assert.Equal(t, "trace-789", task.TraceID)
	assert.Equal(t, domain.TaskStatusPending, task.Status)
}

func TestTaskConverter_InvalidTaskType(t *testing.T) {
	converter := NewTaskConverter()

	msg := &messages.TaskMessage{
		ID:       "test-123",
		Type:     "invalid_type",
		Priority: 1,
		Payload:  json.RawMessage(`{}`),
	}

	_, err := converter.ToDomainTask(msg)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid task type")
}

func TestTaskConverter_ToResultMessage(t *testing.T) {
	converter := NewTaskConverter()

	task := &domain.Task{
		ID:     "test-123",
		Status: domain.TaskStatusCompleted,
		Result: []byte(`{"success": true}`),
	}

	result := converter.ToResultMessage(task, "worker-456", 5*time.Second)

	assert.Equal(t, "test-123", result.TaskID)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, "worker-456", result.WorkerID)
	assert.Equal(t, 5*time.Second, result.Duration)
	assert.Equal(t, json.RawMessage(`{"success": true}`), result.Result)
}

func TestTaskConverter_ToRetryMessage(t *testing.T) {
	converter := NewTaskConverter()

	task := &domain.Task{
		ID:         "test-123",
		Retries:    2,
		MaxRetries: 5,
	}

	err := fmt.Errorf("processing failed")
	msg := converter.ToRetryMessage(task, err)

	assert.Equal(t, "test-123", msg.TaskID)
	assert.Equal(t, 2, msg.Attempt)
	assert.Equal(t, 5, msg.MaxAttempts)
	assert.Equal(t, "processing failed", msg.Error)
	assert.True(t, msg.NextRetryAt.After(time.Now()))
}

func TestTaskConverter_ToDLQMessage(t *testing.T) {
	converter := NewTaskConverter()

	task := &domain.Task{
		ID:      "test-123",
		Type:    domain.TaskTypeImageResize,
		Payload: []byte(`{"file": "test.jpg"}`),
		Retries: 3,
	}

	err := fmt.Errorf("max retries exceeded")
	msg := converter.ToDLQMessage(task, "tasks-normal", err)

	assert.Equal(t, "test-123", msg.TaskID)
	assert.Equal(t, "image_resize", msg.Type)
	assert.Equal(t, json.RawMessage(`{"file": "test.jpg"}`), msg.Payload)
	assert.Equal(t, "max retries exceeded", msg.Error)
	assert.Equal(t, 3, msg.RetryCount)
	assert.Equal(t, "tasks-normal", msg.OriginalTopic)
}

func TestCalculateBackoff(t *testing.T) {
	tests := []struct {
		retries  int
		expected time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{10, 5 * time.Minute}, // Max backoff
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("retries_%d", tt.retries), func(t *testing.T) {
			backoff := calculateBackoff(tt.retries)
			assert.Equal(t, tt.expected, backoff)
		})
	}
}
