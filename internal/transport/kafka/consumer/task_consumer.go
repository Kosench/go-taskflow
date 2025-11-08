package consumer

import (
	"context"
	"fmt"

	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/converter"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/messages"
)

type TaskHandler interface {
	HandleTask(ctx context.Context, task *domain.Task) error
}

type TaskHandlerFunc func(ctx context.Context, task *domain.Task) error

func (f TaskHandlerFunc) HandleTask(ctx context.Context, task *domain.Task) error {
	return f(ctx, task)
}

type TaskConsumer struct {
	consumer  *Consumer
	converter *converter.TaskConverter
	handler   TaskHandler
	logger    *logger.Logger
}

func NewTaskConsumer(cfg *config.KafkaConfig, handler TaskHandler, log *logger.Logger) (*TaskConsumer, error) {
	tc := &TaskConsumer{
		converter: converter.NewTaskConverter(),
		handler:   handler,
		logger:    log,
	}

	msgHandler := MessageHandlerFunc(func(ctx context.Context, msg *messages.TaskMessage) error {
		return tc.handleMessage(ctx, msg)
	})

	consumer, err := NewConsumer(cfg, msgHandler, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}

	tc.consumer = consumer
	return tc, err
}

func (tc *TaskConsumer) Start(ctx context.Context) error {
	return tc.consumer.Start(ctx)
}

func (tc *TaskConsumer) Stop() error {
	return tc.consumer.Stop()
}

func (tc *TaskConsumer) handleMessage(ctx context.Context, msg *messages.TaskMessage) error {
	task, err := tc.converter.ToDomainTask(msg)
	if err != nil {
		tc.logger.Error().
			Err(err).
			Str("message_id", msg.ID).
			Msg("failed to convert message to task")
		return err
	}

	ctx = logger.WithFields(ctx, map[string]interface{}{
		"task_id":   task.ID,
		"task_type": string(task.Type),
		"priority":  int(task.Priority),
		"retries":   task.Retries,
	})

	if err := tc.handler.HandleTask(ctx, task); err != nil {
		tc.logger.Error().
			Err(err).
			Str("task_id", task.ID).
			Msg("failed to handle task")
		return err
	}

	return nil
}

func (tc *TaskConsumer) GetStats() ConsumerStats {
	return tc.consumer.GetStats()
}
