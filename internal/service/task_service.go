package service

import (
	"context"
	"fmt"
	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/repository"
	"github.com/Kosench/go-taskflow/internal/transport/kafka"
	"github.com/google/uuid"
	"time"
)

type taskService struct {
	repo           repository.TaskRepository
	producer       *kafka.TaskProducer
	logger         *logger.Logger
	defaultRetries int
}

func NewTaskService(repo repository.TaskRepository, producer *kafka.TaskProducer, logger *logger.Logger) TaskService {
	return &taskService{
		repo:           repo,
		producer:       producer,
		logger:         logger,
		defaultRetries: 3,
	}
}

// CreateTask creates a new task
func (s *taskService) CreateTask(ctx context.Context, req CreateTaskRequest) (*domain.Task, error) {
	// Validate request
	if err := req.Validate(); err != nil {
		s.logger.Error().Err(err).Msg("invalid task creation request")
		return nil, err
	}

	// Create domain task
	task, err := domain.NewTask(req.Type, req.Priority, req.Payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}

	// Set additional fields
	task.ID = uuid.New().String()
	if req.MaxRetries > 0 {
		task.MaxRetries = req.MaxRetries
	} else {
		task.MaxRetries = s.defaultRetries
	}

	if req.ScheduledAt != nil {
		task.ScheduledAt = req.ScheduledAt
	}

	if req.TraceID != "" {
		task.TraceID = req.TraceID
	} else {
		task.TraceID = uuid.New().String()
	}

	task.Metadata = req.Metadata

	// Add trace ID to context
	ctx = logger.WithCorrelationID(ctx, task.TraceID)

	// Save to database
	if err := s.repo.Create(ctx, task); err != nil {
		s.logger.Error().
			Err(err).
			Str("task_id", task.ID).
			Msg("failed to save task to database")
		return nil, fmt.Errorf("failed to save task: %w", err)
	}

	// Publish to Kafka
	if err := s.producer.PublishTask(ctx, task); err != nil {
		// Log error but don't fail - worker will pick up from DB
		s.logger.Error().
			Err(err).
			Str("task_id", task.ID).
			Msg("failed to publish task to Kafka")

		// Update task status to indicate Kafka publish failed
		// Worker will still process it from DB polling
	}

	s.logger.Info().
		Str("task_id", task.ID).
		Str("type", string(task.Type)).
		Int("priority", int(task.Priority)).
		Str("trace_id", task.TraceID).
		Msg("task created successfully")

	// Track metrics
	taskCreatedCounter.WithLabelValues(string(task.Type)).Inc()

	return task, nil
}

// GetTask retrieves a task by ID
func (s *taskService) GetTask(ctx context.Context, taskID string) (*domain.Task, error) {
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		if err == domain.ErrTaskNotFound {
			return nil, err
		}
		s.logger.Error().
			Err(err).
			Str("task_id", taskID).
			Msg("failed to get task")
		return nil, fmt.Errorf("failed to get task: %w", err)
	}

	return task, nil
}

// ListTasks lists tasks with filtering
func (s *taskService) ListTask(ctx context.Context, req ListTasksRequest) ([]*domain.Task, error) {
	filter := repository.TaskFilter{
		Status:      req.Status,
		Type:        req.Type,
		Priority:    req.Priority,
		WorkerID:    req.WorkerID,
		TraceID:     req.TraceID,
		CreatedFrom: req.CreatedFrom,
		CreatedTo:   req.CreatedTo,
		Limit:       req.Limit,
		Offset:      req.Offset,
		OrderBy:     req.OrderBy,
		OrderDir:    req.OrderDir,
	}

	if filter.Limit <= 0 {
		filter.Limit = 10
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}

	tasks, err := s.repo.List(ctx, filter)
	if err != nil {
		s.logger.Error().
			Err(err).
			Interface("filter", filter).
			Msg("failed to list tasks")
		return nil, fmt.Errorf("failed to list tasks: %w", err)
	}

	return tasks, nil
}

func (s *taskService) UpdateTaskStatus(ctx context.Context, taskID string, status domain.TaskStatus, result []byte, errorMsg string) error {
	if !status.IsValid() {
		return fmt.Errorf("invalid status: %s", status)
	}

	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	previousStatus := task.Status
	task.Status = status
	task.UpdatedAt = time.Now()

	if len(result) > 0 {
		task.Result = result
	}

	if status == domain.TaskStatusProcessing && task.StartedAt == nil {
		now := time.Now()
		task.StartedAt = &now
	}

	if errorMsg != "" {
		task.Error = errorMsg
	}

	if status == domain.TaskStatusCompleted || status == domain.TaskStatusCancelled {
		now := time.Now()
		task.CompletedAt = &now
	}

	if err := s.repo.Update(ctx, task); err != nil {
		s.logger.Error().
			Err(err).
			Str("taskID", task.ID).
			Str("status", string(status)).
			Msg("failed to update task status")
		return fmt.Errorf("failed to update task: %w", err)
	}

	if task.IsTerminal() {
		duration := task.GetProcessingTime()
		resultMsg := s.producer.CreateResultMessage(task, task.WorkerID, duration)

		if err := s.producer.PublishTaskResult(ctx, resultMsg); err != nil {
			s.logger.Error().
				Err(err).
				Str("task_id", task.ID).
				Msg("failed to publish task result")
		}
	}

	s.logger.Info().
		Str("task_id", taskID).
		Str("previous_status", string(previousStatus)).
		Str("new_status", string(status)).
		Msg("task status updated")

	// Track metrics
	taskStatusChangedCounter.WithLabelValues(string(previousStatus), string(status)).Inc()

	return nil
}
func (s *taskService) ProcessTask(ctx context.Context, taskID string, workerID string) error {
	// Lock task for processing
	task, err := s.repo.LockTaskForProcessing(ctx, taskID, workerID)
	if err != nil {
		if err == domain.ErrTaskNotFound {
			s.logger.Warn().
				Str("task_id", taskID).
				Str("worker_id", workerID).
				Msg("task not found or already processing")
			return err
		}
		return fmt.Errorf("failed to lock task: %w", err)
	}

	s.logger.Info().
		Str("task_id", taskID).
		Str("worker_id", workerID).
		Str("type", string(task.Type)).
		Msg("task locked for processing")

	// Track metrics
	tasksProcessingGauge.Inc()
	defer tasksProcessingGauge.Dec()

	return nil
}

