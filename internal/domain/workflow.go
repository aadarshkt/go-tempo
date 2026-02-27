package domain

import (
	"time"

	"github.com/google/uuid"
)

type WorkflowStatus string

const (
	WorkflowRunning   WorkflowStatus = "RUNNING"
	WorkflowCompleted WorkflowStatus = "COMPLETED"
	WorkflowFailed    WorkflowStatus = "FAILED"
	WorkflowPaused    WorkflowStatus = "PAUSED"
)

type WorkflowExecution struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;"`
	UserID       uuid.UUID `gorm:"type:uuid;index;not null"`
	WorkflowType string    `gorm:"type:varchar(50);not null"`
	
	// State
	Status       WorkflowStatus    `gorm:"type:varchar(20);default:'RUNNING'"`
	
	// Relationships
	// Note: We don't necessarily need to load Tasks every time we load a Workflow
	Tasks        []Task    `gorm:"foreignKey:ExecutionID"` 
	
	// Audit
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// --- FACTORY ---
func NewWorkflow(userID uuid.UUID, workflowType string) *WorkflowExecution {
	return &WorkflowExecution{
		ID:           uuid.New(),
		UserID:       userID,
		WorkflowType: workflowType,
		Status:       WorkflowRunning,
		CreatedAt:    time.Now(),
	}
}

// --- METHODS ---
func (w *WorkflowExecution) IsFinished() bool {
	return w.Status == WorkflowCompleted || w.Status == WorkflowFailed
}