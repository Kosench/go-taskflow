package producer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	kafkaconfig "github.com/Kosench/go-taskflow/internal/transport/kafka/config"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/messages"
	"github.com/google/uuid"
)

type Producer struct {
	producer sarama.AsyncProducer
	config   *config.KafkaConfig
	logger   *logger.Logger

	//Metrics
	messagesSent   int64
	messagesErrors int64
	mu             sync.RWMutex

	// Shutdown
	closed chan struct{}
	wg     sync.WaitGroup
}

type deliveryResult struct {
	err error
}

type deliveryMetadata struct {
	result chan deliveryResult
}

func NewProducer(cfg *config.KafkaConfig, log *logger.Logger) (*Producer, error) {
	saramaConfig := kafkaconfig.BuildProducerConfig(&cfg.Producer)

	producer, err := sarama.NewAsyncProducer(cfg.Brokers, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failde to create Kafka producer: %w", err)
	}

	p := &Producer{
		producer: producer,
		config:   cfg,
		logger:   log,
		closed:   make(chan struct{}),
	}

	// Start monitoring goroutines
	p.wg.Add(2)
	go p.handleSuccesses()
	go p.handleErrors()

	log.Info().
		Strs("brokers", cfg.Brokers).
		Msg("Kafka producer started")

	return p, nil
}

func (p *Producer) SendTask(ctx context.Context, task *domain.Task) error {
	topic := p.getTopicByPriority(task.Priority)

	msg, err := p.prepareTaskMessage(task, topic)
	if err != nil {
		return fmt.Errorf("failed to prepare message: %w", err)
	}

	if err := p.SendMessage(ctx, msg); err != nil {
		return err
	}

	p.logger.Debug().
		Str("task_id", task.ID).
		Str("topic", topic).
		Str("task_type", string(task.Type)).
		Msg("task sent to Kafka")
	return nil
}

// SendMessage publishes a prepared message and waits for Kafka acknowledgement.
func (p *Producer) SendMessage(ctx context.Context, msg *sarama.ProducerMessage) error {
	result := make(chan deliveryResult, 1)
	msg.Metadata = &deliveryMetadata{result: result}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.closed:
		return fmt.Errorf("Kafka producer is closed")
	case p.producer.Input() <- msg:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-p.closed:
		return fmt.Errorf("Kafka producer was closed before acknowledgement")
	case result := <-result:
		return result.err
	case <-timer.C:
		return fmt.Errorf("timeout waiting for Kafka acknowledgement")
	}
}

// SendTaskWithKey sends task with specific partition key
func (p *Producer) SendTaskWithKey(ctx context.Context, task *domain.Task, key string) error {
	topic := p.getTopicByPriority(task.Priority)

	msg, err := p.prepareTaskMessage(task, topic)
	if err != nil {
		return fmt.Errorf("failed to prepare message: %w", err)
	}

	// Override key
	msg.Key = sarama.StringEncoder(key)

	return p.SendMessage(ctx, msg)
}

func (p *Producer) SendBatch(ctx context.Context, tasks []*domain.Task) error {
	for _, task := range tasks {
		if err := p.SendTask(ctx, task); err != nil {
			return fmt.Errorf("failed to send task %s: %w", task.ID, err)
		}
	}
	return nil
}

func (p *Producer) prepareTaskMessage(task *domain.Task, topic string) (*sarama.ProducerMessage, error) {
	payload := messages.TaskMessage{
		ID:          task.ID,
		Type:        string(task.Type),
		Priority:    int(task.Priority),
		Payload:     task.Payload,
		Retries:     task.Retries,
		MaxRetries:  task.MaxRetries,
		TraceID:     task.TraceID,
		CreatedAt:   task.CreatedAt,
		ScheduledAt: task.ScheduledAt,
	}

	value, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal task: %w", err)
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(task.ID),
		Value: sarama.ByteEncoder(value),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("task_id"),
				Value: []byte(task.ID),
			},
			{
				Key:   []byte("task_type"),
				Value: []byte(task.Type),
			},
			{
				Key:   []byte("trace_id"),
				Value: []byte(task.TraceID),
			},
			{
				Key:   []byte("timestamp"),
				Value: []byte(fmt.Sprintf("%d", time.Now().Unix())),
			},
		},
		Timestamp: time.Now(),
	}
	// Add correlation ID if not present
	if task.TraceID == "" {
		msg.Headers = append(msg.Headers, sarama.RecordHeader{
			Key:   []byte("correlation_id"),
			Value: []byte(uuid.New().String()),
		})
	}

	return msg, nil
}

func (p *Producer) getTopicByPriority(priority domain.Priority) string {
	switch priority {
	case domain.PriorityCritical, domain.PriorityHigh:
		return p.config.Topics.TasksHigh
	case domain.PriorityNormal:
		return p.config.Topics.TasksNormal
	case domain.PriorityLow:
		return p.config.Topics.TasksLow
	default:
		return p.config.Topics.TasksNormal
	}
}

func (p *Producer) handleSuccesses() {
	defer p.wg.Done()

	for msg := range p.producer.Successes() {
		if msg == nil {
			continue
		}

		p.mu.Lock()
		p.messagesSent++
		p.mu.Unlock()
		notifyDelivery(msg, nil)

		p.logger.Debug().
			Str("topic", msg.Topic).
			Int32("partition", msg.Partition).
			Int64("offset", msg.Offset).
			Msg("message sent successfully")
	}
}

func (p *Producer) handleErrors() {
	defer p.wg.Done()

	for producerErr := range p.producer.Errors() {
		if producerErr == nil {
			continue
		}

		p.mu.Lock()
		p.messagesErrors++
		p.mu.Unlock()
		notifyDelivery(producerErr.Msg, producerErr.Err)

		p.logger.Error().
			Err(producerErr.Err).
			Str("topic", producerErr.Msg.Topic).
			Msg("failed to send message")
	}
}

func notifyDelivery(msg *sarama.ProducerMessage, err error) {
	if msg == nil {
		return
	}

	metadata, ok := msg.Metadata.(*deliveryMetadata)
	if !ok || metadata == nil {
		return
	}

	select {
	case metadata.result <- deliveryResult{err: err}:
	default:
	}
}

// GetStats returns producer statistics
func (p *Producer) GetStats() ProducerStats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return ProducerStats{
		MessagesSent:   p.messagesSent,
		MessagesErrors: p.messagesErrors,
	}
}

// Close closes the producer
func (p *Producer) Close() error {
	err := p.producer.Close()
	p.wg.Wait()
	close(p.closed)
	if err != nil {
		return fmt.Errorf("failed to close producer: %w", err)
	}

	p.logger.Info().Msg("Kafka producer closed")
	return nil
}

// ProducerStats contains producer statistics
type ProducerStats struct {
	MessagesSent   int64 `json:"messages_sent"`
	MessagesErrors int64 `json:"messages_errors"`
}
