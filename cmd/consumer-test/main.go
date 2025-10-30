package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/transport/kafka"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

func main() {
	var (
		groupID = flag.String("group", "test-consumer", "Consumer group ID")
		batch   = flag.Bool("batch", false, "Use batch consumer")
	)
	flag.Parse()

	// Load config
	cfg := config.MustLoad("")

	// Override group ID if provided
	if *groupID != "" {
		cfg.Kafka.Consumer.GroupID = *groupID
	}

	// Initialize logger
	log := logger.InitFromConfig(&cfg.Log)

	// Message counter
	var messageCount int64

	// Create message handler
	handler := kafka.MessageHandlerFunc(func(ctx context.Context, msg *kafka.TaskMessage) error {
		count := atomic.AddInt64(&messageCount, 1)

		log.Info().
			Int64("count", count).
			Str("task_id", msg.ID).
			Str("type", msg.Type).
			Int("priority", msg.Priority).
			RawJSON("payload", msg.Payload).
			Msg("received task")

		// Simulate processing
		time.Sleep(100 * time.Millisecond)

		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var consumer interface {
		Start(context.Context) error
		Stop() error
		GetStats() kafka.ConsumerStats
	}

	if *batch {
		// Create batch consumer
		batchHandler := kafka.BatchMessageHandlerFunc(func(ctx context.Context, messages []*kafka.TaskMessage) error {
			log.Info().
				Int("batch_size", len(messages)).
				Msg("processing batch")

			for _, msg := range messages {
				atomic.AddInt64(&messageCount, 1)
				log.Debug().
					Str("task_id", msg.ID).
					Str("type", msg.Type).
					Msg("task in batch")
			}

			return nil
		})

		bc, err := kafka.NewBatchConsumer(&cfg.Kafka, 10, 5*time.Second, batchHandler, log)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create batch consumer")
		}
		consumer = bc

		log.Info().Msg("starting batch consumer")
	} else {
		// Create regular consumer
		c, err := kafka.NewConsumer(&cfg.Kafka, handler, log)
		if err != nil {
			log.Fatal().Err(err).Msg("failed to create consumer")
		}
		consumer = c

		log.Info().Msg("starting consumer")
	}

	// Start consumer
	if err := consumer.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start consumer")
	}

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Print stats periodically
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			stats := consumer.GetStats()
			log.Info().
				Int64("received", stats.MessagesReceived).
				Int64("errors", stats.MessagesErrors).
				Int64("processed", atomic.LoadInt64(&messageCount)).
				Msg("consumer stats")
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	log.Info().
		Str("signal", sig.String()).
		Msg("shutdown signal received")

	// Stop consumer
	if err := consumer.Stop(); err != nil {
		log.Error().Err(err).Msg("error stopping consumer")
	}

	// Print final stats
	stats := consumer.GetStats()
	fmt.Printf("\nFinal Statistics:\n")
	fmt.Printf("  Messages received: %d\n", stats.MessagesReceived)
	fmt.Printf("  Messages processed: %d\n", atomic.LoadInt64(&messageCount))
	fmt.Printf("  Errors: %d\n", stats.MessagesErrors)
}
