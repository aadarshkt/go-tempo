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

	// Subscribe to termination events (failed/skipped)
	terminationChannel, err := c.eventBus.SubscribeToTerminationEvents(ctx)
	if err != nil {
		log.Fatalf("Failed to subscribe to termination events: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			log.Println("Coordinator shutting down...")
			return

		case event := <-eventChannel:
			c.handleTaskCompleted(ctx, event)

		case event := <-terminationChannel:
			c.handleTaskTerminated(ctx, event)
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

// handleTaskTerminated propagates skip hints to child tasks when a task is terminated (failed or skipped)
func (c *Coordinator) handleTaskTerminated(ctx context.Context, event domain.TaskTerminatedEvent) {
	log.Printf("Coordinator: Task %s (%s) terminated with type '%s': %s. Propagating skip hint...", 
		event.RefID, event.TaskID, event.Type, event.Error)

	// Use the specialized method that sets skip_hint=true for children
	readyTaskIDs, err := c.taskRepo.DecrementAndSetSkipHint(ctx, event.ExecutionID, event.RefID)
	if err != nil {
		log.Printf("Database error while decrementing for terminated task: %v\n", err)
		return
	}

	// Push newly unblocked tasks to the queue (they have skip_hint=true in DB)
	for _, taskID := range readyTaskIDs {
		log.Printf("Coordinator: Task %s is now unblocked (will be skipped). Queuing...", taskID)

		err := c.queue.Push(ctx, taskID.String())
		if err != nil {
			log.Printf("Failed to push task %s to queue: %v\n", taskID, err)
		}
	}

	// No workflow completion check - workflow is already marked FAILED by worker for failed tasks
}