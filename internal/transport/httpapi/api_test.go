package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/config"
	"github.com/Kosench/go-taskflow/internal/pkg/logger"
	"github.com/Kosench/go-taskflow/internal/repository"
	"github.com/Kosench/go-taskflow/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeTaskService struct {
	createRequest service.CreateTaskRequest
	createTask    *domain.Task
	createErr     error
	getTask       *domain.Task
	getErr        error
	listRequest   service.ListTasksRequest
	listTasks     []*domain.Task
	retryErr      error
	cancelReason  string
	stats         *repository.TaskStats
}

func (f *fakeTaskService) CreateTask(_ context.Context, request service.CreateTaskRequest) (*domain.Task, error) {
	f.createRequest = request
	return f.createTask, f.createErr
}
func (f *fakeTaskService) GetTask(context.Context, string) (*domain.Task, error) {
	return f.getTask, f.getErr
}
func (f *fakeTaskService) ListTask(_ context.Context, request service.ListTasksRequest) ([]*domain.Task, error) {
	f.listRequest = request
	return f.listTasks, nil
}
func (f *fakeTaskService) UpdateTaskStatus(context.Context, string, domain.TaskStatus, []byte, string) error {
	return nil
}
func (f *fakeTaskService) ProcessTask(context.Context, string, string) (*domain.Task, error) {
	return nil, nil
}
func (f *fakeTaskService) RetryTask(context.Context, string) error { return f.retryErr }
func (f *fakeTaskService) CancelTask(_ context.Context, _ string, reason string) error {
	f.cancelReason = reason
	return nil
}
func (f *fakeTaskService) GetTaskStats(context.Context) (*repository.TaskStats, error) {
	return f.stats, nil
}
func (f *fakeTaskService) CleanupOldTasks(context.Context, time.Duration) (int64, error) {
	return 0, nil
}

type fakeHealthChecker struct{ err error }

func (f fakeHealthChecker) HealthCheck(context.Context) error { return f.err }

func newTestHandler(tasks service.TaskService, health HealthChecker) http.Handler {
	cfg := &config.HTTPServerConfig{Host: "127.0.0.1", Port: 8080}
	log := logger.New(logger.Config{Level: "disabled"})
	return NewServer(cfg, tasks, health, log).Handler()
}

func TestCreateTask(t *testing.T) {
	now := time.Now().UTC()
	tasks := &fakeTaskService{createTask: &domain.Task{
		ID: "task-1", Type: domain.TaskTypeWebhook, Status: domain.TaskStatusPending,
		Priority: domain.PriorityNormal, Payload: []byte(`{"url":"https://example.com"}`),
		CreatedAt: now, UpdatedAt: now,
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tasks", stringsReader(`{
		"type":"webhook",
		"payload":{"url":"https://example.com"}
	}`))
	recorder := httptest.NewRecorder()

	newTestHandler(tasks, fakeHealthChecker{}).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusCreated, recorder.Code)
	assert.Equal(t, domain.PriorityNormal, tasks.createRequest.Priority)
	assert.JSONEq(t, `{"url":"https://example.com"}`, string(tasks.createRequest.Payload))
	var response map[string]any
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, map[string]any{"url": "https://example.com"}, response["payload"])
}

func TestListTasksParsesFilters(t *testing.T) {
	tasks := &fakeTaskService{listTasks: []*domain.Task{}}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/tasks?status=pending,retrying&type=webhook&priority=1&limit=25&offset=5&order_by=priority&order_dir=asc", nil)
	recorder := httptest.NewRecorder()

	newTestHandler(tasks, fakeHealthChecker{}).ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, []domain.TaskStatus{domain.TaskStatusPending, domain.TaskStatusRetrying}, tasks.listRequest.Status)
	assert.Equal(t, []domain.TaskType{domain.TaskTypeWebhook}, tasks.listRequest.Type)
	assert.Equal(t, []domain.Priority{domain.PriorityNormal}, tasks.listRequest.Priority)
	assert.Equal(t, 25, tasks.listRequest.Limit)
	assert.Equal(t, 5, tasks.listRequest.Offset)
}

func TestListTasksRejectsUnsafeOrdering(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/v1/tasks?order_by=created_at%3BDROP+TABLE+tasks", nil)
	recorder := httptest.NewRecorder()

	newTestHandler(&fakeTaskService{}, fakeHealthChecker{}).ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "validation_error")
}

func TestGetTaskNotFound(t *testing.T) {
	tasks := &fakeTaskService{getErr: domain.ErrTaskNotFound}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/missing", nil)
	recorder := httptest.NewRecorder()

	newTestHandler(tasks, fakeHealthChecker{}).ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "task_not_found")
}

func TestRetryConflict(t *testing.T) {
	tasks := &fakeTaskService{retryErr: domain.NewConflictError("task", "max retries reached")}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/task-1/retry", nil)
	recorder := httptest.NewRecorder()

	newTestHandler(tasks, fakeHealthChecker{}).ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}

func TestCancelRequiresReason(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/v1/tasks/task-1/cancel", stringsReader(`{"reason":""}`))
	recorder := httptest.NewRecorder()

	newTestHandler(&fakeTaskService{}, fakeHealthChecker{}).ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestHealthEndpoints(t *testing.T) {
	tasks := &fakeTaskService{}

	t.Run("liveness does not depend on database", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		newTestHandler(tasks, fakeHealthChecker{err: errors.New("database down")}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/live", nil))
		assert.Equal(t, http.StatusOK, recorder.Code)
	})

	t.Run("readiness reports database failure", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		newTestHandler(tasks, fakeHealthChecker{err: errors.New("database down")}).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
		assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
		assert.Contains(t, recorder.Body.String(), "not_ready")
	})
}

func stringsReader(value string) *strings.Reader { return strings.NewReader(value) }
