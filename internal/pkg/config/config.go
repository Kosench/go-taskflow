package config

import (
	"fmt"
	"time"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	Server   ServerConfig   `mapstructure:"server"`
	Database DatabaseConfig `mapstructure:"database"`
	Kafka    KafkaConfig    `mapstructure:"kafka"`
	Worker   WorkerConfig   `mapstructure:"worker"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
	Tracing  TracingConfig  `mapstructure:"tracing"`
	Log      LogConfig      `mapstructure:"log"`
}

// AppConfig contains application settings
type AppConfig struct {
	Name        string `mapstructure:"name"`
	Version     string `mapstructure:"version"`
	Environment string `mapstructure:"environment"` // development, staging, production
	Debug       bool   `mapstructure:"debug"`
}

type ServerConfig struct {
	HTTP HTTPServerConfig `mapstructure:"http"`
	GRPC GRPCServerConfig `mapstructure:"grpc"`
}

type HTTPServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type GRPCServerConfig struct {
	Host              string        `mapstructure:"host"`
	Port              int           `mapstructure:"port"`
	MaxConnectionAge  time.Duration `mapstructure:"max_connection_age"`
	MaxConnectionIdle time.Duration `mapstructure:"max_connection_idle"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Name            string        `mapstructure:"name"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	SSLMode         string        `mapstructure:"ssl_mode"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// KafkaConfig contains Kafka settings
type KafkaConfig struct {
	Brokers  []string       `mapstructure:"brokers"`
	Topics   TopicsConfig   `mapstructure:"topics"`
	Producer ProducerConfig `mapstructure:"producer"`
	Consumer ConsumerConfig `mapstructure:"consumer"`
}

type TopicsConfig struct {
	TasksHigh   string `mapstructure:"tasks_high"`
	TasksNormal string `mapstructure:"tasks_normal"`
	TasksLow    string `mapstructure:"tasks_low"`
	TasksRetry  string `mapstructure:"tasks_retry"`
	TasksDLQ    string `mapstructure:"tasks_dlq"`
	TaskResults string `mapstructure:"task_results"`
}

type ProducerConfig struct {
	RequiredAcks    int           `mapstructure:"required_acks"` // 0: NoResponse, 1: WaitForLocal, -1: WaitForAll
	Compression     string        `mapstructure:"compression"`   // none, gzip, snappy, lz4, zstd
	MaxMessageBytes int           `mapstructure:"max_message_bytes"`
	FlushFrequency  time.Duration `mapstructure:"flush_frequency"`
	Idempotent      bool          `mapstructure:"idempotent"`
	RetryMax        int           `mapstructure:"retry_max"`
	RetryBackoff    time.Duration `mapstructure:"retry_backoff"`
}

type ConsumerConfig struct {
	GroupID           string        `mapstructure:"group_id"`
	AutoOffsetReset   string        `mapstructure:"auto_offset_reset"` // earliest, latest
	EnableAutoCommit  bool          `mapstructure:"enable_auto_commit"`
	SessionTimeout    time.Duration `mapstructure:"session_timeout"`
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
	MaxPollRecords    int           `mapstructure:"max_poll_records"`
	IsolationLevel    string        `mapstructure:"isolation_level"` // read_uncommitted, read_committed
}

// WorkerConfig contains worker settings
type WorkerConfig struct {
	Concurrency     int           `mapstructure:"concurrency"`
	MaxRetries      int           `mapstructure:"max_retries"`
	RetryBackoff    time.Duration `mapstructure:"retry_backoff"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	ProcessTimeout  time.Duration `mapstructure:"process_timeout"`
	BatchSize       int           `mapstructure:"batch_size"`
}

// MetricsConfig contains metrics settings
type MetricsConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	Path    string `mapstructure:"path"`
}

// TracingConfig contains tracing settings
type TracingConfig struct {
	Enabled        bool    `mapstructure:"enabled"`
	ServiceName    string  `mapstructure:"service_name"`
	JaegerEndpoint string  `mapstructure:"jaeger_endpoint"`
	SampleRate     float64 `mapstructure:"sample_rate"`
}

// LogConfig contains logging settings
type LogConfig struct {
	Level      string `mapstructure:"level"`     // debug, info, warn, error
	Format     string `mapstructure:"format"`    // json, console
	Output     string `mapstructure:"output"`    // stdout, stderr, file
	FilePath   string `mapstructure:"file_path"` // if output is file
	AddCaller  bool   `mapstructure:"add_caller"`
	StackTrace bool   `mapstructure:"stack_trace"` // for error level
}

// DatabaseDSN returns formatted PostgreSQL connection string
func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Name, c.SSLMode,
	)
}

// HTTPAddress returns formatted HTTP address
func (c *HTTPServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GRPCAddress returns formatted gRPC address
func (c *GRPCServerConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// MetricsAddress returns formatted metrics address
func (c *MetricsConfig) Address() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}
