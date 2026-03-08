package repository

import (
	"context"
	"go-tempo/internal/core/ports"
	"go-tempo/internal/domain"
	"go-tempo/internal/metrics"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type workflowRepository struct {
	db *gorm.DB
}

// NewWorkflowRepository creates a new instance of WorkflowRepository
func NewWorkflowRepository(db *gorm.DB) ports.WorkflowRepository {
	return &workflowRepository{db: db}
}

func (r *workflowRepository) Create(ctx context.Context, execution *domain.WorkflowExecution) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("create_workflow").Observe(time.Since(start).Seconds())
	}()
	
	err := r.db.WithContext(ctx).Create(execution).Error
	if err != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("create_workflow").Inc()
	}
	return err
}

func (r *workflowRepository) GetByID(ctx context.Context, executionID uuid.UUID) (*domain.WorkflowExecution, error) {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("get_workflow").Observe(time.Since(start).Seconds())
	}()
	
	var execution domain.WorkflowExecution
	err := r.db.WithContext(ctx).Where("id = ?", executionID).First(&execution).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			metrics.DBQueryErrorsTotal.WithLabelValues("get_workflow").Inc()
		}
		return nil, err
	}
	return &execution, nil
}

// UpdateStatus updates the workflow execution status.
// The status check in the WHERE clause prevents duplicate updates when multiple terminal tasks
// (tasks with no children) complete simultaneously. Each completion triggers a workflow check,
// but only the first one will actually update the status - subsequent attempts will be no-ops
// since the status is already set. This eliminates duplicate "workflow completed" log messages.
// Additionally, once a workflow is FAILED, it cannot be overwritten to COMPLETED.
func (r *workflowRepository) UpdateStatus(ctx context.Context, executionID uuid.UUID, status string) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("update_workflow_status").Observe(time.Since(start).Seconds())
	}()
	
	err := r.db.WithContext(ctx).
		Model(&domain.WorkflowExecution{}).
		Where("id = ? AND status != ? AND status != 'FAILED'", executionID, status).
		Update("status", status).Error
	
	if err != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("update_workflow_status").Inc()
	}
	return err
}
