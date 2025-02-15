package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"time"
)

func IntClient(ctx context.Context, addr string) (*redis.Client, error) {
	redisClient := redis.NewClient(&redis.Options{
		Addr: addr,
		DB:   0,
	})

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return redisClient, nil
}
