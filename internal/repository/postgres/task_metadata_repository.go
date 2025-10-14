package postgres

import (
	"context"
	"fmt"
	"github.com/Kosench/go-taskflow/internal/pkg/database"
	"github.com/jmoiron/sqlx"
)

// TaskMetadataRepository implements repository.TaskMetadataRepository
type TaskMetadataRepository struct {
	db *database.DB
}

// NewTaskMetadataRepository creates new task metadata repository
func NewTaskMetadataRepository(db *database.DB) *TaskMetadataRepository {
	return &TaskMetadataRepository{
		db: db,
	}
}

// SetMetadata sets metadata for a task
func (r *TaskMetadataRepository) SetMetadata(ctx context.Context, taskID string, metadata map[string]string) error {
	return r.db.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		// Delete existing metadata
		deleteQuery := `DELETE FROM task_metadata WHERE task_id = $1`
		if _, err := tx.ExecContext(ctx, deleteQuery, taskID); err != nil {
			return fmt.Errorf("failed to delete existing metadata: %w", err)
		}

		// Insert new metadata
		if len(metadata) > 0 {
			insertQuery := `
				INSERT INTO task_metadata (task_id, key, value)
				VALUES ($1, $2, $3)
			`

			for key, value := range metadata {
				if _, err := tx.ExecContext(ctx, insertQuery, taskID, key, value); err != nil {
					return fmt.Errorf("failed to insert metadata: %w", err)
				}
			}
		}

		return nil
	})
}

// GetMetadata retrieves metadata for a task
func (r *TaskMetadataRepository) GetMetadata(ctx context.Context, taskID string) (map[string]string, error) {
	query := `
		SELECT key, value
		FROM task_metadata
		WHERE task_id = $1
	`

	rows, err := r.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %w", err)
	}
	defer rows.Close()

	metadata := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		metadata[key] = value
	}

	return metadata, nil
}

// DeleteMetadata deletes metadata for a task
func (r *TaskMetadataRepository) DeleteMetadata(ctx context.Context, taskID string) error {
	query := `DELETE FROM task_metadata WHERE task_id = $1`

	_, err := r.db.ExecContext(ctx, query, taskID)
	if err != nil {
		return fmt.Errorf("failed to delete metadata: %w", err)
	}

	return nil
}
