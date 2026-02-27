package worker

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// TaskHandler is the blueprint for any function that does work
type TaskHandler func(ctx context.Context, input []byte) ([]byte, error)

// TaskRegistry holds all our executable actions
type TaskRegistry map[string]TaskHandler

// InitRegistry wires up the actual business logic
func InitRegistry() TaskRegistry {
	registry := make(TaskRegistry)

	registry["create_employee_profile"] = func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Printf("Creating employee profile with payload: %s\n", string(input))
		time.Sleep(10 * time.Second)
		return []byte(`{"status": "success", "employee_id": "EMP-12345", "profile_created": true}`), nil
	}

	registry["setup_email_account"] = func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Printf("Setting up email account with payload: %s\n", string(input))
		time.Sleep(10 * time.Second)
		return []byte(`{"status": "success", "email": "john.doe@company.com", "mailbox_created": true}`), nil
	}

	registry["assign_equipment"] = func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Printf("Assigning equipment with payload: %s\n", string(input))
		time.Sleep(10 * time.Second)
		return []byte(`{"status": "success", "laptop_id": "LT-789", "monitor_id": "MN-456", "assigned": true}`), nil
	}

	registry["enroll_benefits"] = func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Printf("Enrolling in benefits with payload: %s\n", string(input))
		time.Sleep(10 * time.Second)
		return []byte(`{"status": "success", "health_plan": "Premium PPO", "401k_enrolled": true}`), nil
	}

	registry["schedule_orientation"] = func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Printf("Scheduling orientation with payload: %s\n", string(input))
		time.Sleep(10 * time.Second)
		return []byte(`{"status": "success", "orientation_date": "2026-03-01", "calendar_invite_sent": true}`), nil
	}

	// Test function: Always fails to simulate task errors
	registry["failing_task"] = func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Printf("Failing task executed with payload: %s\n", string(input))
		return nil, errors.New("simulated task failure")
	}

	// Test function: Long-running task that respects context cancellation (for timeout testing)
	registry["timeout_task"] = func(ctx context.Context, input []byte) ([]byte, error) {
		fmt.Printf("Long-running task started with payload: %s\n", string(input))
		
		// Run for 10 seconds but check for cancellation every 100ms
		for i := 0; i < 100; i++ {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
		
		return []byte(`{"status": "success", "completed": true}`), nil
	}

	return registry
}