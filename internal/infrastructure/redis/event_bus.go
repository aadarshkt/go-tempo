package redis

import (
	"context"
	"encoding/json"
	"go-tempo/internal/domain" // Update with your actual module path

	"github.com/redis/go-redis/v9"
)

type RedisEventBus struct {
	client              *redis.Client
	channel             string
	terminationChannel  string
}

func NewRedisEventBus(client *redis.Client) *RedisEventBus {
	return &RedisEventBus{
		client:              client,
		channel:             "workflow:events:completed",
		terminationChannel:  "workflow:events:terminated",
	}
}

// PublishTaskCompleted broadcasts the event to the network
func (b *RedisEventBus) PublishTaskCompleted(ctx context.Context, event domain.TaskCompletedEvent) error {
	// Serialize the struct to JSON
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	
	return b.client.Publish(ctx, b.channel, payload).Err()
}

// PublishTaskTerminated broadcasts a task termination event (failed or skipped)
func (b *RedisEventBus) PublishTaskTerminated(ctx context.Context, event domain.TaskTerminatedEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	
	return b.client.Publish(ctx, b.terminationChannel, payload).Err()
}

// SubscribeToEvents opens a continuous stream for the Coordinator
func (b *RedisEventBus) SubscribeToEvents(ctx context.Context) (<-chan domain.TaskCompletedEvent, error) {
	pubsub := b.client.Subscribe(ctx, b.channel)
	
	// Create a Go channel to send messages to the Coordinator
	msgChan := make(chan domain.TaskCompletedEvent)

	// Start a background goroutine to listen to Redis and forward to our Go channel
	go func() {
		defer close(msgChan)
		for {
			select {
			case <-ctx.Done(): // Handle shutdown gracefully
				pubsub.Close()
				return
			default:
				msg, err := pubsub.ReceiveMessage(ctx)
				if err == nil {
					var event domain.TaskCompletedEvent
					if err := json.Unmarshal([]byte(msg.Payload), &event); err == nil {
						msgChan <- event
					}
				}
			}
		}
	}()

	return msgChan, nil
}

// SubscribeToTerminationEvents opens a continuous stream for termination events (failed/skipped)
func (b *RedisEventBus) SubscribeToTerminationEvents(ctx context.Context) (<-chan domain.TaskTerminatedEvent, error) {
	pubsub := b.client.Subscribe(ctx, b.terminationChannel)
	
	msgChan := make(chan domain.TaskTerminatedEvent)

	go func() {
		defer close(msgChan)
		for {
			select {
			case <-ctx.Done():
				pubsub.Close()
				return
			default:
				msg, err := pubsub.ReceiveMessage(ctx)
				if err == nil {
					var event domain.TaskTerminatedEvent
					if err := json.Unmarshal([]byte(msg.Payload), &event); err == nil {
						msgChan <- event
					}
				}
			}
		}
	}()

	return msgChan, nil
}