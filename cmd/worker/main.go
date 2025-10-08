package main

import (
	"context"
	"fmt"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Load configuration
	cfg := config.MustLoad("")

	// Initialize logger
	log := logger.InitFromConfig(&cfg.Log)

	// Worker-specific logger
	log = log.WithField("component", "worker")

	log.Info().
		Str("app", cfg.App.Name).
		Str("version", cfg.App.Version).
		Int("concurrency", cfg.Worker.Concurrency).
		Msg("starting worker")

	ctx := context.Background()

	// Simulate worker processing
	for i := 1; i <= 3; i++ {
		taskID := fmt.Sprintf("task-%d", i)
		processTask(ctx, taskID)
	}

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info().Msg("worker started, waiting for tasks...")

	<-sigChan
	log.Info().Msg("shutting down worker...")
}

func processTask(ctx context.Context, taskID string) {
	start := time.Now()

	// Add task ID to context
	ctx = logger.WithFields(ctx, map[string]interface{}{
		"task_id":   taskID,
		"task_type": "example",
	})

	log := logger.FromContext(ctx)

	log.Info().Msg("processing task")

	// Simulate processing
	time.Sleep(100 * time.Millisecond)

	// Log completion
	logger.LogTaskProcessing(taskID, "example", "completed", time.Since(start), nil)
}
