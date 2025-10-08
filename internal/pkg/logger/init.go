package logger

import "github.com/Kosench/go-taskflow/internal/pkg/config"

func InitFromConfig(cfg *config.LogConfig) *Logger {
	return New(Config{
		Level:      cfg.Level,
		Format:     cfg.Format,
		Output:     cfg.Output,
		FilePath:   cfg.FilePath,
		AddCaller:  cfg.AddCaller,
		StackTrace: cfg.StackTrace,
	})
}
