package metrics

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

// StartDBPoolCollector starts a background goroutine that collects DB connection pool stats
func StartDBPoolCollector(sqlDB *sql.DB, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			stats := sqlDB.Stats()
			DBConnectionPoolSize.WithLabelValues("idle").Set(float64(stats.Idle))
			DBConnectionPoolSize.WithLabelValues("in_use").Set(float64(stats.InUse))
		}
	}()
}

// StartRedisQueueDepthCollector starts a background goroutine that monitors Redis queue depth
func StartRedisQueueDepthCollector(redisClient *redis.Client, queueKey string, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		ctx := context.Background()

		for range ticker.C {
			length, err := redisClient.LLen(ctx, queueKey).Result()
			if err != nil {
				log.Printf("Failed to get queue depth: %v", err)
				RedisConnectionErrorsTotal.WithLabelValues("llen").Inc()
				continue
			}
			RedisQueueDepth.Set(float64(length))
		}
	}()
}
