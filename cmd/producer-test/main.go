package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"time"

	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/transport/kafka"
	"github.com/google/uuid"
)

func main() {
	var (
		taskType = flag.String("type", "image_resize", "Task type")
		priority = flag.Int("priority", 1, "Task priority (0-3)")
		count    = flag.Int("count", 1, "Number of tasks to send")
	)
	flag.Parse()

	// Load config
	cfg := config.MustLoad("")

	// Initialize logger
	log := logger.InitFromConfig(&cfg.Log)

	// Create producer
	producer, err := kafka.NewProducer(&cfg.Kafka, log)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create producer")
	}
	defer producer.Close()

	ctx := context.Background()

	// Send tasks
	for i := 0; i < *count; i++ {
		task := &domain.Task{
			ID:         uuid.New().String(),
			Type:       domain.TaskType(*taskType),
			Priority:   domain.Priority(*priority),
			Payload:    generatePayload(*taskType),
			CreatedAt:  time.Now(),
			MaxRetries: 3,
		}

		if err := producer.SendTask(ctx, task); err != nil {
			log.Error().Err(err).Msg("failed to send task")
		} else {
			log.Info().
				Str("task_id", task.ID).
				Str("type", *taskType).
				Int("priority", *priority).
				Msg("task sent")
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Wait for async sends to complete
	time.Sleep(2 * time.Second)

	stats := producer.GetStats()
	fmt.Printf("\nProducer Stats:\n")
	fmt.Printf("  Messages sent: %d\n", stats.MessagesSent)
	fmt.Printf("  Errors: %d\n", stats.MessagesErrors)
}

func generatePayload(taskType string) []byte {
	var payload interface{}

	switch taskType {
	case "image_resize":
		payload = map[string]interface{}{
			"width":  800,
			"height": 600,
			"url":    "https://example.com/image.jpg",
		}
	case "send_email":
		payload = map[string]interface{}{
			"to":      "user@example.com",
			"subject": "Test Email",
			"body":    "This is a test email",
		}
	default:
		payload = map[string]interface{}{
			"data": "test",
		}
	}

	data, _ := json.Marshal(payload)
	return data
}
