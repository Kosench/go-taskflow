package repository

import (
	"context"
	"time"

	"github.com/Kosench/go-taskflow/internal/domain"
)

// TaskRepository defines methods for task persistence
type TaskRepository interface {
	// Create creates a new task
	Create(ctx context.Context, task *domain.Task) error

	// Get retrieves a task by ID
	Get(ctx context.Context, id string) (*domain.Task, error)

	// Update updates an existing task
	Update(ctx context.Context, task *domain.Task) error

	UpdateForRetry(ctx context.Context, task *domain.Task) error

	// Delete deletes a task by ID
	Delete(ctx context.Context, id string) error

	// List retrieves tasks with filtering
	List(ctx context.Context, filter TaskFilter) ([]*domain.Task, error)

	// ListPending retrieves pending tasks ordered by priority and creation time
	ListPending(ctx context.Context, limit int) ([]*domain.Task, error)

	// ListForRetry retrieves tasks that need to be retried
	ListForRetry(ctx context.Context, before time.Time, limit int) ([]*domain.Task, error)

	// GetStats retrieves task statistics
	GetStats(ctx context.Context) (*TaskStats, error)

	// BulkCreate creates multiple tasks in a transaction
	BulkCreate(ctx context.Context, tasks []*domain.Task) error

	// CleanupOldTasks removes completed/failed tasks older than specified duration
	CleanupOldTasks(ctx context.Context, before time.Time) (int64, error)

	// LockTaskForProcessing locks a task for processing by a worker
	LockTaskForProcessing(ctx context.Context, taskID, workerID string) (*domain.Task, error)
}

// TaskFilter defines filtering options for task queries
type TaskFilter struct {
	Status      []domain.TaskStatus
	Type        []domain.TaskType
	Priority    []domain.Priority
	WorkerID    string
	TraceID     string
	CreatedFrom time.Time
	CreatedTo   time.Time
	Limit       int
	Offset      int
	OrderBy     string // "created_at", "priority", "updated_at"
	OrderDir    string // "asc", "desc"
}

// TaskStats represents task statistics
type TaskStats struct {
	TotalTasks        int64            `json:"total_tasks"`
	PendingTasks      int64            `json:"pending_tasks"`
	ProcessingTasks   int64            `json:"processing_tasks"`
	CompletedTasks    int64            `json:"completed_tasks"`
	FailedTasks       int64            `json:"failed_tasks"`
	RetryingTasks     int64            `json:"retrying_tasks"`
	CancelledTasks    int64            `json:"cancelled_tasks"`
	TasksByType       map[string]int64 `json:"tasks_by_type"`
	TasksByPriority   map[string]int64 `json:"tasks_by_priority"`
	AvgProcessingTime time.Duration    `json:"avg_processing_time"`
	AvgWaitingTime    time.Duration    `json:"avg_waiting_time"`
}

// TaskHistoryRepository defines methods for task history
type TaskHistoryRepository interface {
	// AddHistory adds a history entry for a task
	AddHistory(ctx context.Context, taskID string, status domain.TaskStatus, message string, createdBy string) error

	// GetHistory retrieves history for a task
	GetHistory(ctx context.Context, taskID string) ([]*TaskHistory, error)
}

// TaskHistory represents a task history entry
type TaskHistory struct {
	ID        int               `json:"id" db:"id"`
	TaskID    string            `json:"task_id" db:"task_id"`
	Status    domain.TaskStatus `json:"status" db:"status"`
	Message   string            `json:"message" db:"message"`
	CreatedAt time.Time         `json:"created_at" db:"created_at"`
	CreatedBy string            `json:"created_by" db:"created_by"`
}

// TaskMetadataRepository defines methods for task metadata
type TaskMetadataRepository interface {
	// SetMetadata sets metadata for a task
	SetMetadata(ctx context.Context, taskID string, metadata map[string]string) error

	// GetMetadata retrieves metadata for a task
	GetMetadata(ctx context.Context, taskID string) (map[string]string, error)

	// DeleteMetadata deletes metadata for a task
	DeleteMetadata(ctx context.Context, taskID string) error
}
