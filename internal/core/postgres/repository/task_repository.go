package repository

import (
	"context"
	"go-tempo/internal/domain"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

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

	// 7. Decrement in-degree and get ready tasks
	// Decrements in_degree for all tasks dependent on completedRefID and returns IDs of tasks that became ready
	DecrementAndGetReadyTasks(ctx context.Context, executionID uuid.UUID, completedRefID string) ([]uuid.UUID, error)
}

