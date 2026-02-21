package domain

import (
	"github.com/google/uuid"
)

// TaskCompletedEvent is published to Redis Pub/Sub by the Worker
type TaskCompletedEvent struct {
	ExecutionID uuid.UUID `json:"execution_id"`
	TaskID      uuid.UUID `json:"task_id"`
	RefID       string    `json:"ref_id"` // e.g., "step_1"
}