func (s *taskService) RetryTask(ctx context.Context, taskID string) error {
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	if task.Status != domain.TaskStatusFailed {
		return fmt.Errorf("cannot retry task in status %s (must be failed)", task.Status)
	}

	if !task.CanRetry() {
		return fmt.Errorf("task cannot be retried: max retries reached (%d/%d)",
			task.Retries, task.MaxRetries)
	}

	task.Status = domain.TaskStatusRetrying
	task.IncrementRetries()
	task.UpdatedAt = time.Now()

	task.WorkerID = ""
	task.StartedAt = nil
	task.Error = ""
	task.Result = nil

	// 6. Вычислить exponential backoff
	backoff := s.calculateRetryDelay(task.Retries)
	nextRetry := time.Now().Add(backoff)
	task.ScheduledAt = &nextRetry

	// Update in database
	if err := s.repo.UpdateForRetry(ctx, task); err != nil {
		return fmt.Errorf("failed to update task for retry: %w", err)
	}

	if err := s.producer.PublishRetry(ctx, task); err != nil {
		s.logger.Error().
			Err(err).
			Str("task_id", taskID).
			Msg("failed to publish retry to Kafka")

		// Критическая ошибка - откатить изменения в БД
		task.Status = domain.TaskStatusFailed
		if rollbackErr := s.repo.Update(ctx, task); rollbackErr != nil {
			s.logger.Error().
				Err(rollbackErr).
				Str("task_id", taskID).
				Msg("failed to rollback task status")
		}
		return fmt.Errorf("failed to publish retry: %w", err)
	}

	s.logger.Info().
		Str("task_id", taskID).
		Int("retry_attempt", task.Retries).
		Time("scheduled_at", nextRetry).
		Msg("task scheduled for retry")

	s.logger.Info().
		Str("task_id", taskID).
		Int("retry_attempt", task.Retries).
		Int("max_retries", task.MaxRetries).
		Dur("backoff", backoff).
		Time("scheduled_at", nextRetry).
		Msg("task scheduled for retry")

	// Track metrics
	taskRetriedCounter.WithLabelValues(string(task.Type)).Inc()
	taskRetryBackoffHistogram.WithLabelValues(string(task.Type)).Observe(backoff.Seconds())

	return nil
}

// CancelTask cancels a task
func (s *taskService) CancelTask(ctx context.Context, taskID string, reason string) error {
	// Get task
	task, err := s.repo.Get(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get task: %w", err)
	}

	// Check if task can be cancelled
	if task.IsTerminal() {
		return fmt.Errorf("task is already in terminal state: %s", task.Status)
	}

	// Mark as cancelled
	task.MarkAsCancelled(reason)

	// Update in database
	if err := s.repo.Update(ctx, task); err != nil {
		return fmt.Errorf("failed to update task: %w", err)
	}

	s.logger.Info().
		Str("task_id", taskID).
		Str("reason", reason).
		Msg("task cancelled")

	// Track metrics
	taskCancelledCounter.WithLabelValues(string(task.Type)).Inc()

	return nil
}

// GetTaskStats returns task statistics
func (s *taskService) GetTaskStats(ctx context.Context) (*repository.TaskStats, error) {
	stats, err := s.repo.GetStats(ctx)
	if err != nil {
		s.logger.Error().Err(err).Msg("failed to get task stats")
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	return stats, nil
}

// CleanupOldTasks removes old completed/failed tasks
func (s *taskService) CleanupOldTasks(ctx context.Context, olderThan time.Duration) (int64, error) {
	before := time.Now().Add(-olderThan)

	count, err := s.repo.CleanupOldTasks(ctx, before)
	if err != nil {
		s.logger.Error().
			Err(err).
			Dur("older_than", olderThan).
			Msg("failed to cleanup old tasks")
		return 0, fmt.Errorf("failed to cleanup tasks: %w", err)
	}

	s.logger.Info().
		Int64("count", count).
		Dur("older_than", olderThan).
		Msg("old tasks cleaned up")

	return count, nil
}

// calculateRetryDelay вычисляет exponential backoff
func (s *taskService) calculateRetryDelay(retries int) time.Duration {
	if retries <= 0 {
		return time.Second
	}

	// Exponential backoff: 2^retries seconds
	backoff := time.Duration(1<<uint(retries)) * time.Second

	// Cap at 5 minutes
	maxBackoff := 5 * time.Minute
	if backoff > maxBackoff {
		backoff = maxBackoff
	}

	return backoff
}
