package consumer

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
)

type GracefulConsumer struct {
	consumer        *Consumer
	shutdownTimeout time.Duration
	logger          *logger.Logger

	shutdownChan chan os.Signal
	shutdownOnce sync.Once
	wg           sync.WaitGroup
}

func NewGracefulConsumer(
	cfg *config.KafkaConfig,
	handler MessageHandler,
	shutdownTimeout time.Duration,
	log *logger.Logger,
) (*GracefulConsumer, error) {

	consumer, err := NewConsumer(cfg, handler, log)
	if err != nil {
		return nil, err
	}

	gc := &GracefulConsumer{
		consumer:        consumer,
		shutdownTimeout: shutdownTimeout,
		logger:          log,
		shutdownChan:    make(chan os.Signal, 1),
	}

	// Register signal handlers
	signal.Notify(gc.shutdownChan, syscall.SIGINT, syscall.SIGTERM)

	return gc, nil
}

func (gc *GracefulConsumer) Run(ctx context.Context) error {
	if err := gc.consumer.Start(ctx); err != nil {
		return err
	}

	gc.logger.Info().Msg("graceful consumer started")

	select {
	case sig := <-gc.shutdownChan:
		gc.logger.Info().Str("signal", sig.String()).Msg("shutdown signal received")
	case <-ctx.Done():
		gc.logger.Info().Msg("context cancelled")
	}

	return gc.Shutdown()
}

func (gc *GracefulConsumer) Shutdown() error {
	var err error

	gc.shutdownOnce.Do(func() {
		gc.logger.Info().
			Dur("timeout", gc.shutdownTimeout).
			Msg("starting graceful shutdown")

		// Create shutdown context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), gc.shutdownTimeout)
		defer cancel()

		// Stop accepting new messages
		stopChan := make(chan error, 1)
		go func() {
			stopChan <- gc.consumer.Stop()
		}()

		// Wait for shutdown or timeout
		select {
		case err = <-stopChan:
			if err != nil {
				gc.logger.Error().
					Err(err).
					Msg("error during shutdown")
			} else {
				gc.logger.Info().Msg("graceful shutdown completed")
			}

		case <-ctx.Done():
			gc.logger.Warn().
				Dur("timeout", gc.shutdownTimeout).
				Msg("shutdown timeout exceeded, forcing shutdown")
			err = ctx.Err()
		}
	})

	return err
}

// GetStats returns consumer statistics
func (gc *GracefulConsumer) GetStats() ConsumerStats {
	return gc.consumer.GetStats()
}
