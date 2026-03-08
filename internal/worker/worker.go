package worker

import (
	"context"
	"errors"
	"log"
	"strconv"
	"time"

	"go-tempo/internal/core/ports"
	"go-tempo/internal/domain"
	"go-tempo/internal/metrics"

	"github.com/google/uuid"
)

type Worker struct {
	workerID     string
	queue        ports.TaskQueue // Queue to pull tasks from (main or retry)
	retryQueue   ports.TaskQueue // Queue to push failed tasks for retry
	repo         ports.TaskRepository
	workflowRepo ports.WorkflowRepository
	eventBus     ports.EventBus
	registry     TaskRegistry
}

func NewWorker(q ports.TaskQueue, retryQ ports.TaskQueue, r ports.TaskRepository, wfRepo ports.WorkflowRepository, bus ports.EventBus, reg TaskRegistry) *Worker {
	return &Worker{
		workerID:     uuid.New().String(),
		queue:        q,
		retryQueue:   retryQ,
		repo:         r,
		workflowRepo: wfRepo,
		eventBus:     bus,
		registry:     reg,
	}
}

// ProcessNextTask handles exactly ONE task lifecycle (orchestrates the workflow)
func (w *Worker) ProcessNextTask(ctx context.Context) {
	// 1. Pop and fetch task from queue
	task, err := w.popAndFetchTask(ctx)
	if err != nil {
		return // Error already logged in popAndFetchTask
	}

	// Track queue wait time
	queueWaitTime := time.Since(task.CreatedAt).Seconds()
	metrics.WorkerQueueWaitTime.Observe(queueWaitTime)

	// 2. Check if task should be skipped
	if task.SkipHint {
		w.handleSkippedTask(ctx, task)
		return
	}

	// 3. Claim the task
	if !w.claimTask(ctx, task) {
		return // Failed to claim (already claimed by another worker)
	}

	// Track active task (increment on claim)
	metrics.WorkerActiveTasks.WithLabelValues(w.workerID).Inc()
	defer metrics.WorkerActiveTasks.WithLabelValues(w.workerID).Dec()

	// 4. Execute the task
	output, err := w.executeTaskAction(ctx, task)
	if err != nil {
		w.handleTaskFailure(ctx, task, err)
		return
	}

	// 5. Handle successful completion
	w.handleTaskSuccess(ctx, task, output)
}

// popAndFetchTask pops task ID from queue and fetches full task data from DB
func (w *Worker) popAndFetchTask(ctx context.Context) (*domain.Task, error) {
	taskIDStr, err := w.queue.Pop(ctx)
	if err != nil {
		log.Printf("Worker error popping from queue: %v", err)
		return nil, err
	}

	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		log.Printf("Worker failed to parse task ID %s: %v", taskIDStr, err)
		return nil, err
	}

	task, err := w.repo.FindTaskByID(ctx, taskID)
	if err != nil {
		log.Printf("Worker failed to find task %s: %v", taskIDStr, err)
		return nil, err
	}

	return task, nil
}

// handleSkippedTask marks task as skipped and publishes termination event
func (w *Worker) handleSkippedTask(ctx context.Context, task *domain.Task) {
	log.Printf("Worker %s skipping task %s (parent task failed)", w.workerID, task.RefID)

	err := w.repo.MarkSkipped(ctx, task.ID)
	if err != nil {
		log.Printf("Worker failed to mark task %s as skipped: %v", task.RefID, err)
		return
	}

	// Publish termination event so coordinator can propagate to children
	terminationEvent := domain.NewTaskTerminatedEvent(
		task.ExecutionID,
		task.ID,
		task.RefID,
		domain.TaskTerminationSkipped,
		"skipped due to parent task failure",
	)
	w.eventBus.PublishTaskTerminated(ctx, terminationEvent)

	metrics.WorkerTasksProcessedTotal.WithLabelValues(task.Action, "skipped").Inc()
	log.Printf("Worker successfully skipped task %s", task.RefID)
}

