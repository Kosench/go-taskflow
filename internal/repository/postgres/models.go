package postgres

import (
	"database/sql"
	"database/sql/driver"
	"time"

	"github.com/Kosench/go-taskflow/internal/domain"
	"github.com/Kosench/go-taskflow/internal/pkg/database"
)

// JSONB represents a PostgreSQL JSONB column
type JSONB []byte

// Scan implements the Scanner interface for JSONB
func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*j = make([]byte, len(v))
		copy(*j, v)
	case string:
		*j = []byte(v)
	default:
		return nil
	}
	return nil
}

// Value implements the driver Valuer interface for JSONB
func (j JSONB) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return string(j), nil
}

type dbTask struct {
	ID          string            `db:"id"`
	Type        string            `db:"type"`
	Status      string            `db:"status"`
	Priority    int               `db:"priority"`
	Payload     JSONB             `db:"payload"`
	Result      JSONB             `db:"result"`
	Error       sql.NullString    `db:"error"`
	Retries     int               `db:"retries"`
	MaxRetries  int               `db:"max_retries"`
	WorkerID    sql.NullString    `db:"worker_id"`
	TraceID     sql.NullString    `db:"trace_id"`
	CreatedAt   time.Time         `db:"created_at"`
	UpdatedAt   time.Time         `db:"updated_at"`
	ScheduledAt database.NullTime `db:"scheduled_at"`
	StartedAt   database.NullTime `db:"started_at"`
	CompletedAt database.NullTime `db:"completed_at"`
}

// toDBTask converts domain.Task to dbTask
func toDBTask(task *domain.Task) *dbTask {
	dbTask := &dbTask{
		ID:         task.ID,
		Type:       string(task.Type),
		Status:     string(task.Status),
		Priority:   int(task.Priority),
		Retries:    task.Retries,
		MaxRetries: task.MaxRetries,
		CreatedAt:  task.CreatedAt,
		UpdatedAt:  task.UpdatedAt,
	}

	// Set payload as JSONB
	if len(task.Payload) > 0 {
		dbTask.Payload = JSONB(task.Payload)
	}

	// Handle nullable fields
	if len(task.Result) > 0 {
		dbTask.Result = JSONB(task.Result)
	}

	if task.Error != "" {
		dbTask.Error = sql.NullString{String: task.Error, Valid: true}
	}

	if task.WorkerID != "" {
		dbTask.WorkerID = sql.NullString{String: task.WorkerID, Valid: true}
	}

	if task.TraceID != "" {
		dbTask.TraceID = sql.NullString{String: task.TraceID, Valid: true}
	}

	dbTask.ScheduledAt = database.NewNullTime(task.ScheduledAt)
	dbTask.StartedAt = database.NewNullTime(task.StartedAt)
	dbTask.CompletedAt = database.NewNullTime(task.CompletedAt)

	return dbTask
}

// toDomainTask converts dbTask to domain.Task
func toDomainTask(dbTask *dbTask) *domain.Task {
	task := &domain.Task{
		ID:         dbTask.ID,
		Type:       domain.TaskType(dbTask.Type),
		Status:     domain.TaskStatus(dbTask.Status),
		Priority:   domain.Priority(dbTask.Priority),
		Payload:    []byte(dbTask.Payload),
		Retries:    dbTask.Retries,
		MaxRetries: dbTask.MaxRetries,
		CreatedAt:  dbTask.CreatedAt,
		UpdatedAt:  dbTask.UpdatedAt,
		Metadata:   make(map[string]string),
	}

	// Handle nullable fields
	if len(dbTask.Result) > 0 {
		task.Result = []byte(dbTask.Result)
	}

	if dbTask.Error.Valid {
		task.Error = dbTask.Error.String
	}

	if dbTask.WorkerID.Valid {
		task.WorkerID = dbTask.WorkerID.String
	}

	if dbTask.TraceID.Valid {
		task.TraceID = dbTask.TraceID.String
	}

	if dbTask.ScheduledAt.Valid {
		task.ScheduledAt = &dbTask.ScheduledAt.Time
	}

	if dbTask.StartedAt.Valid {
		task.StartedAt = &dbTask.StartedAt.Time
	}

	if dbTask.CompletedAt.Valid {
		task.CompletedAt = &dbTask.CompletedAt.Time
	}

	return task
}
