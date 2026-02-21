package worker

import (
	"context"
	"fmt"
)

// TaskHandler is the blueprint for any function that does work
type TaskHandler func(ctx context.Context, input []byte) ([]byte, error)

// TaskRegistry holds all our executable actions
type TaskRegistry map[string]TaskHandler

// InitRegistry wires up the actual business logic
func InitRegistry() TaskRegistry {
	registry := make(TaskRegistry)

	registry["send_email"] = func(ctx context.Context, input []byte) ([]byte, error) {
		// Example: Parse input JSON, call an external API (like SendGrid)
		fmt.Printf("Sending email with payload: %s\n", string(input))
		return []byte(`{"status": "success", "email_id": "123"}`), nil
	}

	registry["charge_card"] = func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Printf("Charging card with payload: %s\n", string(input))
		return []byte(`{"transaction_id": "txn_999"}`), nil
	}

	return registry
}