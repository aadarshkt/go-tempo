package repository

import (
	"context"
	"fmt"
	"go-tempo/internal/domain"
	"go-tempo/internal/metrics"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type taskRepository struct {
	db *gorm.DB
}

// NewTaskRepository creates a new instance of TaskRepository
func NewTaskRepository(db *gorm.DB) *taskRepository {
	return &taskRepository{db: db}
}

func (r *taskRepository) CreateExecution(ctx context.Context, execution *domain.WorkflowExecution, tasks []domain.Task) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("create_execution").Observe(time.Since(start).Seconds())
	}()
	
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the workflow execution
		if err := tx.Create(execution).Error; err != nil {
			return err
		}

		// Create all tasks
		if len(tasks) > 0 {
			if err := tx.Create(&tasks).Error; err != nil {
				return err
			}
		}

		return nil
	})
	
	if err != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("create_execution").Inc()
		metrics.DBTransactionsTotal.WithLabelValues("failed").Inc()
		return err
	}
	
	metrics.DBTransactionsTotal.WithLabelValues("success").Inc()
	return nil
}

func (r *taskRepository) FindTaskByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("find_task").Observe(time.Since(start).Seconds())
	}()
	
	var task domain.Task
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&task).Error
	if err != nil {
		if err != gorm.ErrRecordNotFound {
			metrics.DBQueryErrorsTotal.WithLabelValues("find_task").Inc()
		}
		return nil, err
	}
	return &task, nil
}

func (r *taskRepository) ClaimTask(ctx context.Context, taskID uuid.UUID, workerID string, currentVersion int) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("claim_task").Observe(time.Since(start).Seconds())
	}()
	
	result := r.db.WithContext(ctx).
		Model(&domain.Task{}).
		Where("id = ? AND version = ?", taskID, currentVersion).
		Updates(map[string]interface{}{
			"status":    domain.StatusRunning,
			"worker_id": workerID,
			"version":   currentVersion + 1,
		})
	
	if result.Error != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("claim_task").Inc()
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		// Track optimistic lock conflict
		metrics.DBOptimisticLockConflictsTotal.WithLabelValues("claim_task").Inc()
		return gorm.ErrRecordNotFound // Task was already claimed by another worker
	}
	
	return nil
}

func (r *taskRepository) FindChildren(ctx context.Context, executionID uuid.UUID, parentName string) ([]domain.Task, error) {
	var tasks []domain.Task
	// Find tasks where dependencies JSON array contains the parentName
	err := r.db.WithContext(ctx).
		Where("execution_id = ?", executionID).
		Where("dependencies @> ?", `["`+parentName+`"]`).
		Find(&tasks).Error
	
	return tasks, err
}

func (r *taskRepository) MarkCompleted(ctx context.Context, taskID uuid.UUID, output datatypes.JSON) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("mark_completed").Observe(time.Since(start).Seconds())
	}()
	
	err := r.db.WithContext(ctx).
		Model(&domain.Task{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status": domain.StatusCompleted,
			"output": output,
		}).Error
	
	if err != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("mark_completed").Inc()
	}
	return err
}

func (r *taskRepository) MarkFailed(ctx context.Context, taskID uuid.UUID, errMessage string) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("mark_failed").Observe(time.Since(start).Seconds())
	}()
	
	err := r.db.WithContext(ctx).
		Model(&domain.Task{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status":     domain.StatusFailed,
			"last_error": errMessage,
			"output":     datatypes.JSON([]byte(`{"error":"` + errMessage + `"}`)),
		}).Error
	
	if err != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("mark_failed").Inc()
	}
	return err
}

func (r *taskRepository) MarkSkipped(ctx context.Context, taskID uuid.UUID) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("mark_skipped").Observe(time.Since(start).Seconds())
	}()
	
	err := r.db.WithContext(ctx).
		Model(&domain.Task{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status": domain.StatusSkipped,
			"output": datatypes.JSON([]byte(`{"skipped": true, "reason": "parent task failed"}`)),
		}).Error
	
	if err != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("mark_skipped").Inc()
	}
	return err
}

func (r *taskRepository) IncrementRetryCount(ctx context.Context, taskID uuid.UUID, currentVersion int) error {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("increment_retry").Observe(time.Since(start).Seconds())
	}()
	
	result := r.db.WithContext(ctx).
		Model(&domain.Task{}).
		Where("id = ? AND version = ?", taskID, currentVersion).
		Updates(map[string]interface{}{
			"retry_count": gorm.Expr("retry_count + 1"),
			"status":      domain.StatusPending,
			"version":     currentVersion + 1,
		})
	
	if result.Error != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("increment_retry").Inc()
		return result.Error
	}
	
	if result.RowsAffected == 0 {
		metrics.DBOptimisticLockConflictsTotal.WithLabelValues("increment_retry").Inc()
		return gorm.ErrRecordNotFound
	}
	
	return nil
}

func (r *taskRepository) DecrementAndGetReadyTasks(ctx context.Context, executionID uuid.UUID, completedRefID string) ([]uuid.UUID, error) {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("decrement_ready").Observe(time.Since(start).Seconds())
	}()
	
	var readyTaskIDs []uuid.UUID

	query := `
		UPDATE tasks 
		SET in_degree = in_degree - 1,
		    status = CASE WHEN in_degree - 1 = 0 THEN 'QUEUED' ELSE status END
		WHERE execution_id = ? 
		  AND dependencies @> ?
		RETURNING id, in_degree
	`

	depParam := fmt.Sprintf(`["%s"]`, completedRefID)

	rows, err := r.db.WithContext(ctx).Raw(query, executionID, depParam).Rows()
	if err != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("decrement_ready").Inc()
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var inDegree int
		if err := rows.Scan(&id, &inDegree); err != nil {
			return nil, err
		}

		if inDegree == 0 {
			readyTaskIDs = append(readyTaskIDs, id)
		}
	}

	return readyTaskIDs, nil
}

func (r *taskRepository) DecrementAndSetSkipHint(ctx context.Context, executionID uuid.UUID, failedRefID string) ([]uuid.UUID, error) {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("decrement_skip").Observe(time.Since(start).Seconds())
	}()
	
	var readyTaskIDs []uuid.UUID

	query := `
		UPDATE tasks 
		SET in_degree = in_degree - 1,
		    skip_hint = true,
		    status = CASE WHEN in_degree - 1 = 0 THEN 'QUEUED' ELSE status END
		WHERE execution_id = ? 
		  AND dependencies @> ?
		RETURNING id, in_degree
	`

	depParam := fmt.Sprintf(`["%s"]`, failedRefID)

	rows, err := r.db.WithContext(ctx).Raw(query, executionID, depParam).Rows()
	if err != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("decrement_skip").Inc()
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var inDegree int
		if err := rows.Scan(&id, &inDegree); err != nil {
			return nil, err
		}

		if inDegree == 0 {
			readyTaskIDs = append(readyTaskIDs, id)
		}
	}

	return readyTaskIDs, nil
}

func (r *taskRepository) AreAllTasksCompleted(ctx context.Context, executionID uuid.UUID) (bool, error) {
	start := time.Now()
	defer func() {
		metrics.DBQueryDuration.WithLabelValues("check_completed").Observe(time.Since(start).Seconds())
	}()
	
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.Task{}).
		Where("execution_id = ? AND status != ?", executionID, domain.StatusCompleted).
		Count(&count).Error
	
	if err != nil {
		metrics.DBQueryErrorsTotal.WithLabelValues("check_completed").Inc()
		return false, err
	}
	
	return count == 0, nil
}
