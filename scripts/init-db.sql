-- Create extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Create a simple health check table
CREATE TABLE IF NOT EXISTS health_check (
    id SERIAL PRIMARY KEY,
    checked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert initial health check record
INSERT INTO health_check (checked_at) VALUES (NOW());

-- Grant permissions
GRANT ALL PRIVILEGES ON DATABASE taskqueue TO taskqueue;
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO taskqueue;
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO taskqueue;
