package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/database"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/repository"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"strings"
	"time"
)

// TaskRepository implements repository.TaskRepository using PostgreSQL
type TaskRepository struct {
	db *database.DB
}

// NewTaskRepository creates new PostgreSQL task repository
func NewTaskRepository(db *database.DB) *TaskRepository {
	return &TaskRepository{
		db: db,
	}
}

// Create creates a new task
func (r *TaskRepository) Create(ctx context.Context, task *domain.Task) error {
	query := `
		INSERT INTO tasks (
			id, type, status, priority, payload, result, error,
			retries, max_retries, worker_id, trace_id,
			created_at, updated_at, scheduled_at, started_at, completed_at
		) VALUES (
			:id, :type, :status, :priority, :payload, :result, :error,
			:retries, :max_retries, :worker_id, :trace_id,
			:created_at, :updated_at, :scheduled_at, :started_at, :completed_at
		)
	`

	// Convert task to db model
	dbTask := toDBTask(task)

	_, err := r.db.NamedExecContext(ctx, query, dbTask)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok {
			if pqErr.Code == "23505" { // unique_violation
				return domain.ErrTaskAlreadyExists
			}
		}
		return fmt.Errorf("failed to create task: %w", err)
	}

	logger.Get().Debug().
		Str("task_id", task.ID).
		Str("type", string(task.Type)).
		Msg("task created in database")

	return nil
}

// Get retrieves a task by ID
func (r *TaskRepository) Get(ctx context.Context, id string) (*domain.Task, error) {
	query := `
		SELECT
			id, type, status, priority, payload, result, error,
			retries, max_retries, worker_id, trace_id,
			created_at, updated_at, scheduled_at, started_at, completed_at
		FROM tasks
		WHERE id = $1
	`

	var dbTask dbTask
	err := r.db.GetContext(ctx, &dbTask, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	task := toDomainTask(&dbTask)

	// Load metadata if exists
	metadata, err := r.getMetadata(ctx, id)
	if err == nil && len(metadata) > 0 {
		task.Metadata = metadata
	}

	return task, nil
}

// Update updates an existing task
func (r *TaskRepository) Update(ctx context.Context, task *domain.Task) error {
	query := `
		UPDATE tasks SET
			type = :type,
			status = :status,
			priority = :priority,
			payload = :payload,
			result = :result,
			error = :error,
			retries = :retries,
			max_retries = :max_retries,
			worker_id = :worker_id,
			trace_id = :trace_id,
			updated_at = :updated_at,
			scheduled_at = :scheduled_at,
			started_at = :started_at,
			completed_at = :completed_at
		WHERE id = :id
	`

	dbTask := toDBTask(task)

	result, err := r.db.NamedExecContext(ctx, query, dbTask)
	if err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrTaskNotFound
	}

	logger.Get().Debug().
		Str("task_id", task.ID).
		Str("status", string(task.Status)).
		Msg("task updated in database")

	return nil
}

// Delete deletes a task by ID
func (r *TaskRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM tasks WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return domain.ErrTaskNotFound
	}

	return nil
}

