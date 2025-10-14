package postgres

import (
	"context"
	"fmt"
	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/database"
	"github.com/Kosench/go-taskflow/internal/repository"
)

// TaskHistoryRepository implements repository.TaskHistoryRepository
type TaskHistoryRepository struct {
	db *database.DB
}

// NewTaskHistoryRepository creates new task history repository
func NewTaskHistoryRepository(db *database.DB) *TaskHistoryRepository {
	return &TaskHistoryRepository{
		db: db,
	}
}

// AddHistory adds a history entry for a task
func (r *TaskHistoryRepository) AddHistory(ctx context.Context, taskID string, status domain.TaskStatus, message string, createdBy string) error {
	query := `
		INSERT INTO task_history (task_id, status, message, created_by, created_at)
		VALUES ($1, $2, $3, $4, NOW())
	`

	_, err := r.db.ExecContext(ctx, query, taskID, status, message, createdBy)
	if err != nil {
		return fmt.Errorf("failed to add task history: %w", err)
	}

	return nil
}

// GetHistory retrieves history for a task
func (r *TaskHistoryRepository) GetHistory(ctx context.Context, taskID string) ([]*repository.TaskHistory, error) {
	query := `
		SELECT id, task_id, status, message, created_at, created_by
		FROM task_history
		WHERE task_id = $1
		ORDER BY created_at DESC
	`

	var history []*repository.TaskHistory
	err := r.db.SelectContext(ctx, &history, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get task history: %w", err)
	}

	return history, nil
}
