package redis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

func NewRedisClient(address string) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     address, // e.g., "localhost:6379"
		PoolSize: 100,     // Maximum number of socket connections
	})
	
	// Ping to test connection on startup
	if err := client.Ping(context.Background()).Err(); err != nil {
		panic("failed to connect to redis: " + err.Error())
	}
	
	return client
}