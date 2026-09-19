package db

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

func ConnectRedis(ctx context.Context, url string) (*redis.Client, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("redis url: %w", err)
	}
	opts.PoolSize = 30
	opts.ReadTimeout = 0 // pub/sub subscribers block indefinitely by design

	client := redis.NewClient(opts)

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return client, nil
}
