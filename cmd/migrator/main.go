package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/database"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	var (
		command = flag.String("command", "up", "Migration command: up, down, drop, force, steps, version")
		steps   = flag.Int("steps", 0, "Number of migrations to execute (used with steps command)")
		force   = flag.Int("force", 0, "Force migration version (used with force command)")
	)
	flag.Parse()

	cfg := config.MustLoad("")

	log := logger.InitFromConfig(&cfg.Log)

	db, err := database.New(&cfg.Database)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to database")
	}
	defer db.Close()

	// Create migration instance
	driver, err := postgres.WithInstance(db.DB.DB, &postgres.Config{})
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create postgres driver")
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://migrations",
		cfg.Database.Name,
		driver,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create migrate instance")
	}

	// Execute migration command
	switch *command {
	case "up":
		log.Info().Msg("running migrations up")
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatal().Err(err).Msg("failed to run migrations up")
		}
		log.Info().Msg("migrations completed successfully")

	case "down":
		log.Info().Msg("running migrations down")
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatal().Err(err).Msg("failed to run migrations down")
		}
		log.Info().Msg("migrations rolled back successfully")

	case "drop":
		log.Warn().Msg("dropping all migrations")
		if err := m.Drop(); err != nil {
			log.Fatal().Err(err).Msg("failed to drop migrations")
		}
		log.Info().Msg("all migrations dropped")

	case "force":
		log.Warn().Int("version", *force).Msg("forcing migration version")
		if err := m.Force(*force); err != nil {
			log.Fatal().Err(err).Msg("failed to force migration version")
		}
		log.Info().Msg("migration version forced")

	case "steps":
		log.Info().Int("steps", *steps).Msg("running migration steps")
		if err := m.Steps(*steps); err != nil && err != migrate.ErrNoChange {
			log.Fatal().Err(err).Msg("failed to run migration steps")
		}
		log.Info().Msg("migration steps completed")

	case "version":
		version, dirty, err := m.Version()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to get migration version")
		}
		log.Info().
			Uint("version", version).
			Bool("dirty", dirty).
			Msg("current migration version")

	default:
		fmt.Printf("Unknown command: %s\n", *command)
		fmt.Println("Available commands: up, down, drop, force, steps, version")
		os.Exit(1)
	}
}
