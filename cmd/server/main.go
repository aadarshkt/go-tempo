package main

import (
	"go-tempo/internal/api/handler"
	"go-tempo/internal/core/postgres/repository"
	"go-tempo/internal/service"
	"log"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
    // 1. Set up database connection
    dsn := "host=localhost user=postgres password=yourpassword dbname=tempo port=5432 sslmode=disable"
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        log.Fatal("Failed to connect to database:", err)
    }

    // 2. Initialize repository
    taskRepo := repository.NewTaskRepository(db)

    // 3. Initialize service with repository
    workflowSvc := service.NewWorkflowService(taskRepo)

    // 4. Initialize handler with service
    workflowHandler := handler.NewWorkflowHandler(workflowSvc)

    // 5. Set up routes
    router := gin.Default()
    
    api := router.Group("/api/v1")
    {
        api.POST("/workflows", workflowHandler.SubmitWorkflow)
    }

    // 6. Start server
    log.Println("Server starting on :8080")
    if err := router.Run(":8080"); err != nil {
        log.Fatal("Failed to start server:", err)
    }
}