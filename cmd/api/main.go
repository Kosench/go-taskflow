package main

import (
	"context"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/rs/zerolog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	cfg := config.MustLoad(configPath)

	log := logger.InitFromConfig(&cfg.Log)

	log.Info().
		Str("app", cfg.App.Name).
		Str("version", cfg.App.Version).
		Str("environment", cfg.App.Environment).
		Bool("debug", cfg.App.Debug).
		Msg("starting application")

	// Example of using context with logger
	ctx := context.Background()
	ctx = logger.WithRequestID(ctx, "startup-011")
	ctx = logger.WithCorrelationID(ctx, "init-process")

	// Get logger from context
	ctxLogger := logger.FromContext(ctx)
	ctxLogger.Debug().Msg("context logger initialized")

	// Log configuration details
	log.Debug().
		Str("http_address", cfg.Server.HTTP.Address()).
		Str("grpc_address", cfg.Server.GRPC.Address()).
		Strs("kafka_brokers", cfg.Kafka.Brokers).
		Msg("configuration loaded")

	// Example of structured logging
	log.Info().
		Dict("server", zerolog.Dict().
			Str("http", cfg.Server.HTTP.Address()).
			Str("grpc", cfg.Server.GRPC.Address())).
		Dict("database", zerolog.Dict().
			Str("host", cfg.Database.Host).
			Int("port", cfg.Database.Port).
			Str("name", cfg.Database.Name)).
		Msg("service configuration")

	// Example of tracking function execution time
	defer logger.Measure(time.Now(), "main")()

	// Simulate some work
	log.Info().Msg("initializing services...")
	time.Sleep(100 * time.Millisecond)

	// Example of error logging
	if err := simulateWork(); err != nil {
		log.Error().
			Err(err).
			Str("component", "simulator").
			Msg("failed to simulate work")
	}

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Info().Msg("application started successfully")
	log.Info().Msg("press Ctrl+C to shutdown")

	// Wait for shutdown signal
	sig := <-sigChan
	log.Info().
		Str("signal", sig.String()).
		Msg("shutdown signal received")

	// Cleanup
	log.Info().Msg("shutting down gracefully...")
	time.Sleep(100 * time.Millisecond)

	log.Info().Msg("application stopped")
}

func simulateWork() error {
	logger.Debug("starting simulated work")
	time.Sleep(50 * time.Millisecond)
	logger.Debug("simulated work completed")
	return nil
}
