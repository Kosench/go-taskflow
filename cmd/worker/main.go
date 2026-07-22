package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/database"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	postgresrepo "github.com/Kosench/go-taskflow/internal/repository/postgres"
	"github.com/Kosench/go-taskflow/internal/service"
	"github.com/Kosench/go-taskflow/internal/transport/kafka"
	workerpkg "github.com/Kosench/go-taskflow/internal/worker"
)

func main() {
	cfg := config.MustLoad(os.Getenv("CONFIG_PATH"))
	log := logger.InitFromConfig(&cfg.Log).WithField("component", "worker")

	db, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	producer, err := kafka.NewTaskProducer(&cfg.Kafka, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create Kafka producer")
	}
	defer producer.Close()

	repo := postgresrepo.NewTaskRepository(db)
	taskService := service.NewTaskService(repo, producer, log)
	workerID := buildWorkerID()

	processor, err := workerpkg.NewProcessor(workerID, taskService, workerpkg.TaskHandlerFunc(handleTask), log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create task processor")
	}

	consumer, err := kafka.NewTaskConsumer(&cfg.Kafka, kafka.TaskHandlerFunc(func(ctx context.Context, task *domain.Task) error {
		return processor.HandleTask(ctx, task)
	}), log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create Kafka consumer")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := consumer.Start(ctx); err != nil {
		log.Fatal().Err(err).Msg("failed to start Kafka consumer")
	}

	poller := workerpkg.NewPoller(repo, processor, cfg.Worker.RetryBackoff, cfg.Worker.BatchSize, cfg.Worker.Concurrency, log)
	go poller.Run(ctx)

	log.Info().
		Str("worker_id", workerID).
		Int("concurrency", cfg.Worker.Concurrency).
		Msg("worker started")

	<-ctx.Done()
	log.Info().Msg("worker shutdown requested")
	if err := consumer.Stop(); err != nil {
		log.Error().Err(err).Msg("failed to stop Kafka consumer")
	}
}

// handleTask is the default infrastructure handler. It validates the JSON
// payload and returns a durable processing receipt. Task-specific integrations
// can replace this handler without changing the worker lifecycle.
func handleTask(_ context.Context, task *domain.Task) ([]byte, error) {
	if !json.Valid(task.Payload) {
		return nil, fmt.Errorf("task payload is not valid JSON")
	}

	return json.Marshal(map[string]any{
		"task_id":      task.ID,
		"type":         task.Type,
		"processed_at": time.Now().UTC(),
	})
}

func buildWorkerID() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "worker"
	}
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}
