// Package kafka provides Kafka transport implementation for task queue.
// This package re-exports types from subpackages for backward compatibility.
package kafka

import (
	"github.com/Kosench/go-taskflow/internal/transport/kafka/config"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/consumer"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/converter"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/messages"
	"github.com/Kosench/go-taskflow/internal/transport/kafka/producer"
)

// Re-export config functions
var (
	BuildProducerConfig = config.BuildProducerConfig
	BuildConsumerConfig = config.BuildConsumerConfig
	BuildTLSConfig      = config.BuildTLSConfig
)

// Re-export producer types
type (
	Producer      = producer.Producer
	TaskProducer  = producer.TaskProducer
	ProducerStats = producer.ProducerStats
)

// Re-export producer functions
var (
	NewProducer     = producer.NewProducer
	NewTaskProducer = producer.NewTaskProducer
)

// Re-export consumer types
type (
	Consumer                = consumer.Consumer
	TaskConsumer            = consumer.TaskConsumer
	BatchConsumer           = consumer.BatchConsumer
	GracefulConsumer        = consumer.GracefulConsumer
	MessageHandler          = consumer.MessageHandler
	MessageHandlerFunc      = consumer.MessageHandlerFunc
	TaskHandler             = consumer.TaskHandler
	TaskHandlerFunc         = consumer.TaskHandlerFunc
	BatchMessageHandler     = consumer.BatchMessageHandler
	BatchMessageHandlerFunc = consumer.BatchMessageHandlerFunc
	ConsumerStats           = consumer.ConsumerStats
)

// Re-export consumer functions
var (
	NewConsumer         = consumer.NewConsumer
	NewTaskConsumer     = consumer.NewTaskConsumer
	NewBatchConsumer    = consumer.NewBatchConsumer
	NewGracefulConsumer = consumer.NewGracefulConsumer
)

// Re-export converter types
type (
	TaskConverter = converter.TaskConverter
)

// Re-export converter functions
var (
	NewTaskConverter = converter.NewTaskConverter
)

// Re-export message types
type (
	TaskMessage       = messages.TaskMessage
	TaskResultMessage = messages.TaskResultMessage
	RetryMessage      = messages.RetryMessage
	DLQMessage        = messages.DLQMessage
	MessageHeaders    = messages.MessageHeaders
)

// Re-export message functions
var (
	ParseHeaders = messages.ParseHeaders
)
