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

type TaskTerminationType string

const (
	TaskTerminationFailed  TaskTerminationType = "failed"
	TaskTerminationSkipped TaskTerminationType = "skipped"
)

// TaskTerminatedEvent is published when a task fails or is skipped
// Both cases require skip hint propagation to children
type TaskTerminatedEvent struct {
	ExecutionID uuid.UUID           `json:"execution_id"`
	TaskID      uuid.UUID           `json:"task_id"`
	RefID       string              `json:"ref_id"`
	Type        TaskTerminationType `json:"type"`  // "failed" or "skipped"
	Error       string              `json:"error"` // Error message (empty for skipped)
}

