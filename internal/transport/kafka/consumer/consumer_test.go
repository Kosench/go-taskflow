package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/messages"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/producer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockMessageHandler for testing
type MockMessageHandler struct {
	mock.Mock
}

type fakeConsumerGroupSession struct {
	ctx       context.Context
	marked    int
	committed int
}

func (s *fakeConsumerGroupSession) Claims() map[string][]int32                  { return nil }
func (s *fakeConsumerGroupSession) MemberID() string                            { return "test-member" }
func (s *fakeConsumerGroupSession) GenerationID() int32                         { return 1 }
func (s *fakeConsumerGroupSession) MarkOffset(string, int32, int64, string)     {}
func (s *fakeConsumerGroupSession) Commit()                                     { s.committed++ }
func (s *fakeConsumerGroupSession) ResetOffset(string, int32, int64, string)    {}
func (s *fakeConsumerGroupSession) MarkMessage(*sarama.ConsumerMessage, string) { s.marked++ }
func (s *fakeConsumerGroupSession) Context() context.Context                    { return s.ctx }

type fakeConsumerGroupClaim struct {
	messages <-chan *sarama.ConsumerMessage
}

func (c *fakeConsumerGroupClaim) Topic() string                            { return "tasks-normal" }
func (c *fakeConsumerGroupClaim) Partition() int32                         { return 0 }
func (c *fakeConsumerGroupClaim) InitialOffset() int64                     { return 0 }
func (c *fakeConsumerGroupClaim) HighWaterMarkOffset() int64               { return 1 }
func (c *fakeConsumerGroupClaim) Messages() <-chan *sarama.ConsumerMessage { return c.messages }

func (m *MockMessageHandler) HandleMessage(ctx context.Context, msg *messages.TaskMessage) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func TestConsumerGroupHandler_processMessage(t *testing.T) {
	mockHandler := new(MockMessageHandler)
	log := logger.New(logger.Config{Level: "debug"})

	c := &Consumer{
		handler: mockHandler,
		logger:  log,
	}

	handler := &consumerGroupHandler{
		consumer: c,
	}

	// Create test message
	taskMsg := messages.TaskMessage{
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

	// JSON is normalized during marshal/unmarshal, so compare it semantically.
	mockHandler.On("HandleMessage", mock.Anything, mock.MatchedBy(func(actual *messages.TaskMessage) bool {
		if actual == nil || actual.ID != taskMsg.ID || actual.Type != taskMsg.Type || actual.Priority != taskMsg.Priority {
			return false
		}

		var expectedPayload, actualPayload any
		if err := json.Unmarshal(taskMsg.Payload, &expectedPayload); err != nil {
			return false
		}
		if err := json.Unmarshal(actual.Payload, &actualPayload); err != nil {
			return false
		}

		return assert.ObjectsAreEqual(expectedPayload, actualPayload)
	})).Return(nil)

	// Process message
	ctx := context.Background()
	err = handler.processMessage(ctx, kafkaMsg)

	assert.NoError(t, err)
	mockHandler.AssertExpectations(t)
}

func TestConsumerGroupHandler_Offsets(t *testing.T) {
	newClaim := func() sarama.ConsumerGroupClaim {
		messagesCh := make(chan *sarama.ConsumerMessage, 1)
		messagesCh <- &sarama.ConsumerMessage{
			Topic: "tasks-normal",
			Value: []byte(`{"id":"task-1","type":"send_email","priority":1,"payload":{}}`),
		}
		close(messagesCh)
		return &fakeConsumerGroupClaim{messages: messagesCh}
	}

	t.Run("successful message is marked and committed", func(t *testing.T) {
		session := &fakeConsumerGroupSession{ctx: context.Background()}
		consumer := &Consumer{
			config:  &config.KafkaConfig{},
			logger:  logger.New(logger.Config{Level: "disabled"}),
			handler: MessageHandlerFunc(func(context.Context, *messages.TaskMessage) error { return nil }),
			closing: make(chan struct{}),
		}

		err := (&consumerGroupHandler{consumer: consumer}).ConsumeClaim(session, newClaim())

		require.NoError(t, err)
		assert.Equal(t, 1, session.marked)
		assert.Equal(t, 1, session.committed)
	})

	t.Run("failed message does not advance offset", func(t *testing.T) {
		handlerErr := errors.New("processing failed")
		session := &fakeConsumerGroupSession{ctx: context.Background()}
		consumer := &Consumer{
			config:  &config.KafkaConfig{},
			logger:  logger.New(logger.Config{Level: "disabled"}),
			handler: MessageHandlerFunc(func(context.Context, *messages.TaskMessage) error { return handlerErr }),
			closing: make(chan struct{}),
		}

		err := (&consumerGroupHandler{consumer: consumer}).ConsumeClaim(session, newClaim())

		assert.ErrorIs(t, err, handlerErr)
		assert.Zero(t, session.marked)
		assert.Zero(t, session.committed)
	})
}

func TestBatchConsumer_HandleMessage(t *testing.T) {
	processedBatches := make(chan []*messages.TaskMessage, 10)

	handler := BatchMessageHandlerFunc(func(ctx context.Context, msgs []*messages.TaskMessage) error {
		processedBatches <- msgs
		return nil
	})

	log := logger.New(logger.Config{Level: "debug"})

	bc := &BatchConsumer{
		batchSize: 3,
		timeout:   100 * time.Millisecond,
		handler:   handler,
		logger:    log,
		batch:     make([]*messages.TaskMessage, 0, 3),
		ticker:    time.NewTicker(100 * time.Millisecond),
		done:      make(chan struct{}),
	}

	ctx := context.Background()

	// Add messages to batch
	for i := range 3 {
		msg := &messages.TaskMessage{
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
	processedBatches := make(chan []*messages.TaskMessage, 10)

	handler := BatchMessageHandlerFunc(func(ctx context.Context, msgs []*messages.TaskMessage) error {
		processedBatches <- msgs
		return nil
	})

	log := logger.New(logger.Config{Level: "debug"})

	bc := &BatchConsumer{
		batchSize: 10, // Large batch size
		timeout:   200 * time.Millisecond,
		handler:   handler,
		logger:    log,
		batch:     make([]*messages.TaskMessage, 0, 10),
		ticker:    time.NewTicker(200 * time.Millisecond),
		done:      make(chan struct{}),
	}

	// Start batch processor
	go bc.processBatches()
	defer close(bc.done)

	ctx := context.Background()

	// Add only 2 messages (less than batch size)
	for i := range 2 {
		msg := &messages.TaskMessage{
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

	receivedMessages := make(chan *messages.TaskMessage, 10)

	handler := MessageHandlerFunc(func(ctx context.Context, msg *messages.TaskMessage) error {
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

	c, err := NewConsumer(cfg, handler, log)
	require.NoError(t, err)

	ctx := t.Context()

	// Start consumer
	err = c.Start(ctx)
	require.NoError(t, err)

	// Create producer to send test message
	producer, err := producer.NewProducer(cfg, log)
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
	err = c.Stop()
	assert.NoError(t, err)
}
