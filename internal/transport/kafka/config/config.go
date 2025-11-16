package config

import (
	"crypto/tls"
	"github.com/IBM/sarama"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"time"
)

func BuildProducerConfig(cfg *config.ProducerConfig) *sarama.Config {
	saramaConfig := sarama.NewConfig()

	saramaConfig.Version = sarama.V3_3_0_0

	saramaConfig.Producer.Return.Successes = true
	saramaConfig.Producer.Return.Errors = true

	switch cfg.RequiredAcks {
	case 0:
		saramaConfig.Producer.RequiredAcks = sarama.NoResponse
	case 1:
		saramaConfig.Producer.RequiredAcks = sarama.WaitForLocal
	case -1:
		saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
	default:
		saramaConfig.Producer.RequiredAcks = sarama.WaitForLocal
	}

	// Compression
	switch cfg.Compression {
	case "gzip":
		saramaConfig.Producer.Compression = sarama.CompressionGZIP
	case "snappy":
		saramaConfig.Producer.Compression = sarama.CompressionSnappy
	case "lz4":
		saramaConfig.Producer.Compression = sarama.CompressionLZ4
	case "zstd":
		saramaConfig.Producer.Compression = sarama.CompressionZSTD
	default:
		saramaConfig.Producer.Compression = sarama.CompressionNone
	}

	saramaConfig.Producer.Flush.Frequency = cfg.FlushFrequency
	saramaConfig.Producer.MaxMessageBytes = cfg.MaxMessageBytes

	// Retry settings
	saramaConfig.Producer.Retry.Max = cfg.RetryMax
	saramaConfig.Producer.Retry.Backoff = cfg.RetryBackoff

	// Idempotent producer
	if cfg.Idempotent {
		saramaConfig.Producer.Idempotent = true
		saramaConfig.Producer.RequiredAcks = sarama.WaitForAll
		saramaConfig.Producer.Retry.Max = 5
		saramaConfig.Net.MaxOpenRequests = 1
	}

	// Metadata
	saramaConfig.Metadata.Full = true
	saramaConfig.Metadata.Retry.Max = 10
	saramaConfig.Metadata.Retry.Backoff = 250 * time.Millisecond
	saramaConfig.Metadata.RefreshFrequency = 10 * time.Minute

	// Client ID
	saramaConfig.ClientID = "task-queue-producer"

	return saramaConfig
}

func BuildConsumerConfig(cfg *config.ConsumerConfig) *sarama.Config {
	saramaConfig := sarama.NewConfig()

	// Version
	saramaConfig.Version = sarama.V3_3_0_0

	// Consumer settings
	saramaConfig.Consumer.Return.Errors = true
	saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest

	// Auto offset reset
	switch cfg.AutoOffsetReset {
	case "latest":
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetNewest
	case "earliest":
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	default:
		saramaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	}

	// Consumer group settings
	saramaConfig.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	saramaConfig.Consumer.Group.Session.Timeout = cfg.SessionTimeout
	saramaConfig.Consumer.Group.Heartbeat.Interval = cfg.HeartbeatInterval

	// Isolation level
	if cfg.IsolationLevel == "read_committed" {
		saramaConfig.Consumer.IsolationLevel = sarama.ReadCommitted
	}

	// Auto commit
	if cfg.EnableAutoCommit {
		saramaConfig.Consumer.Offsets.AutoCommit.Enable = true
		saramaConfig.Consumer.Offsets.AutoCommit.Interval = 1 * time.Second
	}

	// Performance
	saramaConfig.Consumer.MaxProcessingTime = 30 * time.Second
	saramaConfig.Consumer.Fetch.Default = 1024 * 1024 // 1MB

	// Client ID
	saramaConfig.ClientID = "task-queue-consumer"

	return saramaConfig
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
