package producer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/sarama"
	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/converter"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/messages"
)

type TaskProducer struct {
	producer  *Producer
	converter *converter.TaskConverter
	config    *config.KafkaConfig
	logger    *logger.Logger
}

func NewTaskProducer(cfg *config.KafkaConfig, log *logger.Logger) (*TaskProducer, error) {
	producer, err := NewProducer(cfg, log)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer %w", err)
	}

	return &TaskProducer{
		producer:  producer,
		converter: converter.NewTaskConverter(),
		config:    cfg,
		logger:    log,
	}, nil
}

func (tp *TaskProducer) PublishTask(ctx context.Context, task *domain.Task) error {
	taskMsg, err := tp.converter.ToKafkaMessage(task)
	if err != nil {
		return fmt.Errorf("failde to convert task: %w", err)
	}

	value, err := json.Marshal(taskMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	topic := tp.getTopicByPriority(task.Priority)

	msg := &sarama.ProducerMessage{
		Topic:     topic,
		Key:       sarama.StringEncoder(task.ID),
		Value:     sarama.ByteEncoder(value),
		Headers:   tp.createHeaders(task),
		Timestamp: task.CreatedAt,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case tp.producer.producer.Input() <- msg:
		tp.logger.Debug().
			Str("task_id", task.ID).
			Str("topic", topic).
			Str("type", string(task.Type)).
			Int("priority", int(task.Priority)).
			Msg("task published to Kafka")
		return nil
	}
}

// PublishTaskResult publishes task processing result
func (tp *TaskProducer) PublishTaskResult(ctx context.Context, result *messages.TaskResultMessage) error {
	// Marshal to JSON
	value, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal result: %w", err)
	}

	// Create producer message
	msg := &sarama.ProducerMessage{
		Topic: tp.config.Topics.TaskResults,
		Key:   sarama.StringEncoder(result.TaskID),
		Value: sarama.ByteEncoder(value),
		Headers: []sarama.RecordHeader{
			{Key: []byte("task_id"), Value: []byte(result.TaskID)},
			{Key: []byte("status"), Value: []byte(result.Status)},
			{Key: []byte("worker_id"), Value: []byte(result.WorkerID)},
		},
	}

	// Send message
	select {
	case <-ctx.Done():
		return ctx.Err()
	case tp.producer.producer.Input() <- msg:
		tp.logger.Debug().
			Str("task_id", result.TaskID).
			Str("status", result.Status).
			Msg("task result published")
		return nil
	}
}

func (tp *TaskProducer) PublishRetry(ctx context.Context, task *domain.Task) error {
	// Increment retry count
	task.IncrementRetries()

	// Convert to retry message
	retryMsg := tp.converter.ToRetryMessage(task, fmt.Errorf("retry attempt %d", task.Retries))

	// Marshal retry info
	retryValue, err := json.Marshal(retryMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal retry message: %w", err)
	}

	// Send to retry topic
	retryInfoMsg := &sarama.ProducerMessage{
		Topic: tp.config.Topics.TasksRetry + "-info",
		Key:   sarama.StringEncoder(task.ID),
		Value: sarama.ByteEncoder(retryValue),
	}

	select {
	case tp.producer.producer.Input() <- retryInfoMsg:
		// Continue to send actual task
	case <-ctx.Done():
		return ctx.Err()
	}

	// Send task to retry topic
	taskMsg, err := tp.converter.ToKafkaMessage(task)
	if err != nil {
		return fmt.Errorf("failed to convert task for retry: %w", err)
	}

	value, err := json.Marshal(taskMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal task for retry: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic:   tp.config.Topics.TasksRetry,
		Key:     sarama.StringEncoder(task.ID),
		Value:   sarama.ByteEncoder(value),
		Headers: tp.createHeaders(task),
	}

	// Schedule retry with delay
	if task.ScheduledAt != nil {
		msg.Timestamp = *task.ScheduledAt
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case tp.producer.producer.Input() <- msg:
		tp.logger.Info().
			Str("task_id", task.ID).
			Int("retry_attempt", task.Retries).
			Time("scheduled_at", *task.ScheduledAt).
			Msg("task scheduled for retry")
		return nil
	}
}

func (tp *TaskProducer) PublishToDLQ(ctx context.Context, task *domain.Task, originalTopic string, err error) error {
	// Create DLQ message
	dlqMsg := tp.converter.ToDLQMessage(task, originalTopic, err)

	// Marshal to JSON
	value, err := json.Marshal(dlqMsg)
	if err != nil {
		return fmt.Errorf("failed to marshal DLQ message: %w", err)
	}

	// Create producer message
	msg := &sarama.ProducerMessage{
		Topic: tp.config.Topics.TasksDLQ,
		Key:   sarama.StringEncoder(task.ID),
		Value: sarama.ByteEncoder(value),
		Headers: []sarama.RecordHeader{
			{Key: []byte("task_id"), Value: []byte(task.ID)},
			{Key: []byte("task_type"), Value: []byte(string(task.Type))},
			{Key: []byte("original_topic"), Value: []byte(originalTopic)},
			{Key: []byte("final_error"), Value: []byte(err.Error())},
		},
	}

	// Send message
	select {
	case <-ctx.Done():
		return ctx.Err()
	case tp.producer.producer.Input() <- msg:
		tp.logger.Warn().
			Str("task_id", task.ID).
			Str("original_topic", originalTopic).
			Err(err).
			Msg("task sent to DLQ")
		return nil
	}
}

func (tp *TaskProducer) getTopicByPriority(priority domain.Priority) string {
	switch priority {
	case domain.PriorityCritical, domain.PriorityHigh:
		return tp.config.Topics.TasksHigh
	case domain.PriorityNormal:
		return tp.config.Topics.TasksNormal
	case domain.PriorityLow:
		return tp.config.Topics.TasksLow
	default:
		return tp.config.Topics.TasksNormal
	}
}

func (tp *TaskProducer) createHeaders(task *domain.Task) []sarama.RecordHeader {
	headers := []sarama.RecordHeader{
		{Key: []byte("task_id"), Value: []byte(task.ID)},
		{Key: []byte("task_type"), Value: []byte(string(task.Type))},
		{Key: []byte("priority"), Value: []byte(fmt.Sprintf("%d", task.Priority))},
		{Key: []byte("retry_count"), Value: []byte(fmt.Sprintf("%d", task.Retries))},
	}

	if task.TraceID != "" {
		headers = append(headers, sarama.RecordHeader{
			Key:   []byte("trace_id"),
			Value: []byte(task.TraceID),
		})
	}

	if task.WorkerID != "" {
		headers = append(headers, sarama.RecordHeader{
			Key:   []byte("worker_id"),
			Value: []byte(task.WorkerID),
		})
	}

	return headers
}

func (tp *TaskProducer) Close() error {
	return tp.producer.Close()
}

func (tp *TaskProducer) GetStats() ProducerStats {
	return tp.producer.GetStats()
}
