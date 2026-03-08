package service

import (
	"context"
	"go-tempo/internal/core/ports"
	"go-tempo/internal/domain"

	"github.com/google/uuid"
)

type WorkflowService interface {
	SubmitWorkflow(ctx context.Context, execution *domain.WorkflowExecution, tasks []domain.Task) (uuid.UUID, error)
}

// The Implementation
type workflowService struct {
    repo  ports.TaskRepository
    queue ports.TaskQueue
}

// Constructor
func NewWorkflowService(repo ports.TaskRepository, queue ports.TaskQueue) WorkflowService {
    return &workflowService{
        repo:  repo,
        queue: queue,
    }
}

func (s *workflowService) SubmitWorkflow(ctx context.Context, execution *domain.WorkflowExecution, tasks []domain.Task) (uuid.UUID, error) {
    
    // Persist workflow and tasks atomically
    if err := s.persistWorkflow(ctx, execution, tasks); err != nil {
        return uuid.Nil, err
    }
    
    // Identify root tasks for enqueueing
    rootTasks := s.getRootTasks(tasks)
    
    // Enqueue root tasks for immediate processing
    if err := s.enqueueRootTasks(ctx, rootTasks); err != nil {
        return uuid.Nil, err
    }
    
    return execution.ID, nil
}

// persistWorkflow saves the workflow and its tasks atomically to the database
func (s *workflowService) persistWorkflow(ctx context.Context, execution *domain.WorkflowExecution, tasks []domain.Task) error {
    return s.repo.CreateExecution(ctx, execution, tasks)
}

// enqueueRootTasks pushes root tasks to the Redis queue for immediate processing
func (s *workflowService) enqueueRootTasks(ctx context.Context, rootTasks []domain.Task) error {
    for _, task := range rootTasks {
        if err := s.queue.Push(ctx, task.ID.String()); err != nil {
            return err
        }
    }
    return nil
}

// getRootTasks filters and returns tasks that have no dependencies (InDegree == 0)
func (s *workflowService) getRootTasks(tasks []domain.Task) []domain.Task {
    rootTasks := make([]domain.Task, 0)
    
    for _, task := range tasks {
        if task.InDegree == 0 {
            rootTasks = append(rootTasks, task)
        }
    }
    
    return rootTasks
}