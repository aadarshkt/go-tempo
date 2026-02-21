package redis

import (
	"context"
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
	return q.client.RPush(ctx, q.queueName, taskID).Err()
}

// Pop waits for a task ID and removes it from the front of the list
func (q *RedisQueue) Pop(ctx context.Context) (string, error) {
	// 0 means "Wait forever until an item appears"
	result, err := q.client.BLPop(ctx, 0*time.Second, q.queueName).Result()
	if err != nil {
		return "", err
	}
	// BLPop returns a slice: [QueueName, Element]
	return result[1], nil 
}