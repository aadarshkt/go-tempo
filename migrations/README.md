# Database Migrations

## Running Migrations

### Option 1: Using psql directly

```bash
psql -U postgres -d tempo -f migrations/001_create_workflow_executions.up.sql
psql -U postgres -d tempo -f migrations/002_create_tasks.up.sql
```

### Option 2: Using golang-migrate tool

```bash
# Install golang-migrate
brew install golang-migrate

# Run all migrations
migrate -path migrations -database "postgresql://postgres:password@localhost:5432/tempo?sslmode=disable" up

# Rollback last migration
migrate -path migrations -database "postgresql://postgres:password@localhost:5432/tempo?sslmode=disable" down 1
```

### Option 3: Using GORM AutoMigrate (easiest for development)

Add this line to your main.go after database connection:

```go
db.AutoMigrate(&domain.WorkflowExecution{}, &domain.Task{})
```

## Migration Files

- `001_create_workflow_executions.up.sql` - Creates workflow_executions table
- `001_create_workflow_executions.down.sql` - Rollback for workflow_executions
- `002_create_tasks.up.sql` - Creates tasks table with foreign key
- `002_create_tasks.down.sql` - Rollback for tasks

## Schema Overview

### workflow_executions

- Primary key: `id` (UUID)
- Tracks workflow execution status
- Indexed on: `user_id`, `status`

### tasks

- Primary key: `id` (UUID)
- Foreign key: `execution_id` → `workflow_executions(id)`
- Indexed on: `execution_id`, `status`, `worker_id`
- JSONB fields: `dependencies`, `input`, `output`
