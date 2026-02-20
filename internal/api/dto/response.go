package dto

import "github.com/google/uuid"

type CreateWorkflowResponse struct {
	ID uuid.UUID `json:"execution_id"`
}