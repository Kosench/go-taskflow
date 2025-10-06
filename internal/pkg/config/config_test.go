package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Test loading default config
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("failed to load default config: %v", err)
	}

	// Check defaults
	if cfg.App.Name != "task-queue" {
		t.Errorf("expected app.name to be 'task-queue', got %s", cfg.App.Name)
	}

	if cfg.Worker.Concurrency != 5 {
		t.Errorf("expected worker.concurrency to be 5, got %d", cfg.Worker.Concurrency)
	}
}

func TestEnvironmentVariables(t *testing.T) {
	// Set environment variable
	os.Setenv("TASKQUEUE_APP_NAME", "test-app")
	os.Setenv("TASKQUEUE_WORKER_CONCURRENCY", "10")
	defer os.Unsetenv("TASKQUEUE_APP_NAME")
	defer os.Unsetenv("TASKQUEUE_WORKER_CONCURRENCY")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("failed to load config with env vars: %v", err)
	}

	// Check if env vars override defaults
	if cfg.App.Name != "test-app" {
		t.Errorf("expected app.name to be 'test-app', got %s", cfg.App.Name)
	}

	if cfg.Worker.Concurrency != 10 {
		t.Errorf("expected worker.concurrency to be 10, got %d", cfg.Worker.Concurrency)
	}
}

func TestDatabaseDSN(t *testing.T) {
	cfg := &DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "user",
		Password: "pass",
		Name:     "db",
		SSLMode:  "disable",
	}

	expected := "host=localhost port=5432 user=user password=pass dbname=db sslmode=disable"
	if dsn := cfg.DSN(); dsn != expected {
		t.Errorf("expected DSN %s, got %s", expected, dsn)
	}
}
