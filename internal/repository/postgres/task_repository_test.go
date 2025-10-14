package postgres

import (
	"context"
	"errors"
	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/database"
	"github.com/Kosench/go-taskflow/internal/repository"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) *database.DB {
	cfg := &config.DatabaseConfig{
		Host:            "localhost",
		Port:            5432,
		Name:            "taskqueue",
		User:            "taskqueue",
		Password:        "taskqueue",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}

	db, err := database.New(cfg)
	require.NoError(t, err)

	// Clean up test data
	_, _ = db.Exec("DELETE FROM task_history")
	_, _ = db.Exec("DELETE FROM task_metadata")
	_, _ = db.Exec("DELETE FROM tasks")

	return db
}

func createTestTask() *domain.Task {
	// Create valid JSON payload
	payload := []byte(`{"width": 100, "height": 100}`)

	task, _ := domain.NewTask(
		domain.TaskTypeImageResize,
		domain.PriorityNormal,
		payload,
	)
	task.ID = uuid.New().String()
	return task
}

func TestTaskRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewTaskRepository(db)
	ctx := context.Background()

	t.Run("create valid task", func(t *testing.T) {
		task := createTestTask()

		err := repo.Create(ctx, task)
		assert.NoError(t, err)

		// Verify task was created
		retrieved, err := repo.Get(ctx, task.ID)
		assert.NoError(t, err)
		require.NotNil(t, retrieved)
		assert.Equal(t, task.ID, retrieved.ID)
		assert.Equal(t, task.Type, retrieved.Type)
		assert.Equal(t, task.Status, retrieved.Status)
	})

	t.Run("create duplicate task", func(t *testing.T) {
		task := createTestTask()

		err := repo.Create(ctx, task)
		assert.NoError(t, err)

		// Try to create duplicate
		err = repo.Create(ctx, task)
		assert.Equal(t, domain.ErrTaskAlreadyExists, err)
	})
}

func TestTaskRepository_Get(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewTaskRepository(db)
	ctx := context.Background()

	t.Run("get existing task", func(t *testing.T) {
		task := createTestTask()
		err := repo.Create(ctx, task)
		require.NoError(t, err)

		retrieved, err := repo.Get(ctx, task.ID)
		assert.NoError(t, err)
		assert.Equal(t, task.ID, retrieved.ID)
		assert.Equal(t, task.Type, retrieved.Type)
	})

	t.Run("get non-existent task", func(t *testing.T) {
		_, err := repo.Get(ctx, "non-existent-id")
		assert.Equal(t, domain.ErrTaskNotFound, err)
	})
}

func TestTaskRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewTaskRepository(db)
	ctx := context.Background()

	t.Run("update existing task", func(t *testing.T) {
		task := createTestTask()
		err := repo.Create(ctx, task)
		require.NoError(t, err)

		// Update task
		err = task.MarkAsProcessing("worker-123")
		require.NoError(t, err)

		err = repo.Update(ctx, task)
		assert.NoError(t, err)

		// Verify update
		retrieved, err := repo.Get(ctx, task.ID)
		assert.NoError(t, err)
		assert.Equal(t, domain.TaskStatusProcessing, retrieved.Status)
		assert.Equal(t, "worker-123", retrieved.WorkerID)
	})

	t.Run("update non-existent task", func(t *testing.T) {
		task := createTestTask()
		err := repo.Update(ctx, task)
		assert.Equal(t, domain.ErrTaskNotFound, err)
	})
}

func TestTaskRepository_ListPending(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewTaskRepository(db)
	ctx := context.Background()

	// Create tasks with different priorities
	highPriorityTask := createTestTask()
	highPriorityTask.Priority = domain.PriorityHigh

	normalPriorityTask := createTestTask()
	normalPriorityTask.Priority = domain.PriorityNormal

	lowPriorityTask := createTestTask()
	lowPriorityTask.Priority = domain.PriorityLow

	// Create tasks
	require.NoError(t, repo.Create(ctx, normalPriorityTask))
	require.NoError(t, repo.Create(ctx, highPriorityTask))
	require.NoError(t, repo.Create(ctx, lowPriorityTask))

	// List pending tasks
	tasks, err := repo.ListPending(ctx, 10)
	assert.NoError(t, err)
	assert.Len(t, tasks, 3)

	// Verify order (high priority first)
	assert.Equal(t, domain.PriorityHigh, tasks[0].Priority)
	assert.Equal(t, domain.PriorityNormal, tasks[1].Priority)
	assert.Equal(t, domain.PriorityLow, tasks[2].Priority)
}

