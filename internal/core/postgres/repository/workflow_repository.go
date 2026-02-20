package repository

import (
	"context"
	"go-tempo/internal/domain"
	"github.com/google/uuid"
)

type WorkflowRepository interface {
    // Create a new execution (e.g., "Onboarding for Alice")
    Create(ctx context.Context, execution *domain.WorkflowExecution) error

    // Get the current status (Is Alice's onboarding done yet?)
    GetByID(ctx context.Context, executionID uuid.UUID) (*domain.WorkflowExecution, error)

    // Update status (e.g., mark as COMPLETED when all tasks are done)
    UpdateStatus(ctx context.Context, executionID uuid.UUID, status string) error
}