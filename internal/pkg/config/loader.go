package config

import (
	"fmt"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
)

func Load(configPath string) (*Config, error) {
	v := viper.New()

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("config")
		v.SetConfigType("yaml")
		v.AddConfigPath("./configs")
		v.AddConfigPath(".")
	}
	// Environment-specific config
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	// Try to load environment-specific config
	envConfigPath := filepath.Join("configs", fmt.Sprintf("config.%s.yaml", env))
	if _, err := os.Stat(envConfigPath); err == nil {
		v.SetConfigFile(envConfigPath)
	}

	// Environment variables
	v.SetEnvPrefix("TASKQUEUE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	// Set defaults
	setDefaults(v)

	// Read config file
	if err := v.ReadInConfig(); err != nil {
		// It's okay if config file doesn't exist, we have defaults and env vars
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate config
	if err := validate(&config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// setDefaults sets default values
func setDefaults(v *viper.Viper) {
	// App defaults
	v.SetDefault("app.name", "task-queue")
	v.SetDefault("app.version", "0.1.0")
	v.SetDefault("app.environment", "development")
	v.SetDefault("app.debug", false)

	// Server defaults
	v.SetDefault("server.http.host", "0.0.0.0")
	v.SetDefault("server.http.port", 8080)
	v.SetDefault("server.http.read_timeout", "15s")
	v.SetDefault("server.http.write_timeout", "15s")
	v.SetDefault("server.http.shutdown_timeout", "30s")

	v.SetDefault("server.grpc.host", "0.0.0.0")
	v.SetDefault("server.grpc.port", 50051)
	v.SetDefault("server.grpc.max_connection_age", "10m")
	v.SetDefault("server.grpc.max_connection_idle", "5m")

	// Database defaults
	v.SetDefault("database.host", "localhost")
	v.SetDefault("database.port", 5432)
	v.SetDefault("database.name", "taskqueue")
	v.SetDefault("database.user", "taskqueue")
	v.SetDefault("database.password", "taskqueue")
	v.SetDefault("database.ssl_mode", "disable")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "5m")
	v.SetDefault("database.conn_max_idle_time", "5m")

	// Kafka defaults
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.topics.tasks_high", "tasks-high")
	v.SetDefault("kafka.topics.tasks_normal", "tasks-normal")
	v.SetDefault("kafka.topics.tasks_low", "tasks-low")
	v.SetDefault("kafka.topics.tasks_retry", "tasks-retry")
	v.SetDefault("kafka.topics.tasks_dlq", "tasks-dlq")
	v.SetDefault("kafka.topics.task_results", "task-results")

	v.SetDefault("kafka.producer.required_acks", -1) // WaitForAll
	v.SetDefault("kafka.producer.compression", "snappy")
	v.SetDefault("kafka.producer.max_message_bytes", 1000000)
	v.SetDefault("kafka.producer.flush_frequency", "100ms")
	v.SetDefault("kafka.producer.idempotent", true)
	v.SetDefault("kafka.producer.retry_max", 5)
	v.SetDefault("kafka.producer.retry_backoff", "100ms")

	v.SetDefault("kafka.consumer.group_id", "task-workers")
	v.SetDefault("kafka.consumer.auto_offset_reset", "earliest")
	v.SetDefault("kafka.consumer.enable_auto_commit", false)
	v.SetDefault("kafka.consumer.session_timeout", "20s")
	v.SetDefault("kafka.consumer.heartbeat_interval", "6s")
	v.SetDefault("kafka.consumer.max_poll_records", 100)
	v.SetDefault("kafka.consumer.isolation_level", "read_committed")

	// Worker defaults
	v.SetDefault("worker.concurrency", 5)
	v.SetDefault("worker.max_retries", 3)
	v.SetDefault("worker.retry_backoff", "5s")
	v.SetDefault("worker.shutdown_timeout", "30s")
	v.SetDefault("worker.process_timeout", "5m")
	v.SetDefault("worker.batch_size", 10)

	// Metrics defaults
	v.SetDefault("metrics.enabled", true)
	v.SetDefault("metrics.host", "0.0.0.0")
	v.SetDefault("metrics.port", 9090)
	v.SetDefault("metrics.path", "/metrics")

	// Tracing defaults
	v.SetDefault("tracing.enabled", false)
	v.SetDefault("tracing.service_name", "task-queue")
	v.SetDefault("tracing.jaeger_endpoint", "http://localhost:14268/api/traces")
	v.SetDefault("tracing.sample_rate", 1.0)

	// Log defaults
	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "json")
	v.SetDefault("log.output", "stdout")
	v.SetDefault("log.add_caller", false)
	v.SetDefault("log.stack_trace", false)
}

// validate checks if config is valid
func validate(cfg *Config) error {
	if cfg.App.Name == "" {
		return fmt.Errorf("app.name is required")
	}

	if len(cfg.Kafka.Brokers) == 0 {
		return fmt.Errorf("kafka.brokers is required")
	}

	if cfg.Database.Host == "" || cfg.Database.Port == 0 {
		return fmt.Errorf("database connection settings are required")
	}

	if cfg.Worker.Concurrency < 1 {
		return fmt.Errorf("worker.concurrency must be at least 1")
	}

	return nil
}

// MustLoad loads config or panics
func MustLoad(configPath string) *Config {
	cfg, err := Load(configPath)
	if err != nil {
		panic(fmt.Errorf("failed to load config: %w", err))
	}
	return cfg
}
