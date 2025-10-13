package database

import (
	"context"
	"github.com/jmoiron/sqlx"
	"testing"
	"time"

	"github.com/Kosench/go-taskflow/internal/pkg/config"
)

func TestDatabaseConnection(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	cfg := &config.DatabaseConfig{
		Host:            "localhost",
		Port:            5432,
		Name:            "taskqueue",
		User:            "taskqueue",
		Password:        "taskqueue",
		SSLMode:         "disable",
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 5 * time.Minute,
	}

	db, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test health check
	ctx := context.Background()
	if err := db.HealthCheck(ctx); err != nil {
		t.Errorf("health check failed: %v", err)
	}

	// Test transaction
	err = db.WithTransaction(ctx, func(tx *sqlx.Tx) error {
		var result int
		return tx.Get(&result, "SELECT 1")
	})
	if err != nil {
		t.Errorf("transaction test failed: %v", err)
	}
}

func TestBuildDSN(t *testing.T) {
	cfg := &config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Name:     "testdb",
		User:     "testuser",
		Password: "testpass",
		SSLMode:  "require",
	}

	expected := "host=localhost port=5432 user=testuser password=testpass dbname=testdb sslmode=require"
	dsn := buildDSN(cfg)

	if dsn != expected {
		t.Errorf("expected DSN %s, got %s", expected, dsn)
	}
}
