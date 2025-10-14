-- Drop triggers
DROP TRIGGER IF EXISTS update_tasks_updated_at ON tasks;
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables
DROP TABLE IF EXISTS task_history;
DROP TABLE IF EXISTS task_metadata;
DROP TABLE IF EXISTS tasks;

-- Drop types
DROP TYPE IF EXISTS task_status;
DROP TYPE IF EXISTS task_type;