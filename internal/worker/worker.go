package worker

import (
	"context"
	"log"

	"go-tempo/internal/core/ports"
	"go-tempo/internal/domain"

	"github.com/google/uuid"
)

type Worker struct {
	workerID     string
	queue        ports.TaskQueue
	repo         ports.TaskRepository
	workflowRepo ports.WorkflowRepository
	eventBus     ports.EventBus
	registry     TaskRegistry
}

func NewWorker(q ports.TaskQueue, r ports.TaskRepository, wfRepo ports.WorkflowRepository, bus ports.EventBus, reg TaskRegistry) *Worker {
	return &Worker{
		workerID:     uuid.New().String(),
		queue:        q,
		repo:         r,
		workflowRepo: wfRepo,
		eventBus:     bus,
		registry:     reg,
	}
}

// ProcessNextTask handles exactly ONE task lifecycle
func (w *Worker) ProcessNextTask(ctx context.Context) {
	// 1. POP: Wait until a task is available
	taskIDStr, err := w.queue.Pop(ctx)
	if err != nil {
		log.Printf("Worker error popping from queue: %v", err)
		return
	}

	// 2. FETCH: Get the full task data from DB
	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		log.Printf("Worker failed to parse task ID %s: %v", taskIDStr, err)
		return
	}

	task, err := w.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		log.Printf("Worker failed to find task %s: %v", taskIDStr, err)
		return
	}

	// 2.5. CHECK SKIP HINT: If task is marked for skipping, handle it immediately
	if task.SkipHint {
		log.Printf("Worker %s skipping task %s (parent task failed)", w.workerID, task.RefID)
		
		err = w.repo.MarkSkipped(ctx, task.ID)
		if err != nil {
			log.Printf("Worker failed to mark task %s as skipped: %v", task.RefID, err)
			return
		}

		// Publish termination event (type: skipped) so coordinator can propagate to children
		terminationEvent := domain.TaskTerminatedEvent{
			ExecutionID: task.ExecutionID,
			TaskID:      task.ID,
			RefID:       task.RefID,
			Type:        domain.TaskTerminationSkipped,
			Error:       "skipped due to parent task failure",
		}
		w.eventBus.PublishTaskTerminated(ctx, terminationEvent)
		log.Printf("Worker successfully skipped task %s", task.RefID)
		return
	}

	// 3. CLAIM: Attempt to claim the task with optimistic locking
	err = w.repo.ClaimTask(ctx, task.ID, w.workerID, task.Version)
	if err != nil {
		log.Printf("Worker %s failed to claim task %s (already claimed by another worker): %v", w.workerID, task.RefID, err)
		return
	}
	// Update in-memory version to match DB after claim (version was incremented in DB)
	task.Version++
	log.Printf("Worker %s claimed task %s", w.workerID, task.RefID)

	// 4. EXECUTE: Find the right function and run it
	handler, exists := w.registry[task.Action]
	if !exists {
		log.Printf("Worker unknown action: %s", task.Action)
		w.repo.MarkFailed(ctx, task.ID, "unknown action") // Mark as failed in DB
		
		// Publish termination event (type: failed)
		terminationEvent := domain.TaskTerminatedEvent{
			ExecutionID: task.ExecutionID,
			TaskID:      task.ID,
			RefID:       task.RefID,
			Type:        domain.TaskTerminationFailed,
			Error:       "unknown action",
		}
		w.eventBus.PublishTaskTerminated(ctx, terminationEvent)
		w.workflowRepo.UpdateStatus(ctx, task.ExecutionID, string(domain.WorkflowFailed))
		return
	}

	output, err := handler(ctx, []byte(task.Input))
	if err != nil {
		log.Printf("Worker task %s failed: %v", task.RefID, err)
		
		// Check if task can be retried
		if task.CanRetry(task.MaxRetries) {
			log.Printf("Worker retrying task %s (retry %d/%d)", task.RefID, task.RetryCount+1, task.MaxRetries)
			
			// Increment retry count and reset to PENDING
			retryErr := w.repo.IncrementRetryCount(ctx, task.ID, task.Version)
			if retryErr != nil {
				log.Printf("Worker failed to increment retry count for task %s: %v", task.RefID, retryErr)
				return
			}
			
			// Push back to queue for retry
			pushErr := w.queue.Push(ctx, task.ID.String())
			if pushErr != nil {
				log.Printf("Worker failed to push task %s back to queue: %v", task.RefID, pushErr)
			}
			return
		}
		
		// Retries exhausted - mark as failed permanently
		log.Printf("Worker task %s exhausted all retries, marking as failed", task.RefID)
		w.repo.MarkFailed(ctx, task.ID, err.Error())
		
		// Mark workflow as failed
		w.workflowRepo.UpdateStatus(ctx, task.ExecutionID, string(domain.WorkflowFailed))
		
		// Publish termination event (type: failed) to propagate skip hint to children
		terminationEvent := domain.TaskTerminatedEvent{
			ExecutionID: task.ExecutionID,
			TaskID:      task.ID,
			RefID:       task.RefID,
			Type:        domain.TaskTerminationFailed,
			Error:       err.Error(),
		}
		w.eventBus.PublishTaskTerminated(ctx, terminationEvent)
		return
	}

	// 5. COMPLETE: Save output and publish event
	w.repo.MarkCompleted(ctx, task.ID, output)

	event := domain.TaskCompletedEvent{
		ExecutionID: task.ExecutionID,
		TaskID:      task.ID,
		RefID:       task.RefID,
	}
	w.eventBus.PublishTaskCompleted(ctx, event)

	log.Printf("Worker successfully finished %s", task.RefID)
}

// StartPool launches multiple concurrent worker loops
func (w *Worker) StartPool(ctx context.Context, concurrency int) {
	log.Printf("Starting worker pool with %d concurrent workers...", concurrency)

	for i := 0; i < concurrency; i++ {
		go func(threadID int) {
			log.Printf("Worker thread %d (ID: %s) started", threadID, w.workerID)
			for {
				select {
				case <-ctx.Done():
					log.Printf("Worker thread %d (ID: %s) shutting down", threadID, w.workerID)
					return
				default:
					w.ProcessNextTask(ctx)
				}
			}
		}(i)
	}
}
