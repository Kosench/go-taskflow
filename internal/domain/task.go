package domain

import (
	"errors"
	"time"
)

var (
	// ErrTaskNotFound is returned when task is not found
	ErrTaskNotFound = errors.New("task not found")

	// ErrInvalidTaskStatus is returned when task status transition is invalid
	ErrInvalidTaskStatus = errors.New("invalid task status transition")

	// ErrInvalidTaskType is returned when task type is unknown
	ErrInvalidTaskType = errors.New("invalid task type")

	// ErrTaskAlreadyExists is returned when task with same ID already exists
	ErrTaskAlreadyExists = errors.New("task already exists")

	// ErrInvalidPriority is returned when priority is invalid
	ErrInvalidPriority = errors.New("invalid priority value")

	// ErrEmptyPayload is returned when task payload is empty
	ErrEmptyPayload = errors.New("task payload cannot be empty")

	// ErrMaxRetriesExceeded is returned when max retries exceeded
	ErrMaxRetriesExceeded = errors.New("max retries exceeded")
)

type Task struct {
	ID          string     `json:"id" db:"id"`
	Type        TaskType   `json:"type" db:"type"`
	Status      TaskStatus `json:"status" db:"status"`
	Priority    Priority   `json:"priority" db:"priority"`
	Payload     []byte     `json:"payload" db:"payload"`
	Result      []byte     `json:"result,omitempty" db:"result"`
	Error       string     `json:"error,omitempty" db:"error"`
	Retries     int        `json:"retries" db:"retries"`
	MaxRetries  int        `json:"max_retries" db:"max_retries"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty" db:"scheduled_at"`
	StartedAt   *time.Time `json:"started_at,omitempty" db:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`

	// Additional metadata
	Metadata map[string]string `json:"metadata,omitempty" db:"-"`
	WorkerID string            `json:"worker_id,omitempty" db:"worker_id"`
	TraceID  string            `json:"trace_id,omitempty" db:"trace_id"`
}

func NewTask(taskType TaskType, priority Priority, payload []byte) (*Task, error) {
	if !taskType.IsValid() {
		return nil, ErrInvalidTaskType
	}

	if !priority.IsValid() {
		return nil, ErrInvalidPriority
	}

	if len(payload) == 0 {
		return nil, ErrEmptyPayload
	}

	now := time.Now().UTC()

	return &Task{
		Type:       taskType,
		Status:     TaskStatusPending,
		Priority:   priority,
		Payload:    payload,
		Retries:    0,
		MaxRetries: 3, // default max retries
		CreatedAt:  now,
		UpdatedAt:  now,
		Metadata:   make(map[string]string),
	}, nil
}
func (t *Task) CanRetry() bool {
	return t.Status == TaskStatusFailed && t.Retries < t.MaxRetries
}

func (t *Task) IncrementRetries() {
	t.Retries++
	t.UpdatedAt = time.Now().UTC()
}

func (t *Task) MarkAsProcessing(workerID string) error {
	if t.Status != TaskStatusPending && t.Status != TaskStatusRetrying {
		return ErrInvalidTaskStatus
	}

	now := time.Now().UTC()
	t.Status = TaskStatusProcessing
	t.WorkerID = workerID
	t.StartedAt = &now
	t.UpdatedAt = now

	return nil
}

func (t *Task) MarkAsCompleted(result []byte) {
	now := time.Now().UTC()
	t.Status = TaskStatusCompleted
	t.Result = result
	t.CompletedAt = &now
	t.UpdatedAt = now
	t.Error = ""
}

func (t *Task) MarkAsFailed(err error) {
	t.Status = TaskStatusFailed
	t.Error = err.Error()
	t.UpdatedAt = time.Now().UTC()

	// If can retry, mark as retrying
	if t.CanRetry() {
		t.Status = TaskStatusRetrying
		t.IncrementRetries()

		// Calculate next retry time with exponential backoff
		backoffSeconds := 1 << t.Retries // 2^retries seconds
		nextRetry := time.Now().UTC().Add(time.Duration(backoffSeconds) * time.Second)
		t.ScheduledAt = &nextRetry
	}
}

func (t *Task) MarkAsCancelled(reason string) {
	t.Status = TaskStatusCancelled
	t.Error = reason
	t.UpdatedAt = time.Now().UTC()
}

func (t *Task) IsTerminal() bool {
	return t.Status == TaskStatusCompleted ||
		t.Status == TaskStatusCancelled ||
		(t.Status == TaskStatusFailed && !t.CanRetry())
}

// GetProcessingTime returns task processing time
func (t *Task) GetProcessingTime() time.Duration {
	if t.StartedAt == nil {
		return 0
	}

	if t.CompletedAt != nil {
		return t.CompletedAt.Sub(*t.StartedAt)
	}

	return time.Since(*t.StartedAt)
}

// GetWaitingTime returns time task waited before processing
func (t *Task) GetWaitingTime() time.Duration {
	if t.StartedAt == nil {
		return time.Since(t.CreatedAt)
	}
	return t.StartedAt.Sub(t.CreatedAt)
}

// Validate validates task fields
func (t *Task) Validate() error {
	if t.ID == "" {
		return errors.New("task ID is required")
	}

	if !t.Type.IsValid() {
		return ErrInvalidTaskType
	}

	if !t.Status.IsValid() {
		return ErrInvalidTaskStatus
	}

	if !t.Priority.IsValid() {
		return ErrInvalidPriority
	}

	if len(t.Payload) == 0 {
		return ErrEmptyPayload
	}

	if t.MaxRetries < 0 {
		return errors.New("max retries cannot be negative")
	}

	if t.Retries < 0 {
		return errors.New("retries cannot be negative")
	}

	return nil
}
