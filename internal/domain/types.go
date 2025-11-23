package domain

import (
	"database/sql/driver"
	"fmt"
)

type TaskType string

const (
	// TaskTypeImageResize represents image resize task
	TaskTypeImageResize TaskType = "image_resize"

	// TaskTypeImageConvert represents image conversion task
	TaskTypeImageConvert TaskType = "image_convert"

	// TaskTypeSendEmail represents email sending task
	TaskTypeSendEmail TaskType = "send_email"

	// TaskTypeGenerateReport represents report generation task
	TaskTypeGenerateReport TaskType = "generate_report"

	// TaskTypeDataExport represents data export task
	TaskTypeDataExport TaskType = "data_export"

	// TaskTypeWebhook represents webhook task
	TaskTypeWebhook TaskType = "webhook"
)

// IsValid check if task type is valid
func (t TaskType) IsValid() bool {
	switch t {
	case TaskTypeImageResize, TaskTypeImageConvert,
		TaskTypeSendEmail, TaskTypeGenerateReport,
		TaskTypeDataExport, TaskTypeWebhook:
		return true
	default:
		return false
	}
}

func (t TaskType) String() string {
	return string(t)
}

func (t *TaskType) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		*t = TaskType(v)
	case []byte:
		*t = TaskType(v)
	default:
		return fmt.Errorf("cannot scan %T into TaskStatus", value)
	}

	if !t.IsValid() {
		return ErrInvalidTaskStatus
	}

	return nil
}

func (t TaskType) Value() (driver.Value, error) {
	if !t.IsValid() {
		return nil, ErrInvalidTaskType
	}
	return string(t), nil
}

// TaskStatus represents status of task
type TaskStatus string

const (
	// TaskStatusPending - task is waiting to be processed
	TaskStatusPending TaskStatus = "pending"

	// TaskStatusProcessing - task is being processed
	TaskStatusProcessing TaskStatus = "processing"

	// TaskStatusCompleted - task completed successfully
	TaskStatusCompleted TaskStatus = "completed"

	// TaskStatusFailed - task failed
	TaskStatusFailed TaskStatus = "failed"

	// TaskStatusRetrying - task is scheduled for retry
	TaskStatusRetrying TaskStatus = "retrying"

	// TaskStatusCancelled - task was cancelled
	TaskStatusCancelled TaskStatus = "cancelled"
)

func (s TaskStatus) IsValid() bool {
	switch s {
	case TaskStatusPending, TaskStatusProcessing, TaskStatusCompleted,
		TaskStatusFailed, TaskStatusRetrying, TaskStatusCancelled:
		return true
	default:
		return false
	}
}

func (s TaskStatus) String() string {
	return string(s)
}

// Scan implements sql.Scanner interface
func (s *TaskStatus) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		*s = TaskStatus(v)
	case []byte:
		*s = TaskStatus(v)
	default:
		return fmt.Errorf("cannot scan %T into TaskStatus", value)
	}

	if !s.IsValid() {
		return ErrInvalidTaskStatus
	}

	return nil
}

// Value implements driver.Valuer interface
func (s TaskStatus) Value() (driver.Value, error) {
	if !s.IsValid() {
		return nil, ErrInvalidTaskStatus
	}
	return string(s), nil
}

type Priority int

const (
	// PriorityLow represents low priority
	PriorityLow Priority = 0

	// PriorityNormal represents normal priority
	PriorityNormal Priority = 1

	// PriorityHigh represents high priority
	PriorityHigh Priority = 2

	// PriorityCritical represents critical priority
	PriorityCritical Priority = 3
)

func (p Priority) IsValid() bool {
	return p >= PriorityLow && p <= PriorityCritical
}

func (p Priority) String() string {
	switch p {
	case PriorityLow:
		return "low"
	case PriorityNormal:
		return "normal"
	case PriorityHigh:
		return "high"
	case PriorityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Scan implements sql.Scanner interface
func (p *Priority) Scan(value interface{}) error {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case int64:
		*p = Priority(v)
	case int:
		*p = Priority(v)
	default:
		return fmt.Errorf("cannot scan %T into Priority", value)
	}

	if !p.IsValid() {
		return ErrInvalidPriority
	}

	return nil
}

// Value implements driver.Valuer interface
func (p Priority) Value() (driver.Value, error) {
	if !p.IsValid() {
		return nil, ErrInvalidPriority
	}
	return int64(p), nil
}

// ParsePriority parses string to Priority
func ParsePriority(s string) (Priority, error) {
	switch s {
	case "low":
		return PriorityLow, nil
	case "normal":
		return PriorityNormal, nil
	case "high":
		return PriorityHigh, nil
	case "critical":
		return PriorityCritical, nil
	default:
		return PriorityNormal, ErrInvalidPriority
	}
}

// GetTopicByPriority returns Kafka topic name based on priority
func GetTopicByPriority(p Priority) string {
	switch p {
	case PriorityCritical, PriorityHigh:
		return "tasks-high"
	case PriorityNormal:
		return "tasks-normal"
	case PriorityLow:
		return "tasks-low"
	default:
		return "tasks-normal"
	}
}
