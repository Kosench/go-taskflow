package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
)

// Consumer represents Kafka consumer
type Consumer struct {
	consumerGroup sarama.ConsumerGroup
	config        *config.KafkaConfig
	logger        *logger.Logger
	handler       MessageHandler

	// Topics to consume
	topics []string

	// Metrics
	messagesReceived int64
	messagesErrors   int64
	mu               sync.RWMutex

	// Control
	ready   chan bool
	closing chan struct{}
	closed  chan struct{}
}

// MessageHandler is the interface for handling messages
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *TaskMessage) error
}

// MessageHandlerFunc is a function adapter for MessageHandler
type MessageHandlerFunc func(ctx context.Context, msg *TaskMessage) error

func (f MessageHandlerFunc) HandleMessage(ctx context.Context, msg *TaskMessage) error {
	return f(ctx, msg)
}

// NewConsumer creates new Kafka consumer
func NewConsumer(cfg *config.KafkaConfig, handler MessageHandler, log *logger.Logger) (*Consumer, error) {
	saramaConfig := BuildConsumerConfig(&cfg.Consumer)

	// Create consumer group
	consumerGroup, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.Consumer.GroupID, saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	// Determine topics to consume based on configuration
	topics := []string{
		cfg.Topics.TasksHigh,
		cfg.Topics.TasksNormal,
		cfg.Topics.TasksLow,
		cfg.Topics.TasksRetry,
	}

	c := &Consumer{
		consumerGroup: consumerGroup,
		config:        cfg,
		logger:        log,
		handler:       handler,
		topics:        topics,
		ready:         make(chan bool),
		closing:       make(chan struct{}),
		closed:        make(chan struct{}),
	}

	log.Info().
		Strs("brokers", cfg.Brokers).
		Str("group", cfg.Consumer.GroupID).
		Strs("topics", topics).
		Msg("Kafka consumer created")

	return c, nil
}

// Start starts consuming messages
func (c *Consumer) Start(ctx context.Context) error {
	consumerHandler := &consumerGroupHandler{
		consumer: c,
		ready:    c.ready,
	}

	go func() {
		for {
			// Check if context was cancelled
			if ctx.Err() != nil {
				return
			}

			// Start consuming
			c.logger.Info().Msg("starting consumer group session")

			err := c.consumerGroup.Consume(ctx, c.topics, consumerHandler)
			if err != nil {
				c.logger.Error().Err(err).Msg("error from consumer")
				time.Sleep(5 * time.Second) // Backoff before retry
			}

			// Check if consumer was closed
			select {
			case <-c.closing:
				return
			default:
				c.logger.Info().Msg("rebalancing consumer group")
			}
		}
	}()

	// Wait for consumer to be ready
	<-c.ready
	c.logger.Info().Msg("consumer started and ready")

	return nil
}

// Stop stops the consumer
func (c *Consumer) Stop() error {
	c.logger.Info().Msg("stopping consumer")

	close(c.closing)

	if err := c.consumerGroup.Close(); err != nil {
		return fmt.Errorf("failed to close consumer group: %w", err)
	}

	close(c.closed)
	c.logger.Info().Msg("consumer stopped")

	return nil
}

// GetStats returns consumer statistics
func (c *Consumer) GetStats() ConsumerStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return ConsumerStats{
		MessagesReceived: c.messagesReceived,
		MessagesErrors:   c.messagesErrors,
	}
}

// consumerGroupHandler implements sarama.ConsumerGroupHandler
type consumerGroupHandler struct {
	consumer *Consumer
	ready    chan bool
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *consumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	h.consumer.logger.Debug().Msg("consumer group session setup")
	close(h.ready)
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *consumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	h.consumer.logger.Debug().Msg("consumer group session cleanup")
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages()
func (h *consumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	// Process each message
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			h.consumer.mu.Lock()
			h.consumer.messagesReceived++
			h.consumer.mu.Unlock()

			// Log message receipt
			h.consumer.logger.Debug().
				Str("topic", message.Topic).
				Int32("partition", message.Partition).
				Int64("offset", message.Offset).
				Int("size", len(message.Value)).
				Msg("received message")

			// Process message
			if err := h.processMessage(session.Context(), message); err != nil {
				h.consumer.mu.Lock()
				h.consumer.messagesErrors++
				h.consumer.mu.Unlock()

				h.consumer.logger.Error().
					Err(err).
					Str("topic", message.Topic).
					Int64("offset", message.Offset).
					Msg("failed to process message")

				// Continue processing other messages even if one fails
				// In production, you might want to send to DLQ or retry
			}

			// Mark message as processed
			session.MarkMessage(message, "")

		case <-h.consumer.closing:
			return nil
		}
	}
}

// processMessage processes a single message
func (h *consumerGroupHandler) processMessage(ctx context.Context, message *sarama.ConsumerMessage) error {
	// Parse message
	var taskMsg TaskMessage
	if err := json.Unmarshal(message.Value, &taskMsg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Add metadata from Kafka to context
	headers := ParseHeaders(message.Headers)
	ctx = context.WithValue(ctx, "kafka_topic", message.Topic)
	ctx = context.WithValue(ctx, "kafka_partition", message.Partition)
	ctx = context.WithValue(ctx, "kafka_offset", message.Offset)

	// Add trace ID to context if present
	if headers.TraceID != "" {
		ctx = logger.WithCorrelationID(ctx, headers.TraceID)
	}

	// Log processing start
	log := logger.FromContext(ctx)
	log.Info().
		Str("task_id", taskMsg.ID).
		Str("task_type", taskMsg.Type).
		Msg("processing task")

	// Call handler
	start := time.Now()
	err := h.consumer.handler.HandleMessage(ctx, &taskMsg)
	duration := time.Since(start)

	if err != nil {
		log.Error().
			Err(err).
			Str("task_id", taskMsg.ID).
			Dur("duration", duration).
			Msg("failed to handle message")
		return err
	}

	log.Info().
		Str("task_id", taskMsg.ID).
		Dur("duration", duration).
		Msg("task processed successfully")

	return nil
}

// ConsumerStats contains consumer statistics
type ConsumerStats struct {
	MessagesReceived int64 `json:"messages_received"`
	MessagesErrors   int64 `json:"messages_errors"`
}
