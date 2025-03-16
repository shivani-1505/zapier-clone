-- Create users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    full_name TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Create auth_tokens table for storing refresh tokens
CREATE TABLE IF NOT EXISTS auth_tokens (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create connections table for third-party service connections
CREATE TABLE IF NOT EXISTS connections (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    service TEXT NOT NULL,
    status TEXT NOT NULL,
    auth_type TEXT NOT NULL,
    auth_data TEXT NOT NULL, -- Encrypted authentication data
    metadata TEXT, -- JSON metadata about the connection
    last_used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create workflows table
CREATE TABLE IF NOT EXISTS workflows (
    id SERIAL PRIMARY KEY,
    user_id INTEGER NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL,
    trigger_service TEXT NOT NULL,
    trigger_id TEXT NOT NULL,
    trigger_config TEXT NOT NULL, -- JSON configuration for the trigger
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

-- Create workflow_actions table
CREATE TABLE IF NOT EXISTS workflow_actions (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL,
    action_service TEXT NOT NULL,
    action_id TEXT NOT NULL,
    action_config TEXT NOT NULL, -- JSON configuration for the action
    position INTEGER NOT NULL, -- Order of action in the workflow
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

-- Create workflow_data_mappings table
CREATE TABLE IF NOT EXISTS workflow_data_mappings (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL,
    source_service TEXT NOT NULL,
    source_field TEXT NOT NULL,
    target_service TEXT NOT NULL,
    target_field TEXT NOT NULL,
    transformer TEXT, -- Optional transformer function
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

-- Create workflow_executions table
CREATE TABLE IF NOT EXISTS workflow_executions (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL,
    status TEXT NOT NULL,
    trigger_data TEXT, -- JSON data from the trigger
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    error TEXT,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

-- Create workflow_action_executions table
CREATE TABLE IF NOT EXISTS workflow_action_executions (
    id SERIAL PRIMARY KEY,
    workflow_execution_id INTEGER NOT NULL,
    workflow_action_id INTEGER NOT NULL,
    status TEXT NOT NULL,
    input_data TEXT, -- JSON input data
    output_data TEXT, -- JSON output data
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    error TEXT,
    FOREIGN KEY (workflow_execution_id) REFERENCES workflow_executions(id) ON DELETE CASCADE,
    FOREIGN KEY (workflow_action_id) REFERENCES workflow_actions(id) ON DELETE CASCADE
);

-- Create scheduled_triggers table
CREATE TABLE IF NOT EXISTS scheduled_triggers (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL,
    schedule TEXT NOT NULL, -- Cron expression
    last_triggered_at TIMESTAMP,
    next_trigger_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

-- Create webhook_triggers table
CREATE TABLE IF NOT EXISTS webhook_triggers (
    id SERIAL PRIMARY KEY,
    workflow_id INTEGER NOT NULL,
    webhook_url TEXT NOT NULL UNIQUE,
    secret TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (workflow_id) REFERENCES workflows(id) ON DELETE CASCADE
);

-- Create trigger_types table for available trigger types
CREATE TABLE IF NOT EXISTS trigger_types (
    id SERIAL PRIMARY KEY,
    service TEXT NOT NULL,
    trigger_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    input_schema TEXT NOT NULL, -- JSON schema for trigger configuration
    output_schema TEXT NOT NULL, -- JSON schema for trigger output
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(service, trigger_id)
);

-- Create action_types table for available action types
CREATE TABLE IF NOT EXISTS action_types (
    id SERIAL PRIMARY KEY,
    service TEXT NOT NULL,
    action_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    input_schema TEXT NOT NULL, -- JSON schema for action configuration
    output_schema TEXT NOT NULL, -- JSON schema for action output
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(service, action_id)
);

-- Create function for RETURNING clauses in older PostgreSQL versions
CREATE OR REPLACE FUNCTION last_insert_id() RETURNS INTEGER AS $$
BEGIN
    RETURN lastval();
END;
$$ LANGUAGE plpgsql;

-- Create indices for frequent queries
CREATE INDEX IF NOT EXISTS idx_connections_user_id ON connections(user_id);
CREATE INDEX IF NOT EXISTS idx_connections_service ON connections(service);
CREATE INDEX IF NOT EXISTS idx_workflows_user_id ON workflows(user_id);
CREATE INDEX IF NOT EXISTS idx_workflows_status ON workflows(status);
CREATE INDEX IF NOT EXISTS idx_workflow_actions_workflow_id ON workflow_actions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_workflow_id ON workflow_executions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_status ON workflow_executions(status);