// claimTask attempts to claim the task with optimistic locking
func (w *Worker) claimTask(ctx context.Context, task *domain.Task) bool {
	err := w.repo.ClaimTask(ctx, task.ID, w.workerID, task.Version)
	if err != nil {
		log.Printf("Worker %s failed to claim task %s (already claimed by another worker): %v", w.workerID, task.RefID, err)
		metrics.WorkerClaimFailuresTotal.Inc()
		return false
	}

	// Update in-memory version to match DB after claim (version was incremented in DB)
	task.Version++ //Important if you fetching task again in this worker.
	log.Printf("Worker %s claimed task %s", w.workerID, task.RefID)
	return true
}

// executeTaskAction looks up and executes the task handler
func (w *Worker) executeTaskAction(ctx context.Context, task *domain.Task) ([]byte, error) {
	handler, exists := w.registry[task.Action]
	if !exists {
		log.Printf("Worker unknown action: %s", task.Action)
		w.repo.MarkFailed(ctx, task.ID, "unknown action")

		metrics.WorkerRegistryErrorsTotal.WithLabelValues(task.Action).Inc()
		metrics.WorkerTasksProcessedTotal.WithLabelValues(task.Action, "failed").Inc()

		// Publish termination event
		terminationEvent := domain.NewTaskTerminatedEvent(
			task.ExecutionID,
			task.ID,
			task.RefID,
			domain.TaskTerminationFailed,
			"unknown action",
		)
		w.eventBus.PublishTaskTerminated(ctx, terminationEvent)
		w.workflowRepo.UpdateStatus(ctx, task.ExecutionID, string(domain.WorkflowFailed))

		return nil, errors.New("unknown action")
	}

	// Execute handler and track execution time
	execStart := time.Now()
	output, err := handler(ctx, []byte(task.Input))
	execDuration := time.Since(execStart).Seconds()
	metrics.WorkerTaskDuration.WithLabelValues(task.Action).Observe(execDuration)

	return output, err
}

// handleTaskFailure handles task failure with retry logic
func (w *Worker) handleTaskFailure(ctx context.Context, task *domain.Task, execErr error) {
	log.Printf("Worker task %s failed: %v", task.RefID, execErr)

	// Check if task can be retried
	if task.CanRetry(task.MaxRetries) {
		w.retryTask(ctx, task)
		return
	}

	// Retries exhausted - mark as failed permanently
	w.markTaskFailedPermanently(ctx, task, execErr)
}

// retryTask increments retry count and pushes task back to retry queue
func (w *Worker) retryTask(ctx context.Context, task *domain.Task) {
	log.Printf("Worker retrying task %s (retry %d/%d)", task.RefID, task.RetryCount+1, task.MaxRetries)

	metrics.WorkerRetriesTotal.WithLabelValues(task.Action, strconv.Itoa(task.RetryCount+1)).Inc()

	err := w.repo.IncrementRetryCount(ctx, task.ID, task.Version)
	if err != nil {
		log.Printf("Worker failed to increment retry count for task %s: %v", task.RefID, err)
		return
	}

	pushErr := w.retryQueue.Push(ctx, task.ID.String())
	if pushErr != nil {
		log.Printf("Worker failed to push task %s to retry queue: %v", task.RefID, pushErr)
	}
}

// markTaskFailedPermanently marks task as failed and publishes termination event
func (w *Worker) markTaskFailedPermanently(ctx context.Context, task *domain.Task, execErr error) {
	log.Printf("Worker task %s exhausted all retries, marking as failed", task.RefID)

	w.repo.MarkFailed(ctx, task.ID, execErr.Error())

	metrics.TaskRetryExhaustionTotal.WithLabelValues(task.Action).Inc()
	metrics.WorkerTasksProcessedTotal.WithLabelValues(task.Action, "failed").Inc()

	w.workflowRepo.UpdateStatus(ctx, task.ExecutionID, string(domain.WorkflowFailed))

	// Publish termination event to propagate skip hint to children
	terminationEvent := domain.NewTaskTerminatedEvent(
		task.ExecutionID,
		task.ID,
		task.RefID,
		domain.TaskTerminationFailed,
		execErr.Error(),
	)
	w.eventBus.PublishTaskTerminated(ctx, terminationEvent)
}

// handleTaskSuccess marks task as completed and publishes completion event
func (w *Worker) handleTaskSuccess(ctx context.Context, task *domain.Task, output []byte) {
	w.repo.MarkCompleted(ctx, task.ID, output)

	metrics.WorkerTasksProcessedTotal.WithLabelValues(task.Action, "success").Inc()

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
