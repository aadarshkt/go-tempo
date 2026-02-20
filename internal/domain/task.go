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
	ID uuid.UUID `gorm:"type:uuid;primary_key;"`
	ExecutionID uuid.UUID `gorm:"type:uuid;index;not null"`
	Name string `gorm:"type:varchar(100);not null"`
	Status TaskStatus `gorm:"type:varchar(20);index;default:'PENDING'"`
	RetryCount int `gorm:"default:0"`
	Dependencies datatypes.JSON `gorm:"type:jsonb"`
	WorkerID *string `gorm:"type:varchar(100);index"`
	Version int `gorm:"default:1"`

	Input datatypes.JSON `gorm:"type:jsonb"`
	Output datatypes.JSON `gorm:"type:jsonb"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewTask(executionID uuid.UUID, name string) *Task {
	return &Task{
		ID:          uuid.New(),
		ExecutionID: executionID,
		Name:        name,
		Status:      StatusPending,
		Version:     1,
		CreatedAt:   time.Now(),
	}
}

func (t *Task) canRetry(maxRetry int) bool {
	return t.RetryCount < maxRetry
}




