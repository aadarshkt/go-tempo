package worker

import (
	"context"
	"log"

	"go-tempo/internal/core/ports"
	"go-tempo/internal/domain"

	"github.com/google/uuid"
)

type Worker struct {
	queue    ports.TaskQueue
	repo     ports.TaskRepository
	eventBus ports.EventBus
	registry TaskRegistry
}

func NewWorker(q ports.TaskQueue, r ports.TaskRepository, bus ports.EventBus, reg TaskRegistry) *Worker {
	return &Worker{
		queue:    q,
		repo:     r,
		eventBus: bus,
		registry: reg,
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

	// 3. EXECUTE: Find the right function and run it
	handler, exists := w.registry[task.Action]
	if !exists {
		log.Printf("Worker unknown action: %s", task.Action)
		w.repo.MarkFailed(ctx, task.ID, "unknown action") // Mark as failed in DB
		return
	}

	output, err := handler(ctx, []byte(task.Input))
	if err != nil {
		log.Printf("Worker task failed: %v", err)
		w.repo.MarkFailed(ctx, task.ID, err.Error())
		return
	}

	// 4. COMPLETE: Save output and publish event
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
		go func(workerID int) {
			log.Printf("Worker thread %d started", workerID)
			for {
				select {
				case <-ctx.Done():
					log.Printf("Worker thread %d shutting down", workerID)
					return
				default:
					w.ProcessNextTask(ctx)
				}
			}
		}(i)
	}
}
