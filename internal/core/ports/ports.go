package ports

import (
	"context"
	"go-tempo/internal/domain"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

// TaskQueue represents the task queue operations
type TaskQueue interface {
	// Push a Task UUID to the "To-Do" list
	Push(ctx context.Context, taskID string) error

	// Wait (Block) until a Task UUID is available
	Pop(ctx context.Context) (string, error)
}

// EventBus represents the event bus operations
type EventBus interface {
	// Publish "Task A is done" to Redis Pub/Sub
	PublishTaskCompleted(ctx context.Context, event domain.TaskCompletedEvent) error

	// Publish "Task A terminated" (failed or skipped)
	PublishTaskTerminated(ctx context.Context, event domain.TaskTerminatedEvent) error

	// Subscribe to completion events (Used by Coordinator)
	SubscribeToEvents(ctx context.Context) (<-chan domain.TaskCompletedEvent, error)

	// Subscribe to termination events (failed/skipped) (Used by Coordinator)
	SubscribeToTerminationEvents(ctx context.Context) (<-chan domain.TaskTerminatedEvent, error)
}

// TaskRepository represents the task repository operations
type TaskRepository interface {
	// 1. Create a new workflow with all its tasks in one transaction
	CreateExecution(ctx context.Context, execution *domain.WorkflowExecution, tasks []domain.Task) error

	// 2. The "Worker Poll" Query
	// "Find me a task that is QUEUED"
	FindTaskByID(ctx context.Context, id uuid.UUID) (*domain.Task, error)

	// 3. The "Claim" (Optimistic Locking)
	// "Set Status=RUNNING WHERE ID=? AND Version=?"
	ClaimTask(ctx context.Context, taskID uuid.UUID, workerID string, currentVersion int) error

	// 4. The "Coordinator" Logic (Dependency Resolution)
	// "Find all tasks where 'parentName' is in their dependencies list"
	FindChildren(ctx context.Context, executionID uuid.UUID, parentName string) ([]domain.Task, error)

	// 5. The "Check" Logic
	// "Are there any parents for this task that are NOT completed?"
	CountPendingParents(ctx context.Context, executionID uuid.UUID, parentNames []string) (int64, error)

	// 6. Update Final Status
	MarkCompleted(ctx context.Context, taskID uuid.UUID, output datatypes.JSON) error
	MarkFailed(ctx context.Context, taskID uuid.UUID, errMessage string) error
	MarkSkipped(ctx context.Context, taskID uuid.UUID) error

	// 7. Retry Management
	// Increments retry_count and resets status to PENDING using optimistic locking
	IncrementRetryCount(ctx context.Context, taskID uuid.UUID, currentVersion int) error

	// 8. Decrement in-degree and get ready tasks
	// Decrements in_degree for all tasks dependent on completedRefID and returns IDs of tasks that became ready
	DecrementAndGetReadyTasks(ctx context.Context, executionID uuid.UUID, completedRefID string) ([]uuid.UUID, error)

	// 9. Decrement in-degree and set skip hint for failed parent
	// Like DecrementAndGetReadyTasks but also sets skip_hint=true for all children
	DecrementAndSetSkipHint(ctx context.Context, executionID uuid.UUID, failedRefID string) ([]uuid.UUID, error)

	// 10. Check if all tasks in a workflow execution are completed
	// Returns true if all tasks have status COMPLETED, false otherwise
	AreAllTasksCompleted(ctx context.Context, executionID uuid.UUID) (bool, error)
}

// WorkflowRepository represents the workflow repository operations
type WorkflowRepository interface {
	// Create a new execution (e.g., "Onboarding for Alice")
	Create(ctx context.Context, execution *domain.WorkflowExecution) error

	// Get the current status (Is Alice's onboarding done yet?)
	GetByID(ctx context.Context, executionID uuid.UUID) (*domain.WorkflowExecution, error)

	// Update status (e.g., mark as COMPLETED when all tasks are done)
	UpdateStatus(ctx context.Context, executionID uuid.UUID, status string) error
}
