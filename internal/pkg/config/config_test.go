package config

import (
	"os"
	"path/filepath"
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
	os.Setenv("TASKQUEUE_KAFKA_BROKERS", "broker-1:9092,broker-2:9092")
	defer os.Unsetenv("TASKQUEUE_APP_NAME")
	defer os.Unsetenv("TASKQUEUE_WORKER_CONCURRENCY")
	defer os.Unsetenv("TASKQUEUE_KAFKA_BROKERS")

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

	if len(cfg.Kafka.Brokers) != 2 || cfg.Kafka.Brokers[0] != "broker-1:9092" || cfg.Kafka.Brokers[1] != "broker-2:9092" {
		t.Errorf("expected comma-separated Kafka brokers, got %#v", cfg.Kafka.Brokers)
	}
}

func TestLoadMergesBaseAndEnvironmentConfig(t *testing.T) {
	workingDir := t.TempDir()
	configDir := filepath.Join(workingDir, "configs")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	requireWriteFile(t, filepath.Join(configDir, "config.yaml"), []byte("app:\n  name: merged-app\nworker:\n  concurrency: 3\n"))
	requireWriteFile(t, filepath.Join(configDir, "config.development.yaml"), []byte("app:\n  debug: true\nworker:\n  concurrency: 7\n"))

	previousDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDir) })

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("failed to load merged config: %v", err)
	}

	if cfg.App.Name != "merged-app" {
		t.Errorf("expected base app name, got %q", cfg.App.Name)
	}
	if !cfg.App.Debug {
		t.Error("expected environment config to enable debug")
	}
	if cfg.Worker.Concurrency != 7 {
		t.Errorf("expected environment config to override concurrency, got %d", cfg.Worker.Concurrency)
	}
}

func TestExplicitConfigIsNotReplacedByEnvironmentConfig(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "custom.yaml")
	requireWriteFile(t, configPath, []byte("app:\n  name: explicit-app\nworker:\n  concurrency: 2\n"))
	t.Setenv("APP_ENV", "development")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("failed to load explicit config: %v", err)
	}

	if cfg.App.Name != "explicit-app" {
		t.Errorf("expected explicit app name, got %q", cfg.App.Name)
	}
	if cfg.Worker.Concurrency != 2 {
		t.Errorf("expected explicit concurrency, got %d", cfg.Worker.Concurrency)
	}
}

func requireWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
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
