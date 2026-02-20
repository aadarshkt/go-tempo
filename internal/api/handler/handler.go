package handler

import (
	"go-tempo/internal/api/dto"
	"go-tempo/internal/service"
	"net/http"
	"github.com/gin-gonic/gin"
)

type WorkflowHandler struct {
	service service.WorkflowService
}

func NewWorkflowHandler(svc service.WorkflowService) *WorkflowHandler {
    return &WorkflowHandler{service: svc}
}

func (h *WorkflowHandler) SubmitWorkflow(c *gin.Context) {
    var req dto.CreateWorkflowRequest

    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error" : err.Error()})
        return
    }

    executionID, err := h.service.SubmitWorkflow(c.Request.Context(), req)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusCreated, dto.CreateWorkflowResponse{ID: executionID})
}