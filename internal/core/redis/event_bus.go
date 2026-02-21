package redis

import (
	"context"
	"go-tempo/internal/domain"
)

type TaskQueue interface {
    // Push a Task UUID to the "To-Do" list
    Push(ctx context.Context, taskID string) error

    // Wait (Block) until a Task UUID is available
    Pop(ctx context.Context) (string, error)
}

type EventBus interface {
	// Publish "Task A is done" to Redis Pub/Sub
	PublishTaskCompleted(ctx context.Context, event domain.TaskCompletedEvent) error

	// Subscribe to events (Used by Coordinator)
	SubscribeToEvents(ctx context.Context) (<-chan domain.TaskCompletedEvent, error)
}