package coordinator

import (
	"context"
	"log"
	"go-tempo/internal/core/ports"
	"go-tempo/internal/domain"
	"github.com/google/uuid"
)

type Coordinator struct {
	taskRepo    ports.TaskRepository
	workflowRepo ports.WorkflowRepository
	queue        ports.TaskQueue
	eventBus     ports.EventBus
}

func NewCoordinator(
	taskRepo ports.TaskRepository,
	workflowRepo ports.WorkflowRepository,
	queue ports.TaskQueue,
	bus ports.EventBus,
) *Coordinator {
	return &Coordinator{
		taskRepo:     taskRepo,
		workflowRepo: workflowRepo,
		queue:        queue,
		eventBus:     bus,
	}
}

// Start begins the infinite listening loop. Call this in main.go as a goroutine.
func (c *Coordinator) Start(ctx context.Context) {
	log.Println("Coordinator started, listening for events...")

	// Subscribe returns a Go channel that receives messages from Redis
	eventChannel, err := c.eventBus.SubscribeToEvents(ctx)
	if err != nil {
		log.Fatalf("Failed to subscribe to event bus: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Coordinator shutting down...")
			return

		case event := <-eventChannel:
			c.handleTaskCompleted(ctx, event)
		}
	}
}

// handleTaskCompleted executes Kahn's Algorithm
func (c *Coordinator) handleTaskCompleted(ctx context.Context, event domain.TaskCompletedEvent) {
	log.Printf("Coordinator: Task %s (%s) completed. Checking children...", event.RefID, event.TaskID)

	// 1. The Atomic Update: Tell Postgres this RefID is done.
	readyTaskIDs, err := c.taskRepo.DecrementAndGetReadyTasks(ctx, event.ExecutionID, event.RefID)
	if err != nil {
		log.Printf("Database error while decrementing: %v\n", err)
		return
	}

	// 2. The Kickoff: Push newly unblocked tasks to the queue
	for _, taskID := range readyTaskIDs {
		log.Printf("Coordinator: Task %s is now unblocked! Queuing...", taskID)

		err := c.queue.Push(ctx, taskID.String())
		if err != nil {
			log.Printf("Failed to push task %s to queue: %v\n", taskID, err)
			// Note: In production, you would add a retry mechanism here
		}
	}

	// 3. Workflow Completion Check
	// If readyTaskIDs is empty, we should check if ALL tasks in this execution are COMPLETED.
	// If yes, we mark the WorkflowExecution status as "COMPLETED".
	if len(readyTaskIDs) == 0 {
		c.checkIfWorkflowFinished(ctx, event.ExecutionID)
	}
}

func (c *Coordinator) checkIfWorkflowFinished(ctx context.Context, executionID uuid.UUID) {
	// Check if any tasks are still PENDING, QUEUED, or RUNNING.
	// If count == 0, mark WorkflowExecution as COMPLETED.
	// For now, we'll implement a simple version that marks as completed
	// You would implement a repo method to check if any tasks are still in progress

	log.Printf("Checking if workflow %s is finished...", executionID)

	// TODO: Implement a method in TaskRepository to count pending/running tasks
	// For now, we'll assume if no tasks are ready and we got here, workflow is done
	err := c.workflowRepo.UpdateStatus(ctx, executionID, "COMPLETED")
	if err != nil {
		log.Printf("Failed to mark workflow %s as completed: %v\n", executionID, err)
		return
	}

	log.Printf("Workflow %s marked as COMPLETED", executionID)
}