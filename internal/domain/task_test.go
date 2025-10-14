package domain

import (
	"errors"
	"testing"
	"time"
)

func TestNewTask(t *testing.T) {
	tests := []struct {
		name      string
		taskType  TaskType
		priority  Priority
		payload   []byte
		wantError bool
	}{
		{
			name:      "valid task",
			taskType:  TaskTypeImageResize,
			priority:  PriorityNormal,
			payload:   []byte(`{"width": 100}`),
			wantError: false,
		},
		{
			name:      "invalid task type",
			taskType:  TaskType("invalid"),
			priority:  PriorityNormal,
			payload:   []byte(`{}`),
			wantError: true,
		},
		{
			name:      "invalid priority",
			taskType:  TaskTypeImageResize,
			priority:  Priority(99),
			payload:   []byte(`{}`),
			wantError: true,
		},
		{
			name:      "empty payload",
			taskType:  TaskTypeImageResize,
			priority:  PriorityNormal,
			payload:   []byte{},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task, err := NewTask(tt.taskType, tt.priority, tt.payload)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if task.Type != tt.taskType {
				t.Errorf("expected type %s, got %s", tt.taskType, task.Type)
			}

			if task.Priority != tt.priority {
				t.Errorf("expected priority %d, got %d", tt.priority, task.Priority)
			}

			if task.Status != TaskStatusPending {
				t.Errorf("expected status pending, got %s", task.Status)
			}

			if task.MaxRetries != 3 {
				t.Errorf("expected max retries 3, got %d", task.MaxRetries)
			}
		})
	}
}

func TestTask_CanRetry(t *testing.T) {
	tests := []struct {
		name       string
		status     TaskStatus
		retries    int
		maxRetries int
		want       bool
	}{
		{
			name:       "can retry - failed with retries left",
			status:     TaskStatusFailed,
			retries:    1,
			maxRetries: 3,
			want:       true,
		},
		{
			name:       "cannot retry - max retries reached",
			status:     TaskStatusFailed,
			retries:    3,
			maxRetries: 3,
			want:       false,
		},
		{
			name:       "cannot retry - completed task",
			status:     TaskStatusCompleted,
			retries:    0,
			maxRetries: 3,
			want:       false,
		},
		{
			name:       "cannot retry - processing task",
			status:     TaskStatusProcessing,
			retries:    1,
			maxRetries: 3,
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Status:     tt.status,
				Retries:    tt.retries,
				MaxRetries: tt.maxRetries,
			}

			if got := task.CanRetry(); got != tt.want {
				t.Errorf("CanRetry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTask_MarkAsProcessing(t *testing.T) {
	task := &Task{
		Status: TaskStatusPending,
	}

	workerID := "worker-123"
	err := task.MarkAsProcessing(workerID)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if task.Status != TaskStatusProcessing {
		t.Errorf("expected status processing, got %s", task.Status)
	}

	if task.WorkerID != workerID {
		t.Errorf("expected worker ID %s, got %s", workerID, task.WorkerID)
	}

	if task.StartedAt == nil {
		t.Error("expected StartedAt to be set")
	}

	// Test invalid transition
	err = task.MarkAsProcessing(workerID)
	if err == nil {
		t.Error("expected error for invalid status transition")
	}
}

func TestTask_MarkAsCompleted(t *testing.T) {
	task := &Task{
		Status: TaskStatusProcessing,
	}

	result := []byte(`{"result": "success"}`)
	task.MarkAsCompleted(result)

	if task.Status != TaskStatusCompleted {
		t.Errorf("expected status completed, got %s", task.Status)
	}

	if string(task.Result) != string(result) {
		t.Errorf("expected result %s, got %s", result, task.Result)
	}

	if task.CompletedAt == nil {
		t.Error("expected CompletedAt to be set")
	}

	if task.Error != "" {
		t.Errorf("expected error to be empty, got %s", task.Error)
	}
}

func TestTask_MarkAsFailed(t *testing.T) {
	t.Run("with retry", func(t *testing.T) {
		task := &Task{
			Status:     TaskStatusProcessing,
			Retries:    0,
			MaxRetries: 3,
		}

		err := errors.New("processing failed")
		task.MarkAsFailed(err)

		if task.Status != TaskStatusRetrying {
			t.Errorf("expected status retrying, got %s", task.Status)
		}

		if task.Error != err.Error() {
			t.Errorf("expected error %s, got %s", err.Error(), task.Error)
		}

		if task.Retries != 1 {
			t.Errorf("expected retries 1, got %d", task.Retries)
		}

		if task.ScheduledAt == nil {
			t.Error("expected ScheduledAt to be set for retry")
		}
	})

	t.Run("without retry", func(t *testing.T) {
		task := &Task{
			Status:     TaskStatusProcessing,
			Retries:    3,
			MaxRetries: 3,
		}

		err := errors.New("final failure")
		task.MarkAsFailed(err)

		if task.Status != TaskStatusFailed {
			t.Errorf("expected status failed, got %s", task.Status)
		}

		if task.ScheduledAt != nil {
			t.Error("expected ScheduledAt to be nil for final failure")
		}
	})
}

func TestTask_IsTerminal(t *testing.T) {
	tests := []struct {
		name       string
		status     TaskStatus
		retries    int
		maxRetries int
		want       bool
	}{
		{
			name:   "completed is terminal",
			status: TaskStatusCompleted,
			want:   true,
		},
		{
			name:   "cancelled is terminal",
			status: TaskStatusCancelled,
			want:   true,
		},
		{
			name:       "failed without retries is terminal",
			status:     TaskStatusFailed,
			retries:    3,
			maxRetries: 3,
			want:       true,
		},
		{
			name:       "failed with retries is not terminal",
			status:     TaskStatusFailed,
			retries:    1,
			maxRetries: 3,
			want:       false,
		},
		{
			name:   "processing is not terminal",
			status: TaskStatusProcessing,
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				Status:     tt.status,
				Retries:    tt.retries,
				MaxRetries: tt.maxRetries,
			}

			if got := task.IsTerminal(); got != tt.want {
				t.Errorf("IsTerminal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTask_GetProcessingTime(t *testing.T) {
	now := time.Now()
	tenSecondsAgo := now.Add(-10 * time.Second)
	fiveSecondsAgo := now.Add(-5 * time.Second)

	tests := []struct {
		name        string
		startedAt   *time.Time
		completedAt *time.Time
		wantMin     time.Duration
	}{
		{
			name:      "not started",
			startedAt: nil,
			wantMin:   0,
		},
		{
			name:        "completed task",
			startedAt:   &tenSecondsAgo,
			completedAt: &fiveSecondsAgo,
			wantMin:     5 * time.Second,
		},
		{
			name:      "still processing",
			startedAt: &tenSecondsAgo,
			wantMin:   10 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &Task{
				StartedAt:   tt.startedAt,
				CompletedAt: tt.completedAt,
			}

			duration := task.GetProcessingTime()
			if duration < tt.wantMin {
				t.Errorf("GetProcessingTime() = %v, want at least %v", duration, tt.wantMin)
			}
		})
	}
}
