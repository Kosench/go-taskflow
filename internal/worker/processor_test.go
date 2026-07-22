package worker

import (
	"context"
	"errors"
	"testing"

	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLifecycle struct {
	task          *domain.Task
	processErr    error
	updatedStatus domain.TaskStatus
	result        []byte
	errorMessage  string
	retried       bool
}

func (f *fakeLifecycle) ProcessTask(context.Context, string, string) (*domain.Task, error) {
	return f.task, f.processErr
}

func (f *fakeLifecycle) UpdateTaskStatus(_ context.Context, _ string, status domain.TaskStatus, result []byte, errorMessage string) error {
	f.updatedStatus = status
	f.result = result
	f.errorMessage = errorMessage
	return nil
}

func (f *fakeLifecycle) RetryTask(context.Context, string) error {
	f.retried = true
	return nil
}

func TestProcessorHandleTask(t *testing.T) {
	log := logger.New(logger.Config{Level: "disabled"})
	incoming := &domain.Task{ID: "task-1"}

	t.Run("completes successful task", func(t *testing.T) {
		lifecycle := &fakeLifecycle{task: &domain.Task{ID: incoming.ID, MaxRetries: 3}}
		processor, err := NewProcessor("worker-1", lifecycle, TaskHandlerFunc(func(context.Context, *domain.Task) ([]byte, error) {
			return []byte(`{"ok":true}`), nil
		}), log)
		require.NoError(t, err)

		require.NoError(t, processor.HandleTask(context.Background(), incoming))
		assert.Equal(t, domain.TaskStatusCompleted, lifecycle.updatedStatus)
		assert.JSONEq(t, `{"ok":true}`, string(lifecycle.result))
		assert.False(t, lifecycle.retried)
	})

	t.Run("persists failure and schedules retry", func(t *testing.T) {
		lifecycle := &fakeLifecycle{task: &domain.Task{ID: incoming.ID, Retries: 1, MaxRetries: 3}}
		processor, err := NewProcessor("worker-1", lifecycle, TaskHandlerFunc(func(context.Context, *domain.Task) ([]byte, error) {
			return nil, errors.New("temporary failure")
		}), log)
		require.NoError(t, err)

		require.NoError(t, processor.HandleTask(context.Background(), incoming))
		assert.Equal(t, domain.TaskStatusFailed, lifecycle.updatedStatus)
		assert.Equal(t, "temporary failure", lifecycle.errorMessage)
		assert.True(t, lifecycle.retried)
	})

	t.Run("does not retry exhausted task", func(t *testing.T) {
		lifecycle := &fakeLifecycle{task: &domain.Task{ID: incoming.ID, Retries: 3, MaxRetries: 3}}
		processor, err := NewProcessor("worker-1", lifecycle, TaskHandlerFunc(func(context.Context, *domain.Task) ([]byte, error) {
			return nil, errors.New("permanent failure")
		}), log)
		require.NoError(t, err)

		require.NoError(t, processor.HandleTask(context.Background(), incoming))
		assert.Equal(t, domain.TaskStatusFailed, lifecycle.updatedStatus)
		assert.False(t, lifecycle.retried)
	})

	t.Run("ignores duplicate delivery", func(t *testing.T) {
		lifecycle := &fakeLifecycle{processErr: domain.ErrTaskNotFound}
		called := false
		processor, err := NewProcessor("worker-1", lifecycle, TaskHandlerFunc(func(context.Context, *domain.Task) ([]byte, error) {
			called = true
			return nil, nil
		}), log)
		require.NoError(t, err)

		require.NoError(t, processor.HandleTask(context.Background(), incoming))
		assert.False(t, called)
	})
}