func TestTaskRepository_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewTaskRepository(db)
	ctx := context.Background()

	// Create test tasks
	for i := 0; i < 5; i++ {
		task := createTestTask()
		if i%2 == 0 {
			task.Type = domain.TaskTypeSendEmail
		}
		require.NoError(t, repo.Create(ctx, task))
	}

	t.Run("list all tasks", func(t *testing.T) {
		filter := repository.TaskFilter{
			Limit: 10,
		}

		tasks, err := repo.List(ctx, filter)
		assert.NoError(t, err)
		assert.Len(t, tasks, 5)
	})

	t.Run("filter by type", func(t *testing.T) {
		filter := repository.TaskFilter{
			Type:  []domain.TaskType{domain.TaskTypeSendEmail},
			Limit: 10,
		}

		tasks, err := repo.List(ctx, filter)
		assert.NoError(t, err)
		assert.Len(t, tasks, 3)

		for _, task := range tasks {
			assert.Equal(t, domain.TaskTypeSendEmail, task.Type)
		}
	})

	t.Run("pagination", func(t *testing.T) {
		filter := repository.TaskFilter{
			Limit:  2,
			Offset: 0,
		}

		tasks1, err := repo.List(ctx, filter)
		assert.NoError(t, err)
		assert.Len(t, tasks1, 2)

		filter.Offset = 2
		tasks2, err := repo.List(ctx, filter)
		assert.NoError(t, err)
		assert.Len(t, tasks2, 2)

		// Ensure different tasks
		assert.NotEqual(t, tasks1[0].ID, tasks2[0].ID)
	})
}

func TestTaskRepository_BulkCreate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewTaskRepository(db)
	ctx := context.Background()

	// Create multiple tasks
	tasks := make([]*domain.Task, 5)
	for i := 0; i < 5; i++ {
		tasks[i] = createTestTask()
	}

	err := repo.BulkCreate(ctx, tasks)
	assert.NoError(t, err)

	// Verify all tasks were created
	for _, task := range tasks {
		retrieved, err := repo.Get(ctx, task.ID)
		assert.NoError(t, err)
		assert.Equal(t, task.ID, retrieved.ID)
	}
}

func TestTaskRepository_LockTaskForProcessing(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewTaskRepository(db)
	ctx := context.Background()

	task := createTestTask()
	err := repo.Create(ctx, task)
	require.NoError(t, err)

	// Lock task for processing
	workerID := "worker-456"
	lockedTask, err := repo.LockTaskForProcessing(ctx, task.ID, workerID)
	assert.NoError(t, err)
	assert.Equal(t, domain.TaskStatusProcessing, lockedTask.Status)
	assert.Equal(t, workerID, lockedTask.WorkerID)
	assert.NotNil(t, lockedTask.StartedAt)

	// Try to lock again (should fail)
	_, err = repo.LockTaskForProcessing(ctx, task.ID, "another-worker")
	assert.Equal(t, domain.ErrTaskNotFound, err)
}

func TestTaskRepository_CleanupOldTasks(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewTaskRepository(db)
	ctx := context.Background()

	// Create old completed task
	oldTask := createTestTask()
	oldTask.MarkAsCompleted([]byte(`{"result": "success"}`))
	err := repo.Create(ctx, oldTask)
	require.NoError(t, err)

	// Force update the updated_at to simulate old task (bypass trigger)
	oldTime := time.Now().Add(-48 * time.Hour)

	// Disable trigger
	_, err = db.Exec("ALTER TABLE tasks DISABLE TRIGGER update_tasks_updated_at")
	require.NoError(t, err)

	// Update the task with old time
	_, err = db.Exec("UPDATE tasks SET updated_at = $1 WHERE id = $2", oldTime, oldTask.ID)
	require.NoError(t, err)

	// Re-enable trigger
	_, err = db.Exec("ALTER TABLE tasks ENABLE TRIGGER update_tasks_updated_at")
	require.NoError(t, err)

	// Create recent task
	recentTask := createTestTask()
	err = repo.Create(ctx, recentTask)
	require.NoError(t, err)

	// Cleanup tasks older than 24 hours
	before := time.Now().Add(-24 * time.Hour)
	deleted, err := repo.CleanupOldTasks(ctx, before)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	// Verify old task was deleted
	_, err = repo.Get(ctx, oldTask.ID)
	assert.Equal(t, domain.ErrTaskNotFound, err)

	// Verify recent task still exists
	_, err = repo.Get(ctx, recentTask.ID)
	assert.NoError(t, err)
}

func TestTaskRepository_GetStats(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	repo := NewTaskRepository(db)
	ctx := context.Background()

	// Create tasks with different statuses
	pendingTask := createTestTask()
	require.NoError(t, repo.Create(ctx, pendingTask))

	completedTask := createTestTask()
	completedTask.Status = domain.TaskStatusCompleted
	now := time.Now()
	tenSecondsAgo := now.Add(-10 * time.Second)
	completedTask.StartedAt = &tenSecondsAgo
	completedTask.CompletedAt = &now
	require.NoError(t, repo.Create(ctx, completedTask))

	failedTask := createTestTask()
	// Exhaust retries so it becomes truly failed
	failedTask.Retries = failedTask.MaxRetries
	failedTask.MarkAsFailed(errors.New("test error"))
	require.NoError(t, repo.Create(ctx, failedTask))

	// Get stats
	stats, err := repo.GetStats(ctx)
	assert.NoError(t, err)
	require.NotNil(t, stats)
	assert.Equal(t, int64(3), stats.TotalTasks)
	assert.Equal(t, int64(1), stats.PendingTasks)
	assert.Equal(t, int64(1), stats.CompletedTasks)
	assert.Equal(t, int64(1), stats.FailedTasks)
}
