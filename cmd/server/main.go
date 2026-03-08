package main

import (
	"context"
	"go-tempo/internal/api/handler"
	"go-tempo/internal/api/middleware"
	"go-tempo/internal/coordinator"
	"go-tempo/internal/core/postgres/repository"
	"go-tempo/internal/infrastructure/redis"
	"go-tempo/internal/metrics"
	"go-tempo/internal/service"
	"go-tempo/internal/worker"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
    // 1. Set up database connection
    dsn := "host=localhost user=postgres password=postgres dbname=workflow_db port=5432 sslmode=disable"
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

    // Start DB connection pool metrics collector
    metrics.StartDBPoolCollector(sqlDB, 10*time.Second)

    // 2. Initialize repository
    taskRepo := repository.NewTaskRepository(db)
    workflowRepo := repository.NewWorkflowRepository(db)

    // 3. Init Redis Client
    rdb := redis.NewRedisClient("localhost:6379")

    // 4. Create the Queues and Bus
    mainQueue := redis.NewRedisQueue(rdb, "workflow:queue:pending")
    retryQueue := redis.NewRedisQueue(rdb, "workflow:queue:retry")
    eventBus := redis.NewRedisEventBus(rdb)

    // Start Redis queue depth metrics collectors for both queues
    metrics.StartRedisQueueDepthCollector(rdb, "workflow:queue:pending", 10*time.Second)
    metrics.StartRedisQueueDepthCollector(rdb, "workflow:queue:retry", 10*time.Second)

    // 5. Initialize service with repository and main queue
    workflowSvc := service.NewWorkflowService(taskRepo, mainQueue)

    // 6. Initialize coordinator and start it
    coord := coordinator.NewCoordinator(taskRepo, workflowRepo, mainQueue, eventBus)
    go coord.Start(context.Background())

    // 7. Init Registry and Workers
    registry := worker.InitRegistry()
    
    // 8. Start worker pools with 9:1 ratio (9 main workers, 1 retry worker)
    // Main queue workers - pull from mainQueue, push retries to retryQueue
    mainWorker := worker.NewWorker(mainQueue, retryQueue, taskRepo, workflowRepo, eventBus, registry)
    go mainWorker.StartPool(context.Background(), 9)
    
    // Retry queue workers - pull from retryQueue, push retries back to retryQueue
    retryWorker := worker.NewWorker(retryQueue, retryQueue, taskRepo, workflowRepo, eventBus, registry)
    go retryWorker.StartPool(context.Background(), 1)

    // 9. Initialize handler with service
    workflowHandler := handler.NewWorkflowHandler(workflowSvc)

    // 10. Set up routes
    router := gin.Default()
    
    // Add Prometheus middleware
    router.Use(middleware.PrometheusMiddleware())

    // Health and readiness endpoints
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{"status": "healthy"})
    })
    router.GET("/readiness", func(c *gin.Context) {
        // Check if dependencies are ready
        if err := rdb.Ping(context.Background()).Err(); err != nil {
            c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "redis unavailable"})
            return
        }
        if err := sqlDB.Ping(); err != nil {
            c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": "database unavailable"})
            return
        }
        c.JSON(http.StatusOK, gin.H{"status": "ready"})
    })

    // Metrics endpoint
    router.GET("/metrics", gin.WrapH(promhttp.Handler()))
    
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