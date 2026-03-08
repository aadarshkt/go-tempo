package mapper

import (
	"encoding/json"
	"go-tempo/internal/api/dto"
	"go-tempo/internal/domain"

	"github.com/google/uuid"
)

// ToWorkflowExecution converts a CreateWorkflowRequest DTO to domain entities
// Returns the workflow execution and all tasks
func ToWorkflowExecution(req dto.CreateWorkflowRequest) (*domain.WorkflowExecution, []domain.Task) {
	execution := domain.NewWorkflow(req.UserID, req.Type)
	
	tasks := make([]domain.Task, 0, len(req.Tasks))
	for _, taskDTO := range req.Tasks {
		task := ToTask(execution.ID, taskDTO)
		tasks = append(tasks, *task)
	}
	
	return execution, tasks
}

// ToTask converts a single TaskDTO to a Task domain entity
func ToTask(workflowID uuid.UUID, taskDTO dto.TaskDTO) *domain.Task {
	task := domain.NewTask(workflowID, taskDTO.RefID, taskDTO.Action)
	
	// Marshal dependencies to JSON
	depJSON, _ := json.Marshal(taskDTO.Dependencies)
	task.Dependencies = depJSON
	task.InDegree = len(taskDTO.Dependencies)
	
	// Marshal input to JSON
	inputJSON, _ := json.Marshal(taskDTO.Input)
	task.Input = inputJSON
	
	// Set initial status based on dependencies
	if task.InDegree == 0 {
		task.Status = domain.StatusQueued
	} else {
		task.Status = domain.StatusPending
	}
	
	return task
}
