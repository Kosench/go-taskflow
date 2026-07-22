package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/service"
)

type HealthChecker interface {
	HealthCheck(ctx context.Context) error
}

type Server struct {
	httpServer *http.Server
	handler    http.Handler
	logger     *logger.Logger
}

func NewServer(cfg *config.HTTPServerConfig, tasks service.TaskService, health HealthChecker, log *logger.Logger) *Server {
	api := &API{tasks: tasks, health: health, logger: log}
	mux := http.NewServeMux()
	api.registerRoutes(mux)

	handler := api.recoverPanics(api.logRequests(mux))
	return &Server{
		handler: handler,
		logger:  log,
		httpServer: &http.Server{
			Addr:         cfg.Address(),
			Handler:      handler,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  60 * time.Second,
		},
	}
}

func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) Run() error {
	s.logger.Info().Str("address", s.httpServer.Addr).Msg("HTTP API started")
	err := s.httpServer.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("HTTP server failed: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
