package repository

import (
	"context"
	"fmt"
	"go-tempo/internal/domain"

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
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
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
}

func (r *taskRepository) FindTaskByID(ctx context.Context, id uuid.UUID) (*domain.Task, error) {
	var task domain.Task
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&task).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *taskRepository) ClaimTask(ctx context.Context, taskID uuid.UUID, workerID string, currentVersion int) error {
	result := r.db.WithContext(ctx).
		Model(&domain.Task{}).
		Where("id = ? AND version = ?", taskID, currentVersion).
		Updates(map[string]interface{}{
			"status":    domain.StatusRunning,
			"worker_id": workerID,
			"version":   currentVersion + 1,
		})
	
	if result.Error != nil {
		return result.Error
	}
	
	if result.RowsAffected == 0 {
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

func (r *taskRepository) CountPendingParents(ctx context.Context, executionID uuid.UUID, parentNames []string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&domain.Task{}).
		Where("execution_id = ? AND name IN ? AND status != ?", 
			executionID, parentNames, domain.StatusCompleted).
		Count(&count).Error
	
	return count, err
}

func (r *taskRepository) MarkCompleted(ctx context.Context, taskID uuid.UUID, output datatypes.JSON) error {
	return r.db.WithContext(ctx).
		Model(&domain.Task{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status": domain.StatusCompleted,
			"output": output,
		}).Error
}

func (r *taskRepository) MarkFailed(ctx context.Context, taskID uuid.UUID, errMessage string) error {
	return r.db.WithContext(ctx).
		Model(&domain.Task{}).
		Where("id = ?", taskID).
		Updates(map[string]interface{}{
			"status": domain.StatusFailed,
			"output": datatypes.JSON([]byte(`{"error":"` + errMessage + `"}`)),
		}).Error
}

func (r *taskRepository) DecrementAndGetReadyTasks(ctx context.Context, executionID uuid.UUID, completedRefID string) ([]uuid.UUID, error) {
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