// List retrieves tasks with filtering
func (r *TaskRepository) List(ctx context.Context, filter repository.TaskFilter) ([]*domain.Task, error) {
	query := `
		SELECT
			id, type, status, priority, payload, result, error,
			retries, max_retries, worker_id, trace_id,
			created_at, updated_at, scheduled_at, started_at, completed_at
		FROM tasks
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 0

	// Build dynamic query
	if len(filter.Status) > 0 {
		argCount++
		query += fmt.Sprintf(" AND status = ANY($%d)", argCount)
		args = append(args, pq.Array(filter.Status))
	}

	if len(filter.Type) > 0 {
		argCount++
		query += fmt.Sprintf(" AND type = ANY($%d)", argCount)
		args = append(args, pq.Array(filter.Type))
	}

	if len(filter.Priority) > 0 {
		argCount++
		query += fmt.Sprintf(" AND priority = ANY($%d)", argCount)
		args = append(args, pq.Array(filter.Priority))
	}

	if filter.WorkerID != "" {
		argCount++
		query += fmt.Sprintf(" AND worker_id = $%d", argCount)
		args = append(args, filter.WorkerID)
	}

	if filter.TraceID != "" {
		argCount++
		query += fmt.Sprintf(" AND trace_id = $%d", argCount)
		args = append(args, filter.TraceID)
	}

	if !filter.CreatedFrom.IsZero() {
		argCount++
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, filter.CreatedFrom)
	}

	if !filter.CreatedTo.IsZero() {
		argCount++
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, filter.CreatedTo)
	}

	// Add ordering
	orderBy := "created_at"
	if filter.OrderBy != "" {
		orderBy = filter.OrderBy
	}

	orderDir := "DESC"
	if filter.OrderDir != "" {
		orderDir = strings.ToUpper(filter.OrderDir)
	}

	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDir)

	// Add limit and offset
	if filter.Limit > 0 {
		argCount++
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, filter.Limit)
	}

	if filter.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filter.Offset)
	}

	var dbTasks []dbTask
	err := r.db.SelectContext(ctx, &dbTasks, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	tasks := make([]*domain.Task, len(dbTasks))
	for i, dbTask := range dbTasks {
		tasks[i] = toDomainTask(&dbTask)
	}

	return tasks, nil
}

// ListPending retrieves pending tasks ordered by priority and creation time
func (r *TaskRepository) ListPending(ctx context.Context, limit int) ([]*domain.Task, error) {
	query := `
		SELECT
			id, type, status, priority, payload, result, error,
			retries, max_retries, worker_id, trace_id,
			created_at, updated_at, scheduled_at, started_at, completed_at
		FROM tasks
		WHERE status IN ('pending', 'retrying')
			AND (scheduled_at IS NULL OR scheduled_at <= NOW())
		ORDER BY priority DESC, created_at ASC
		LIMIT $1
	`

	var dbTasks []dbTask
	err := r.db.SelectContext(ctx, &dbTasks, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list pending tasks: %w", err)
	}

	tasks := make([]*domain.Task, len(dbTasks))
	for i, dbTask := range dbTasks {
		tasks[i] = toDomainTask(&dbTask)
	}

	return tasks, nil
}

// ListForRetry retrieves tasks that need to be retried
func (r *TaskRepository) ListForRetry(ctx context.Context, before time.Time, limit int) ([]*domain.Task, error) {
	query := `
		SELECT
			id, type, status, priority, payload, result, error,
			retries, max_retries, worker_id, trace_id,
			created_at, updated_at, scheduled_at, started_at, completed_at
		FROM tasks
		WHERE status = 'retrying'
			AND scheduled_at IS NOT NULL
			AND scheduled_at <= $1
		ORDER BY scheduled_at ASC
		LIMIT $2
	`

	var dbTasks []dbTask
	err := r.db.SelectContext(ctx, &dbTasks, query, before, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list tasks for retry: %w", err)
	}

	tasks := make([]*domain.Task, len(dbTasks))
	for i, dbTask := range dbTasks {
		tasks[i] = toDomainTask(&dbTask)
	}

	return tasks, nil
}

// GetStats retrieves task statistics
func (r *TaskRepository) GetStats(ctx context.Context) (*repository.TaskStats, error) {
	stats := &repository.TaskStats{
		TasksByType:     make(map[string]int64),
		TasksByPriority: make(map[string]int64),
	}

	// Get counts by status
	statusQuery := `
		SELECT
			COUNT(*) FILTER (WHERE status = 'pending') as pending,
			COUNT(*) FILTER (WHERE status = 'processing') as processing,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed,
			COUNT(*) FILTER (WHERE status = 'retrying') as retrying,
			COUNT(*) FILTER (WHERE status = 'cancelled') as cancelled,
			COUNT(*) as total
		FROM tasks
	`

	var statCounts struct {
		Pending    int64 `db:"pending"`
		Processing int64 `db:"processing"`
		Completed  int64 `db:"completed"`
		Failed     int64 `db:"failed"`
		Retrying   int64 `db:"retrying"`
		Cancelled  int64 `db:"cancelled"`
		Total      int64 `db:"total"`
	}

	err := r.db.GetContext(ctx, &statCounts, statusQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get task stats: %w", err)
	}

	stats.PendingTasks = statCounts.Pending
	stats.ProcessingTasks = statCounts.Processing
	stats.CompletedTasks = statCounts.Completed
	stats.FailedTasks = statCounts.Failed
	stats.RetryingTasks = statCounts.Retrying
	stats.CancelledTasks = statCounts.Cancelled
	stats.TotalTasks = statCounts.Total

	// Get counts by type
	typeQuery := `
		SELECT type, COUNT(*) as count
		FROM tasks
		GROUP BY type
	`

	rows, err := r.db.QueryContext(ctx, typeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get task type stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var taskType string
		var count int64
		if err := rows.Scan(&taskType, &count); err != nil {
			continue
		}
		stats.TasksByType[taskType] = count
	}

	// Get average times
	timeQuery := `
		SELECT
			COALESCE(AVG(EXTRACT(EPOCH FROM (completed_at - started_at))), 0) as avg_processing_time,
			COALESCE(AVG(EXTRACT(EPOCH FROM (started_at - created_at))), 0) as avg_waiting_time
		FROM tasks
		WHERE completed_at IS NOT NULL AND started_at IS NOT NULL
	`

	var avgProcessing, avgWaiting float64
	err = r.db.QueryRowContext(ctx, timeQuery).Scan(&avgProcessing, &avgWaiting)
	if err == nil {
		stats.AvgProcessingTime = time.Duration(avgProcessing * float64(time.Second))
		stats.AvgWaitingTime = time.Duration(avgWaiting * float64(time.Second))
	}

	return stats, nil
}

// BulkCreate creates multiple tasks in a transaction
func (r *TaskRepository) BulkCreate(ctx context.Context, tasks []*domain.Task) error {
	return r.db.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		query := `
			INSERT INTO tasks (
				id, type, status, priority, payload, result, error,
				retries, max_retries, worker_id, trace_id,
				created_at, updated_at, scheduled_at, started_at, completed_at
			) VALUES (
				:id, :type, :status, :priority, :payload, :result, :error,
				:retries, :max_retries, :worker_id, :trace_id,
				:created_at, :updated_at, :scheduled_at, :started_at, :completed_at
			)
		`

		for _, task := range tasks {
			dbTask := toDBTask(task)
			if _, err := tx.NamedExecContext(ctx, query, dbTask); err != nil {
				return fmt.Errorf("failed to create task %s: %w", task.ID, err)
			}
		}

		logger.Get().Debug().
			Int("count", len(tasks)).
			Msg("bulk created tasks")

		return nil
	})
}

// CleanupOldTasks removes completed/failed tasks older than specified duration
func (r *TaskRepository) CleanupOldTasks(ctx context.Context, before time.Time) (int64, error) {
	query := `
		DELETE FROM tasks
		WHERE status IN ('completed', 'failed', 'cancelled')
			AND updated_at < $1
	`

	result, err := r.db.ExecContext(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old tasks: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get rows affected: %w", err)
	}

	logger.Get().Info().
		Int64("deleted", rowsAffected).
		Time("before", before).
		Msg("cleaned up old tasks")

	return rowsAffected, nil
}

// LockTaskForProcessing locks a task for processing by a worker
func (r *TaskRepository) LockTaskForProcessing(ctx context.Context, taskID, workerID string) (*domain.Task, error) {
	query := `
		UPDATE tasks
		SET status = 'processing',
			worker_id = $2,
			started_at = NOW(),
			updated_at = NOW()
		WHERE id = $1
			AND status IN ('pending', 'retrying')
			AND (scheduled_at IS NULL OR scheduled_at <= NOW())
		RETURNING
			id, type, status, priority, payload, result, error,
			retries, max_retries, worker_id, trace_id,
			created_at, updated_at, scheduled_at, started_at, completed_at
	`

	var dbTask dbTask
	err := r.db.GetContext(ctx, &dbTask, query, taskID, workerID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrTaskNotFound
		}
		return nil, fmt.Errorf("failed to lock task for processing: %w", err)
	}

	return toDomainTask(&dbTask), nil
}

// getMetadata retrieves metadata for a task
func (r *TaskRepository) getMetadata(ctx context.Context, taskID string) (map[string]string, error) {
	query := `SELECT key, value FROM task_metadata WHERE task_id = $1`

	rows, err := r.db.QueryContext(ctx, query, taskID)
	if err != nil {
		return nil, err
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
