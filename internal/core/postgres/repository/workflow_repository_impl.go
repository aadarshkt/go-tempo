package repository

import (
	"context"
	"go-tempo/internal/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"go-tempo/internal/core/ports"
)

type workflowRepository struct {
	db *gorm.DB
}

// NewWorkflowRepository creates a new instance of WorkflowRepository
func NewWorkflowRepository(db *gorm.DB) ports.WorkflowRepository {
	return &workflowRepository{db: db}
}

func (r *workflowRepository) Create(ctx context.Context, execution *domain.WorkflowExecution) error {
	return r.db.WithContext(ctx).Create(execution).Error
}

func (r *workflowRepository) GetByID(ctx context.Context, executionID uuid.UUID) (*domain.WorkflowExecution, error) {
	var execution domain.WorkflowExecution
	err := r.db.WithContext(ctx).Where("id = ?", executionID).First(&execution).Error
	if err != nil {
		return nil, err
	}
	return &execution, nil
}

func (r *workflowRepository) UpdateStatus(ctx context.Context, executionID uuid.UUID, status string) error {
	return r.db.WithContext(ctx).
		Model(&domain.WorkflowExecution{}).
		Where("id = ?", executionID).
		Update("status", status).Error
}
