package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type TaskStatus string

const (
	StatusPending   TaskStatus = "PENDING"
	StatusQueued    TaskStatus = "QUEUED"
	StatusRunning   TaskStatus = "RUNNING"
	StatusCompleted TaskStatus = "COMPLETED"
	StatusFailed    TaskStatus = "FAILED"
)

type Task struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;"`
	ExecutionID uuid.UUID `gorm:"type:uuid;index;not null"`
	
	// --- THE FIX IS HERE ---
	RefID       string    `gorm:"type:varchar(100);not null"` // e.g. "step_1_welcome_email"
	Action      string    `gorm:"type:varchar(100);not null"` // e.g. "send_email"
	// -----------------------

	Status       TaskStatus     `gorm:"type:varchar(20);index;default:'PENDING'"`
	RetryCount   int            `gorm:"default:0"`
	
	// Stores array of RefIDs: ["step_1", "step_2"]
	Dependencies datatypes.JSON `gorm:"type:jsonb"` 
	
	// NEW: Topological Sort Counter
	InDegree     int            `gorm:"default:0"` 
	
	WorkerID     *string        `gorm:"type:varchar(100);index"`
	Version      int            `gorm:"default:1"`

	Input        datatypes.JSON `gorm:"type:jsonb"` // Args for the Action
	Output       datatypes.JSON `gorm:"type:jsonb"` // Result from the Action

	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Updated Factory
func NewTask(executionID uuid.UUID, refID string, action string) *Task {
	return &Task{
		ID:          uuid.New(),
		ExecutionID: executionID,
		RefID:       refID,
		Action:      action,
		Status:      StatusPending,
		Version:     1,
		CreatedAt:   time.Now(),
	}
}

func (t *Task) CanRetry(maxRetry int) bool {
	return t.RetryCount < maxRetry
}




