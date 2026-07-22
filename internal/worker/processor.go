package worker

import (
	"context"
	"errors"
	"fmt"

	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
)

// TaskLifecycle contains the state transitions required by a worker.
type TaskLifecycle interface {
	ProcessTask(ctx context.Context, taskID string, workerID string) (*domain.Task, error)
	UpdateTaskStatus(ctx context.Context, taskID string, status domain.TaskStatus, result []byte, errorMsg string) error
	RetryTask(ctx context.Context, taskID string) error
}

// TaskHandler executes the task-specific business operation.
type TaskHandler interface {
	HandleTask(ctx context.Context, task *domain.Task) ([]byte, error)
}

type TaskHandlerFunc func(ctx context.Context, task *domain.Task) ([]byte, error)

func (f TaskHandlerFunc) HandleTask(ctx context.Context, task *domain.Task) ([]byte, error) {
	return f(ctx, task)
}

// Processor owns the durable task lifecycle around a business handler.
type Processor struct {
	workerID  string
	lifecycle TaskLifecycle
	handler   TaskHandler
	logger    *logger.Logger
}

func NewProcessor(workerID string, lifecycle TaskLifecycle, handler TaskHandler, log *logger.Logger) (*Processor, error) {
	if workerID == "" {
		return nil, fmt.Errorf("worker ID is required")
	}
	if lifecycle == nil {
		return nil, fmt.Errorf("task lifecycle is required")
	}
	if handler == nil {
		return nil, fmt.Errorf("task handler is required")
	}

	return &Processor{
		workerID:  workerID,
		lifecycle: lifecycle,
		handler:   handler,
		logger:    log,
	}, nil
}

// HandleTask locks, executes and durably records the result of a task.
// A task which was already locked is treated as a harmless duplicate.
func (p *Processor) HandleTask(ctx context.Context, incoming *domain.Task) error {
	if incoming == nil || incoming.ID == "" {
		return fmt.Errorf("task with ID is required")
	}

	task, err := p.lifecycle.ProcessTask(ctx, incoming.ID, p.workerID)
	if err != nil {
		if errors.Is(err, domain.ErrTaskNotFound) {
			p.logger.Debug().Str("task_id", incoming.ID).Msg("duplicate or unavailable task ignored")
			return nil
		}
		return fmt.Errorf("failed to claim task: %w", err)
	}

	result, handleErr := p.handler.HandleTask(ctx, task)
	if handleErr == nil {
		if err := p.lifecycle.UpdateTaskStatus(ctx, task.ID, domain.TaskStatusCompleted, result, ""); err != nil {
			return fmt.Errorf("failed to complete task: %w", err)
		}
		return nil
	}

	if err := p.lifecycle.UpdateTaskStatus(ctx, task.ID, domain.TaskStatusFailed, nil, handleErr.Error()); err != nil {
		return fmt.Errorf("failed to persist task failure: %w", err)
	}

	if task.Retries < task.MaxRetries {
		if err := p.lifecycle.RetryTask(ctx, task.ID); err != nil {
			return fmt.Errorf("failed to schedule task retry: %w", err)
		}
	}

	p.logger.Warn().
		Err(handleErr).
		Str("task_id", task.ID).
		Int("retries", task.Retries).
		Int("max_retries", task.MaxRetries).
		Msg("task handler failed")

	// The failure and its retry decision are durable. Kafka may commit the input.
	return nil
}
