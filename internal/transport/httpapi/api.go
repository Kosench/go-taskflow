package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/service"
	"github.com/google/uuid"
)

const maxRequestBody = 2 << 20

type API struct {
	tasks  service.TaskService
	health HealthChecker
	logger *logger.Logger
}

type createTaskRequest struct {
	Type        domain.TaskType   `json:"type"`
	Priority    *domain.Priority  `json:"priority"`
	Payload     json.RawMessage   `json:"payload"`
	MaxRetries  int               `json:"max_retries"`
	ScheduledAt *time.Time        `json:"scheduled_at"`
	Metadata    map[string]string `json:"metadata"`
	TraceID     string            `json:"trace_id"`
}

type cancelTaskRequest struct {
	Reason string `json:"reason"`
}

type taskResponse struct {
	ID          string            `json:"id"`
	Type        domain.TaskType   `json:"type"`
	Status      domain.TaskStatus `json:"status"`
	Priority    domain.Priority   `json:"priority"`
	Payload     json.RawMessage   `json:"payload"`
	Result      json.RawMessage   `json:"result,omitempty"`
	Error       string            `json:"error,omitempty"`
	Retries     int               `json:"retries"`
	MaxRetries  int               `json:"max_retries"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	ScheduledAt *time.Time        `json:"scheduled_at,omitempty"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CompletedAt *time.Time        `json:"completed_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	WorkerID    string            `json:"worker_id,omitempty"`
	TraceID     string            `json:"trace_id,omitempty"`
}

func (a *API) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /health/live", a.liveness)
	mux.HandleFunc("GET /health/ready", a.readiness)
	mux.HandleFunc("GET /health", a.readiness)
	mux.HandleFunc("POST /api/v1/tasks", a.createTask)
	mux.HandleFunc("GET /api/v1/tasks", a.listTasks)
	mux.HandleFunc("GET /api/v1/tasks/stats", a.taskStats)
	mux.HandleFunc("GET /api/v1/tasks/{id}", a.getTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/retry", a.retryTask)
	mux.HandleFunc("POST /api/v1/tasks/{id}/cancel", a.cancelTask)
}

func (a *API) createTask(w http.ResponseWriter, r *http.Request) {
	var input createTaskRequest
	if err := decodeJSON(w, r, &input); err != nil {
		a.writeError(w, err)
		return
	}

	priority := domain.PriorityNormal
	if input.Priority != nil {
		priority = *input.Priority
	}
	task, err := a.tasks.CreateTask(r.Context(), service.CreateTaskRequest{
		Type: input.Type, Priority: priority, Payload: input.Payload,
		MaxRetries: input.MaxRetries, ScheduledAt: input.ScheduledAt,
		Metadata: input.Metadata, TraceID: input.TraceID,
	})
	if err != nil {
		a.writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, toTaskResponse(task))
}

func (a *API) getTask(w http.ResponseWriter, r *http.Request) {
	task, err := a.tasks.GetTask(r.Context(), r.PathValue("id"))
	if err != nil {
		a.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toTaskResponse(task))
}

func (a *API) listTasks(w http.ResponseWriter, r *http.Request) {
	request, err := parseListRequest(r)
	if err != nil {
		a.writeError(w, err)
		return
	}
	tasks, err := a.tasks.ListTask(r.Context(), request)
	if err != nil {
		a.writeError(w, err)
		return
	}

	items := make([]taskResponse, len(tasks))
	for i, task := range tasks {
		items[i] = toTaskResponse(task)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "count": len(items), "limit": request.Limit, "offset": request.Offset,
	})
}

func (a *API) retryTask(w http.ResponseWriter, r *http.Request) {
	if err := a.tasks.RetryTask(r.Context(), r.PathValue("id")); err != nil {
		a.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) cancelTask(w http.ResponseWriter, r *http.Request) {
	var input cancelTaskRequest
	if err := decodeJSON(w, r, &input); err != nil {
		a.writeError(w, err)
		return
	}
	if strings.TrimSpace(input.Reason) == "" {
		a.writeError(w, domain.NewValidationError("reason", "is required"))
		return
	}
	if err := a.tasks.CancelTask(r.Context(), r.PathValue("id"), input.Reason); err != nil {
		a.writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) taskStats(w http.ResponseWriter, r *http.Request) {
	stats, err := a.tasks.GetTaskStats(r.Context())
	if err != nil {
		a.writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (a *API) liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
}

func (a *API) readiness(w http.ResponseWriter, r *http.Request) {
	if a.health == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "database": "not_configured"})
		return
	}
	if err := a.health.HealthCheck(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "database": "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready", "database": "available"})
}

func parseListRequest(r *http.Request) (service.ListTasksRequest, error) {
	query := r.URL.Query()
	limit, err := parseIntQuery(query.Get("limit"), "limit")
	if err != nil {
		return service.ListTasksRequest{}, err
	}
	offset, err := parseIntQuery(query.Get("offset"), "offset")
	if err != nil {
		return service.ListTasksRequest{}, err
	}

	request := service.ListTasksRequest{
		WorkerID: query.Get("worker_id"), TraceID: query.Get("trace_id"),
		Limit: limit, Offset: offset, OrderBy: query.Get("order_by"), OrderDir: query.Get("order_dir"),
	}
	for _, value := range splitQuery(query.Get("status")) {
		request.Status = append(request.Status, domain.TaskStatus(value))
	}
	for _, value := range splitQuery(query.Get("type")) {
		request.Type = append(request.Type, domain.TaskType(value))
	}
	for _, value := range splitQuery(query.Get("priority")) {
		priority, err := strconv.Atoi(value)
		if err != nil {
			return request, domain.NewValidationError("priority", fmt.Sprintf("invalid integer %q", value))
		}
		request.Priority = append(request.Priority, domain.Priority(priority))
	}
	if request.CreatedFrom, err = parseTimeQuery(query.Get("created_from"), "created_from"); err != nil {
		return request, err
	}
	if request.CreatedTo, err = parseTimeQuery(query.Get("created_to"), "created_to"); err != nil {
		return request, err
	}
	if err := request.Validate(); err != nil {
		return request, err
	}
	return request, nil
}

func parseIntQuery(value, field string) (int, error) {
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, domain.NewValidationError(field, "must be an integer")
	}
	return parsed, nil
}

func parseTimeQuery(value, field string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, domain.NewValidationError(field, "must be RFC3339")
	}
	return parsed, nil
}

func splitQuery(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return domain.NewValidationError("body", err.Error())
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return domain.NewValidationError("body", "must contain one JSON object")
	}
	return nil
}

func (a *API) writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	code := "internal_error"
	message := "internal server error"

	var validationErr domain.ValidationError
	var conflictErr domain.ConflictError
	switch {
	case errors.As(err, &validationErr), errors.Is(err, domain.ErrInvalidTaskType),
		errors.Is(err, domain.ErrInvalidPriority), errors.Is(err, domain.ErrEmptyPayload):
		status, code, message = http.StatusBadRequest, "validation_error", err.Error()
	case errors.Is(err, domain.ErrTaskNotFound):
		status, code, message = http.StatusNotFound, "task_not_found", err.Error()
	case errors.As(err, &conflictErr), errors.Is(err, domain.ErrTaskAlreadyExists):
		status, code, message = http.StatusConflict, "conflict", err.Error()
	default:
		a.logger.Error().Err(err).Msg("HTTP API request failed")
	}
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}

func toTaskResponse(task *domain.Task) taskResponse {
	return taskResponse{
		ID: task.ID, Type: task.Type, Status: task.Status, Priority: task.Priority,
		Payload: json.RawMessage(task.Payload), Result: json.RawMessage(task.Result), Error: task.Error,
		Retries: task.Retries, MaxRetries: task.MaxRetries, CreatedAt: task.CreatedAt, UpdatedAt: task.UpdatedAt,
		ScheduledAt: task.ScheduledAt, StartedAt: task.StartedAt, CompletedAt: task.CompletedAt,
		Metadata: task.Metadata, WorkerID: task.WorkerID, TraceID: task.TraceID,
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (w *statusRecorder) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (a *API) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		w.Header().Set("X-Request-ID", requestID)
		ctx := logger.WithRequestID(r.Context(), requestID)
		recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r.WithContext(ctx))
		logger.LogRequest(r.Method, r.URL.Path, recorder.status, time.Since(started))
	})
}

func (a *API) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				a.logger.Error().Interface("panic", recovered).Msg("HTTP handler panic")
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "internal_error", "message": "internal server error"}})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
