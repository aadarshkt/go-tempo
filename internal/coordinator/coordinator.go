package coordinator

import (
	"context"
	"go-tempo/internal/core/ports"
	"go-tempo/internal/domain"
	"log"

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
	// In Kahn's algorithm, when a terminal node (task with no children) completes,
	// readyTaskIDs will be empty. This is an efficient filter to check for workflow completion.
	if len(readyTaskIDs) == 0 {
		c.checkIfWorkflowFinished(ctx, event.ExecutionID)
	}
}

func (c *Coordinator) checkIfWorkflowFinished(ctx context.Context, executionID uuid.UUID) {
	// Check if all tasks in this workflow execution are completed
	allCompleted, err := c.taskRepo.AreAllTasksCompleted(ctx, executionID)
	if err != nil {
		log.Printf("Failed to check workflow %s completion status: %v\n", executionID, err)
		return
	}

	if !allCompleted {
		log.Printf("Workflow %s still has tasks in progress", executionID)
		return
	}

	log.Printf("All tasks completed for workflow %s. Marking as COMPLETED...", executionID)

	// UpdateStatus is idempotent - it only updates if status is different.
	// This prevents duplicate logs when multiple terminal tasks complete simultaneously.
	err = c.workflowRepo.UpdateStatus(ctx, executionID, string(domain.WorkflowCompleted))
	if err != nil {
		log.Printf("Failed to mark workflow %s as completed: %v\n", executionID, err)
		return
	}

	log.Printf("Workflow %s marked as COMPLETED", executionID)
}