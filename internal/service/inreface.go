package service

import (
	"context"
	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/repository"
	"time"
)

// TaskService defines the interface for task operations
type TaskService interface {
	//CreateTask creates a new task amd publishes it to Kafka
	CreateTask(ctx context.Context, req CreateTaskRequest) (*domain.Task, error)
	GetTask(ctx context.Context, taskID string) (*domain.Task, error)
	ListTask(ctx context.Context, filter ListTasksRequest) ([]*domain.Task, error)
	UpdateTaskStatus(ctx context.Context, taskID string, status domain.TaskStatus, result []byte, errorMsg string) error
	ProcessTask(ctx context.Context, taskID string, workerID string) error
	RetryTask(ctx context.Context, taskID string) error
	CancelTask(ctx context.Context, taskID string, reason string) error
	GetTaskStats(ctx context.Context) (*repository.TaskStats, error)
	CleanupOldTasks(ctx context.Context, olderThan time.Duration) (int64, error)
}

type CreateTaskRequest struct {
	Type        domain.TaskType   `json:"type" validate:"required"`
	Priority    domain.Priority   `json:"priority"`
	Payload     []byte            `json:"payload" validate:"required"`
	MaxRetries  int               `json:"max_retries"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	TraceID     string            `json:"trace_id,omitempty"`
}

type ListTasksRequest struct {
	Status      []domain.TaskStatus `json:"status,omitempty"`
	Type        []domain.TaskType   `json:"type,omitempty"`
	Priority    []domain.Priority   `json:"priority,omitempty"`
	WorkerID    string              `json:"worker_id,omitempty"`
	TraceID     string              `json:"trace_id,omitempty"`
	CreatedFrom time.Time           `json:"created_from,omitempty"`
	CreatedTo   time.Time           `json:"created_to,omitempty"`
	Limit       int                 `json:"limit"`
	Offset      int                 `json:"offset"`
	OrderBy     string              `json:"order_by"`
	OrderDir    string              `json:"order_dir"`
}

func (r *CreateTaskRequest) Validate() error {
	if !r.Type.IsValid() {
		return domain.ErrInvalidTaskType
	}

	if len(r.Payload) == 0 {
		return domain.ErrEmptyPayload
	}

	if r.MaxRetries < 0 {
		return domain.NewValidationError("max_retries", "cannot be negative")
	}

	if r.ScheduledAt != nil && r.ScheduledAt.Before(time.Now()) {
		return domain.NewValidationError("scheduled_at", "cannot be in the past")
	}

	return nil
}
