package dto

import "github.com/google/uuid"

type TaskDTO struct {
	RefID string `json:"ref_id" binding:"required"`
	Action string `json:"action" binding:"required"`
	Dependencies []string `json:"dependencies"`
	Input map[string]any `json:"input" binding:"required"`
}

type CreateWorkflowRequest struct {
	Type string `json:"type" binding:"required"` 
	UserID uuid.UUID `json:"user_id" binding:"required"`
	Tasks []TaskDTO `json:"tasks" binding:"required,min=1"`
}