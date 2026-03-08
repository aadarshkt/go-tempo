package redis

import (
	"context"
	"go-tempo/internal/metrics"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisQueue struct {
	client    *redis.Client
	queueName string
}

func NewRedisQueue(client *redis.Client) *RedisQueue {
	return &RedisQueue{
		client:    client,
		queueName: "workflow:queue:pending",
	}
}

// Push adds a task ID to the end of the list
func (q *RedisQueue) Push(ctx context.Context, taskID string) error {
	err := q.client.RPush(ctx, q.queueName, taskID).Err()
	if err != nil {
		metrics.RedisQueuePushTotal.WithLabelValues("error").Inc()
		metrics.RedisConnectionErrorsTotal.WithLabelValues("push").Inc()
		return err
	}
	metrics.RedisQueuePushTotal.WithLabelValues("success").Inc()
	return nil
}

// Pop waits for a task ID and removes it from the front of the list
func (q *RedisQueue) Pop(ctx context.Context) (string, error) {
	start := time.Now()
	defer func() {
		metrics.RedisQueuePopDuration.Observe(time.Since(start).Seconds())
	}()
	
	// 0 means "Wait forever until an item appears"
	result, err := q.client.BLPop(ctx, 0*time.Second, q.queueName).Result()
	if err != nil {
		metrics.RedisConnectionErrorsTotal.WithLabelValues("pop").Inc()
		return "", err
	}
	// BLPop returns a slice: [QueueName, Element]
	return result[1], nil 
}