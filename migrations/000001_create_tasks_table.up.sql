-- Create extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm"; -- For text search

-- Create custom types
CREATE TYPE task_status AS ENUM (
    'pending',
    'processing',
    'completed',
    'failed',
    'retrying',
    'cancelled'
);

CREATE TYPE task_type AS ENUM (
    'image_resize',
    'image_convert',
    'send_email',
    'generate_report',
    'data_export',
    'webhook'
);

-- Create tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id VARCHAR(36) PRIMARY KEY DEFAULT uuid_generate_v4()::VARCHAR,
    type task_type NOT NULL,
    status task_status NOT NULL DEFAULT 'pending',
    priority INTEGER NOT NULL DEFAULT 1 CHECK (priority >= 0 AND priority <= 3),
    payload JSONB NOT NULL,
    result JSONB,
    error TEXT,
    retries INTEGER NOT NULL DEFAULT 0,
    max_retries INTEGER NOT NULL DEFAULT 3,
    worker_id VARCHAR(255),
    trace_id VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    scheduled_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,

    -- Indexes for performance
    CONSTRAINT tasks_retries_check CHECK (retries >= 0),
    CONSTRAINT tasks_max_retries_check CHECK (max_retries >= 0)
    );

-- Create indexes
CREATE INDEX idx_tasks_status ON tasks(status) WHERE status IN ('pending', 'retrying');
CREATE INDEX idx_tasks_priority ON tasks(priority DESC) WHERE status IN ('pending', 'retrying');
CREATE INDEX idx_tasks_type ON tasks(type);
CREATE INDEX idx_tasks_created_at ON tasks(created_at DESC);
CREATE INDEX idx_tasks_scheduled_at ON tasks(scheduled_at) WHERE scheduled_at IS NOT NULL;
CREATE INDEX idx_tasks_worker_id ON tasks(worker_id) WHERE worker_id IS NOT NULL;
CREATE INDEX idx_tasks_trace_id ON tasks(trace_id) WHERE trace_id IS NOT NULL;

-- Composite index for queue queries
CREATE INDEX idx_tasks_queue ON tasks(status, priority DESC, created_at)
WHERE status IN ('pending', 'retrying');

-- Create updated_at trigger
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_tasks_updated_at BEFORE UPDATE ON tasks
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create task_metadata table for additional key-value pairs
CREATE TABLE IF NOT EXISTS task_metadata (
    task_id VARCHAR(36) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    key VARCHAR(255) NOT NULL,
    value TEXT,
    PRIMARY KEY (task_id, key)
    );

CREATE INDEX idx_task_metadata_task_id ON task_metadata(task_id);

-- Create task_history table for audit trail
CREATE TABLE IF NOT EXISTS task_history (
    id SERIAL PRIMARY KEY,
    task_id VARCHAR(36) NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    status task_status NOT NULL,
    message TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255)
    );

CREATE INDEX idx_task_history_task_id ON task_history(task_id);
CREATE INDEX idx_task_history_created_at ON task_history(created_at DESC);

-- Add comments for documentation
COMMENT ON TABLE tasks IS 'Main table for storing tasks';
COMMENT ON COLUMN tasks.id IS 'Unique task identifier (UUID)';
COMMENT ON COLUMN tasks.type IS 'Type of the task';
COMMENT ON COLUMN tasks.status IS 'Current status of the task';
COMMENT ON COLUMN tasks.priority IS 'Task priority (0=low, 1=normal, 2=high, 3=critical)';
COMMENT ON COLUMN tasks.payload IS 'Task input data in JSON format';
COMMENT ON COLUMN tasks.result IS 'Task result data in JSON format';
COMMENT ON COLUMN tasks.error IS 'Error message if task failed';
COMMENT ON COLUMN tasks.retries IS 'Number of retry attempts';
COMMENT ON COLUMN tasks.max_retries IS 'Maximum number of retry attempts allowed';
COMMENT ON COLUMN tasks.worker_id IS 'ID of the worker processing this task';
COMMENT ON COLUMN tasks.trace_id IS 'Distributed tracing ID';