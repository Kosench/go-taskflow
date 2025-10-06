package main

import (
	"fmt"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"log"
	"os"
)

func main() {
	configPath := os.Getenv("CONFIG_PATH")
	cfg := config.MustLoad(configPath)

	fmt.Printf("=== Task Queue API Server ===\n")
	fmt.Printf("Environment: %s\n", cfg.App.Environment)
	fmt.Printf("Debug mode: %v\n", cfg.App.Debug)
	fmt.Printf("HTTP Server: %s\n", cfg.Server.HTTP.Address())
	fmt.Printf("gRPC Server: %s\n", cfg.Server.GRPC.Address())
	fmt.Printf("Database: %s@%s:%d/%s\n",
		cfg.Database.User,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name)
	fmt.Printf("Kafka Brokers: %v\n", cfg.Kafka.Brokers)
	fmt.Printf("Log Level: %s\n", cfg.Log.Level)

	log.Println("Configuration loaded successfully!")
	log.Println("API Server will be implemented in the next phases...")
}
