package kafka

import (
	"crypto/tls"
	"github.com/IBM/sarama"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"time"
)

func BuildProducerConfig(cfg *config.ProducerConfig) *sarama.Config {
	config := sarama.NewConfig()

	config.Version = sarama.V3_3_0_0

	config.Producer.Return.Successes = true
	config.Producer.Return.Errors = true

	switch cfg.RequiredAcks {
	case 0:
		config.Producer.RequiredAcks = sarama.NoResponse
	case 1:
		config.Producer.RequiredAcks = sarama.WaitForLocal
	case -1:
		config.Producer.RequiredAcks = sarama.WaitForAll
	default:
		config.Producer.RequiredAcks = sarama.WaitForLocal
	}

	// Compression
	switch cfg.Compression {
	case "gzip":
		config.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		config.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		config.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		config.Producer.Compression = sarama.CompressionZSTD
	default:
		config.Producer.Compression = sarama.CompressionNone
	}

	config.Producer.Flush.Frequency = cfg.FlushFrequency
	config.Producer.MaxMessageBytes = cfg.MaxMessageBytes

	// Retry settings
	config.Producer.Retry.Max = cfg.RetryMax
	config.Producer.Retry.Backoff = cfg.RetryBackoff

	// Idempotent producer
	if cfg.Idempotent {
		config.Producer.Idempotent = true
		config.Producer.RequiredAcks = sarama.WaitForAll
		config.Producer.Retry.Max = 5
		config.Net.MaxOpenRequests = 1
	}

	// Metadata
	config.Metadata.Full = true
	config.Metadata.Retry.Max = 10
	config.Metadata.Retry.Backoff = 250 * time.Millisecond
	config.Metadata.RefreshFrequency = 10 * time.Minute

	// Client ID
	config.ClientID = "task-queue-producer"

	return config
}

func BuildConsumerConfig(cfg *config.ConsumerConfig) *sarama.Config {
	config := sarama.NewConfig()

	// Version
	config.Version = sarama.V3_3_0_0

	// Consumer settings
	config.Consumer.Return.Errors = true
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	// Auto offset reset
	switch cfg.AutoOffsetReset {
	case "latest":
		config.Consumer.Offsets.Initial = sarama.OffsetNewest
	case "earliest":
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	default:
		config.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	// Consumer group settings
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Group.Session.Timeout = cfg.SessionTimeout
	config.Consumer.Group.Heartbeat.Interval = cfg.HeartbeatInterval

	// Isolation level
	if cfg.IsolationLevel == "read_committed" {
		config.Consumer.IsolationLevel = sarama.ReadCommitted
	}

	// Auto commit
	if cfg.EnableAutoCommit {
		config.Consumer.Offsets.AutoCommit.Enable = true
		config.Consumer.Offsets.AutoCommit.Interval = 1 * time.Second
	}

	// Performance
	config.Consumer.MaxProcessingTime = 30 * time.Second
	config.Consumer.Fetch.Default = 1024 * 1024 // 1MB

	// Client ID
	config.ClientID = "task-queue-consumer"

	return config
}

// BuildTLSConfig creates TLS configuration if needed
func BuildTLSConfig(certFile, keyFile, caFile string, skipVerify bool) (*tls.Config, error) {
	cfg := &tls.Config{
		InsecureSkipVerify: skipVerify,
	}

	// Load client cert if provided
	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, err
		}
		cfg.Certificates = []tls.Certificate{cert}
	}

	return cfg, nil
}
