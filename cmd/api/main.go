package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/database"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	postgresrepo "github.com/Kosench/go-taskflow/internal/repository/postgres"
	"github.com/Kosench/go-taskflow/internal/service"
	"github.com/Kosench/go-taskflow/internal/transport/httpapi"
	"github.com/Kosench/go-taskflow/internal/transport/kafka"
)

func main() {
	cfg := config.MustLoad(os.Getenv("CONFIG_PATH"))
	log := logger.InitFromConfig(&cfg.Log).WithField("component", "api")

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
	server := httpapi.NewServer(&cfg.Server.HTTP, taskService, db, log)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	serverErrors := make(chan error, 1)
	go func() { serverErrors <- server.Run() }()

	select {
	case err := <-serverErrors:
		if err != nil {
			log.Fatal().Err(err).Msg("HTTP API stopped unexpectedly")
		}
	case <-ctx.Done():
		log.Info().Msg("HTTP API shutdown requested")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("failed to shutdown HTTP API gracefully")
	}
}
