package service

import (
	"context"
	"fmt"
	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/repository"
	"strings"
	"time"
)

// TaskService defines the interface for task operations
type TaskService interface {
	//CreateTask creates a new task amd publishes it to Kafka
	CreateTask(ctx context.Context, req CreateTaskRequest) (*domain.Task, error)
	GetTask(ctx context.Context, taskID string) (*domain.Task, error)
	ListTask(ctx context.Context, filter ListTasksRequest) ([]*domain.Task, error)
	UpdateTaskStatus(ctx context.Context, taskID string, status domain.TaskStatus, result []byte, errorMsg string) error
	ProcessTask(ctx context.Context, taskID string, workerID string) (*domain.Task, error)
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
	ScheduledAt *time.Time        `json:"scheduled_at,omitzero"`
	Metadata    map[string]string `json:"metadata,omitzero"`
	TraceID     string            `json:"trace_id,omitempty"`
}

type ListTasksRequest struct {
	Status      []domain.TaskStatus `json:"status,omitzero"`
	Type        []domain.TaskType   `json:"type,omitzero"`
	Priority    []domain.Priority   `json:"priority,omitzero"`
	WorkerID    string              `json:"worker_id,omitempty"`
	TraceID     string              `json:"trace_id,omitempty"`
	CreatedFrom time.Time           `json:"created_from,omitzero"`
	CreatedTo   time.Time           `json:"created_to,omitzero"`
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

func (r *ListTasksRequest) Validate() error {
	if r.Limit < 0 {
		return domain.NewValidationError("limit", "cannot be negative")
	}
	if r.Offset < 0 {
		return domain.NewValidationError("offset", "cannot be negative")
	}
	for _, status := range r.Status {
		if !status.IsValid() {
			return domain.NewValidationError("status", fmt.Sprintf("invalid value %q", status))
		}
	}
	for _, taskType := range r.Type {
		if !taskType.IsValid() {
			return domain.NewValidationError("type", fmt.Sprintf("invalid value %q", taskType))
		}
	}
	for _, priority := range r.Priority {
		if !priority.IsValid() {
			return domain.NewValidationError("priority", fmt.Sprintf("invalid value %d", priority))
		}
	}

	allowedOrderBy := map[string]bool{"": true, "created_at": true, "updated_at": true, "priority": true}
	if !allowedOrderBy[r.OrderBy] {
		return domain.NewValidationError("order_by", "must be created_at, updated_at, or priority")
	}
	orderDir := strings.ToLower(r.OrderDir)
	if orderDir != "" && orderDir != "asc" && orderDir != "desc" {
		return domain.NewValidationError("order_dir", "must be asc or desc")
	}
	if !r.CreatedFrom.IsZero() && !r.CreatedTo.IsZero() && r.CreatedFrom.After(r.CreatedTo) {
		return domain.NewValidationError("created_from", "cannot be after created_to")
	}
	return nil
}
