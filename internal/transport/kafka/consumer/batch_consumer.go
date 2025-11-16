package consumer

import (
	"context"
	"sync"
	"time"

	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/messages"
)

type BatchConsumer struct {
	consumer  *Consumer
	batchSize int
	timeout   time.Duration
	handler   BatchMessageHandler
	logger    *logger.Logger

	// Batch buffer
	batch      []*messages.TaskMessage
	batchMutex sync.Mutex

	// Control
	ticker *time.Ticker
	done   chan struct{}
}

// BatchMessageHandler handles batches of messages
type BatchMessageHandler interface {
	HandleBatch(ctx context.Context, messages []*messages.TaskMessage) error
}

// BatchMessageHandlerFunc is a function adapter for BatchMessageHandler
type BatchMessageHandlerFunc func(ctx context.Context, messages []*messages.TaskMessage) error

func (f BatchMessageHandlerFunc) HandleBatch(ctx context.Context, messages []*messages.TaskMessage) error {
	return f(ctx, messages)
}

// NewBatchConsumer creates a new batch consumer
func NewBatchConsumer(
	cfg *config.KafkaConfig,
	batchSize int,
	timeout time.Duration,
	handler BatchMessageHandler,
	log *logger.Logger,
) (*BatchConsumer, error) {

	bc := &BatchConsumer{
		batchSize: batchSize,
		timeout:   timeout,
		handler:   handler,
		logger:    log,
		batch:     make([]*messages.TaskMessage, 0, batchSize),
		ticker:    time.NewTicker(timeout),
		done:      make(chan struct{}),
	}

	// Create underlying consumer with batch message handler
	consumer, err := NewConsumer(cfg, MessageHandlerFunc(bc.handleMessage), log)
	if err != nil {
		return nil, err
	}

	bc.consumer = consumer

	// Start batch processor
	go bc.processBatches()

	return bc, nil
}

// Start starts the batch consumer
func (bc *BatchConsumer) Start(ctx context.Context) error {
	return bc.consumer.Start(ctx)
}

// Stop stops the batch consumer
func (bc *BatchConsumer) Stop() error {
	// Stop ticker
	bc.ticker.Stop()
	close(bc.done)

	// Process remaining batch
	bc.batchMutex.Lock()
	if len(bc.batch) > 0 {
		ctx := context.Background()
		if err := bc.handler.HandleBatch(ctx, bc.batch); err != nil {
			bc.logger.Error().
				Err(err).
				Int("batch_size", len(bc.batch)).
				Msg("failed to process final batch")
		}
		bc.batch = bc.batch[:0]
	}
	bc.batchMutex.Unlock()

	// Stop underlying consumer
	return bc.consumer.Stop()
}

// handleMessage adds message to batch
func (bc *BatchConsumer) handleMessage(ctx context.Context, msg *messages.TaskMessage) error {
	bc.batchMutex.Lock()
	defer bc.batchMutex.Unlock()

	bc.batch = append(bc.batch, msg)

	// Process batch if it's full
	if len(bc.batch) >= bc.batchSize {
		if err := bc.processBatchLocked(ctx); err != nil {
			return err
		}
	}

	return nil
}

// processBatches processes batches on timeout
func (bc *BatchConsumer) processBatches() {
	for {
		select {
		case <-bc.ticker.C:
			bc.batchMutex.Lock()
			if len(bc.batch) > 0 {
				ctx := context.Background()
				if err := bc.processBatchLocked(ctx); err != nil {
					bc.logger.Error().
						Err(err).
						Int("batch_size", len(bc.batch)).
						Msg("failed to process batch on timeout")
				}
			}
			bc.batchMutex.Unlock()

		case <-bc.done:
			return
		}
	}
}

// processBatchLocked processes current batch (must be called with lock held)
func (bc *BatchConsumer) processBatchLocked(ctx context.Context) error {
	if len(bc.batch) == 0 {
		return nil
	}

	// Copy batch for processing
	batchCopy := make([]*messages.TaskMessage, len(bc.batch))
	copy(batchCopy, bc.batch)

	// Clear batch
	bc.batch = bc.batch[:0]

	// Process batch
	bc.logger.Debug().
		Int("batch_size", len(batchCopy)).
		Msg("processing batch")

	start := time.Now()
	err := bc.handler.HandleBatch(ctx, batchCopy)
	duration := time.Since(start)

	if err != nil {
		bc.logger.Error().
			Err(err).
			Int("batch_size", len(batchCopy)).
			Dur("duration", duration).
			Msg("failed to process batch")
		return err
	}

	bc.logger.Info().
		Int("batch_size", len(batchCopy)).
		Dur("duration", duration).
		Msg("batch processed successfully")

	return nil
}

// GetStats returns consumer statistics
func (bc *BatchConsumer) GetStats() ConsumerStats {
	return bc.consumer.GetStats()
}
