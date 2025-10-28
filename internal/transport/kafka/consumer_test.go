package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

// MockMessageHandler for testing
type MockMessageHandler struct {
	mock.Mock
}

func (m *MockMessageHandler) HandleMessage(ctx context.Context, msg *TaskMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func TestConsumerGroupHandler_processMessage(t *testing.T) {
	mockHandler := new(MockMessageHandler)
	log := logger.New(logger.Config{Level: "debug"})

	consumer := &Consumer{
		handler: mockHandler,
		logger:  log,
	}

	handler := &consumerGroupHandler{
		consumer: consumer,
	}

	// Create test message
	taskMsg := TaskMessage{
		ID:       "test-123",
		Type:     "test_task",
		Priority: 1,
		Payload:  json.RawMessage(`{"test": true}`),
	}

	msgData, err := json.Marshal(taskMsg)
	require.NoError(t, err)

	kafkaMsg := &sarama.ConsumerMessage{
		Topic:     "test-topic",
		Partition: 0,
		Offset:    100,
		Value:     msgData,
		Headers: []*sarama.RecordHeader{
			{Key: []byte("task_id"), Value: []byte("test-123")},
		},
	}

	// Set expectation
	mockHandler.On("HandleMessage", mock.Anything, &taskMsg).Return(nil)

	// Process message
	ctx := context.Background()
	err = handler.processMessage(ctx, kafkaMsg)

	assert.NoError(t, err)
	mockHandler.AssertExpectations(t)
}

func TestBatchConsumer_HandleMessage(t *testing.T) {
	processedBatches := make(chan []*TaskMessage, 10)

	handler := BatchMessageHandlerFunc(func(ctx context.Context, messages []*TaskMessage) error {
		processedBatches <- messages
		return nil
	})

	log := logger.New(logger.Config{Level: "debug"})

	bc := &BatchConsumer{
		batchSize: 3,
		timeout:   100 * time.Millisecond,
		handler:   handler,
		logger:    log,
		batch:     make([]*TaskMessage, 0, 3),
		ticker:    time.NewTicker(100 * time.Millisecond),
		done:      make(chan struct{}),
	}

	ctx := context.Background()

	// Add messages to batch
	for i := 0; i < 3; i++ {
		msg := &TaskMessage{
			ID:   fmt.Sprintf("test-%d", i),
			Type: "test",
		}
		err := bc.handleMessage(ctx, msg)
		assert.NoError(t, err)
	}

	// Should trigger batch processing
	select {
	case batch := <-processedBatches:
		assert.Len(t, batch, 3)
		assert.Equal(t, "test-0", batch[0].ID)
	case <-time.After(1 * time.Second):
		t.Fatal("batch not processed")
	}
}

func TestBatchConsumer_Timeout(t *testing.T) {
	processedBatches := make(chan []*TaskMessage, 10)

	handler := BatchMessageHandlerFunc(func(ctx context.Context, messages []*TaskMessage) error {
		processedBatches <- messages
		return nil
	})

	log := logger.New(logger.Config{Level: "debug"})

	bc := &BatchConsumer{
		batchSize: 10, // Large batch size
		timeout:   200 * time.Millisecond,
		handler:   handler,
		logger:    log,
		batch:     make([]*TaskMessage, 0, 10),
		ticker:    time.NewTicker(200 * time.Millisecond),
		done:      make(chan struct{}),
	}

	// Start batch processor
	go bc.processBatches()
	defer close(bc.done)

	ctx := context.Background()

	// Add only 2 messages (less than batch size)
	for i := 0; i < 2; i++ {
		msg := &TaskMessage{
			ID:   fmt.Sprintf("timeout-%d", i),
			Type: "test",
		}
		err := bc.handleMessage(ctx, msg)
		assert.NoError(t, err)
	}

	// Should process on timeout
	select {
	case batch := <-processedBatches:
		assert.Len(t, batch, 2)
		assert.Equal(t, "timeout-0", batch[0].ID)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("batch not processed on timeout")
	}
}

// Integration test
func TestConsumer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	receivedMessages := make(chan *TaskMessage, 10)

	handler := MessageHandlerFunc(func(ctx context.Context, msg *TaskMessage) error {
		receivedMessages <- msg
		return nil
	})

	cfg := &config.KafkaConfig{
		Brokers: []string{"localhost:9092"},
		Topics: config.TopicsConfig{
			TasksHigh:   "tasks-high",
			TasksNormal: "tasks-normal",
			TasksLow:    "tasks-low",
			TasksRetry:  "tasks-retry",
		},
		Consumer: config.ConsumerConfig{
			GroupID:           "test-consumer-group",
			AutoOffsetReset:   "earliest",
			EnableAutoCommit:  false,
			SessionTimeout:    20 * time.Second,
			HeartbeatInterval: 6 * time.Second,
			IsolationLevel:    "read_committed",
		},
	}

	log := logger.New(logger.Config{Level: "debug"})

	consumer, err := NewConsumer(cfg, handler, log)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Start consumer
	err = consumer.Start(ctx)
	require.NoError(t, err)

	// Create producer to send test message
	producer, err := NewProducer(cfg, log)
	require.NoError(t, err)
	defer producer.Close()

	// Wait for message to be consumed
	select {
	case received := <-receivedMessages:
		assert.Equal(t, "integration-consumer-test", received.ID)
	case <-time.After(5 * time.Second):
		t.Fatal("message not received")
	}

	// Stop consumer
	err = consumer.Stop()
	assert.NoError(t, err)
}
