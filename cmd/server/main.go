package main

import (
	"context"
	"go-tempo/internal/api/handler"
	"go-tempo/internal/coordinator"
	"go-tempo/internal/core/postgres/repository"
	"go-tempo/internal/infrastructure/redis"
	"go-tempo/internal/service"
	"go-tempo/internal/worker"
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

    // Configure database connection pool
    sqlDB, err := db.DB()
    if err != nil {
        log.Fatal("Failed to get database instance:", err)
    }
    sqlDB.SetMaxOpenConns(50)
    sqlDB.SetMaxIdleConns(10)
    sqlDB.SetConnMaxLifetime(0) // 0 means connections are reused forever

    // 2. Initialize repository
    taskRepo := repository.NewTaskRepository(db)
    workflowRepo := repository.NewWorkflowRepository(db)

    // 3. Init Redis Client
    rdb := redis.NewRedisClient("localhost:6379")

    // 4. Create the Queue and Bus
    taskQueue := redis.NewRedisQueue(rdb)
    eventBus := redis.NewRedisEventBus(rdb)

    // 5. Initialize service with repository and queue
    workflowSvc := service.NewWorkflowService(taskRepo, taskQueue)

    // 6. Initialize coordinator
    coord := coordinator.NewCoordinator(taskRepo, workflowRepo, taskQueue, eventBus)
    _ = coord // Coordinator will be started later when needed

    // 7. Init Registry and Worker
    registry := worker.InitRegistry()
    w := worker.NewWorker(taskQueue, taskRepo, eventBus, registry)

    // 8. Start 10 concurrent worker threads in the background
    w.StartPool(context.Background(), 10)

    // 9. Initialize handler with service
    workflowHandler := handler.NewWorkflowHandler(workflowSvc)

    // 10. Set up routes
    router := gin.Default()
    
    api := router.Group("/api/v1")
    {
        api.POST("/workflows", workflowHandler.SubmitWorkflow)
    }

    // 11. Start server
    log.Println("Server starting on :8080")
    if err := router.Run(":8080"); err != nil {
        log.Fatal("Failed to start server:", err)
    }
}