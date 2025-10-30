package kafka

import (
	"encoding/json"
	"fmt"
	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/google/uuid"
	"time"
)

type TaskConverter struct{}

func NewTaskConverter() *TaskConverter {
	return &TaskConverter{}
}

func (c *TaskConverter) ToKafkaMessage(task *domain.Task) (*TaskMessage, error) {
	if task == nil {
		return nil, fmt.Errorf("task is nil")
	}

	msg := &TaskMessage{
		ID:          task.ID,
		Type:        string(task.Type),
		Priority:    int(task.Priority),
		Payload:     json.RawMessage(task.Payload),
		Retries:     task.Retries,
		MaxRetries:  task.MaxRetries,
		TraceID:     task.TraceID,
		CreatedAt:   task.CreatedAt,
		ScheduledAt: task.ScheduledAt,
	}

	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}

	// Set trace ID if not present
	if msg.TraceID == "" {
		msg.TraceID = uuid.New().String()
	}

	return msg, nil
}

func (c *TaskConverter) ToDomainTask(msg *TaskMessage) (*domain.Task, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	task := &domain.Task{
		ID:          msg.ID,
		Type:        domain.TaskType(msg.Type),
		Status:      domain.TaskStatusPending,
		Priority:    domain.Priority(msg.Priority),
		Payload:     []byte(msg.Payload),
		Retries:     msg.Retries,
		MaxRetries:  msg.MaxRetries,
		TraceID:     msg.TraceID,
		CreatedAt:   msg.CreatedAt,
		UpdatedAt:   time.Now(),
		ScheduledAt: msg.ScheduledAt,
		Metadata:    make(map[string]string),
	}

	// Validate task type
	if !task.Type.IsValid() {
		return nil, fmt.Errorf("invalid task type: %s", msg.Type)
	}

	// Validate priority
	if !task.Priority.IsValid() {
		return nil, fmt.Errorf("invalid priority: %d", msg.Priority)
	}

	return task, nil
}

func (c *TaskConverter) ToResultMessage(task *domain.Task, workerID string, duration time.Duration) *TaskResultMessage {
	msg := &TaskResultMessage{
		TaskID:      task.ID,
		Status:      string(task.Status),
		WorkerID:    workerID,
		ProcessedAt: time.Now(),
		Duration:    duration,
	}

	if len(task.Result) > 0 {
		msg.Result = json.RawMessage(task.Result)
	}

	if task.Error != "" {
		msg.Error = task.Error
	}

	return msg
}

// ToRetryMessage creates retry message
func (c *TaskConverter) ToRetryMessage(task *domain.Task, err error) *RetryMessage {
	nextRetryAt := time.Now().Add(calculateBackoff(task.Retries))

	return &RetryMessage{
		TaskID:      task.ID,
		Attempt:     task.Retries,
		MaxAttempts: task.MaxRetries,
		Error:       err.Error(),
		NextRetryAt: nextRetryAt,
	}
}

// ToDLQMessage creates DLQ message
func (c *TaskConverter) ToDLQMessage(task *domain.Task, originalTopic string, err error) *DLQMessage {
	return &DLQMessage{
		TaskID:        task.ID,
		Type:          string(task.Type),
		Payload:       json.RawMessage(task.Payload),
		Error:         err.Error(),
		FailedAt:      time.Now(),
		RetryCount:    task.Retries,
		OriginalTopic: originalTopic,
	}
}

// calculateBackoff calculates exponential backoff
func calculateBackoff(retries int) time.Duration {
	if retries <= 0 {
		return time.Second
	}

	backoff := time.Duration(1<<uint(retries)) * time.Second

	// Max backoff of 5 minutes
	maxBackoff := 5 * time.Minute
	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	return backoff
}
