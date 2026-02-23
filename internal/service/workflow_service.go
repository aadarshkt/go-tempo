package service

import (
	"context"
	"encoding/json"
	"go-tempo/internal/api/dto"
	"go-tempo/internal/core/ports"
	"go-tempo/internal/domain"

	"github.com/google/uuid"
)

type WorkflowService interface {
	SubmitWorkflow(ctx context.Context, req dto.CreateWorkflowRequest) (uuid.UUID, error)
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

func (s *workflowService) SubmitWorkflow(ctx context.Context, req dto.CreateWorkflowRequest) (uuid.UUID, error) {
    // 1. Create the Workflow Entity
    execution := domain.NewWorkflow(req.UserID, req.Type)

    // 2. Convert TaskDTOs -> Task Entities
    var tasks []domain.Task
    var rootTasks []domain.Task // Tasks with NO dependencies

    for _, tDto := range req.Tasks {
        //Converting the dto object to domain object
        newTask := domain.NewTask(execution.ID, tDto.RefID, tDto.Action)
        depJSON, _ := json.Marshal(tDto.Dependencies)
        newTask.Dependencies = depJSON
        newTask.InDegree = len(tDto.Dependencies) // e.g., 0 for roots, 2 if waiting on two tasks
        
        // Logic: Is this a root task?
        if newTask.InDegree == 0 {
            newTask.Status = domain.StatusQueued // Ready to run immediately!
            rootTasks = append(rootTasks, *newTask)
        } else {
            newTask.Status = domain.StatusPending // Must wait
        }

        tasks = append(tasks, *newTask)
    }

    // 3. TRANSACTION: Save Workflow + Tasks to DB
    // We pass both to the repository so they save together (Atomic)
    err := s.repo.CreateExecution(ctx, execution, tasks)
    if err != nil {
        return uuid.Nil, err
    }

    // 4. QUEUE: Push Root Tasks to Redis
    // Only the tasks with Status=QUEUED go to Redis now
    for _, t := range rootTasks {
        go s.queue.Push(ctx, t.ID.String()) // Run in background for speed
    }

    return execution.ID, nil
}