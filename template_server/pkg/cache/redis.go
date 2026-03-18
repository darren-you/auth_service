package cache

import (
	"fmt"
	"time"

	"github.com/darren-you/auth_service/template_server/internal/config"
	"github.com/go-redis/redis/v8"
)

func InitRedis(cfg *config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: max(cfg.PoolSize/2, 1),
		MaxRetries:   3,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolTimeout:  4 * time.Second,
	})

	if _, err := client.Ping(client.Context()).Result(); err != nil {
		return nil, fmt.Errorf("failed to connect redis: %w", err)
	}
	return client, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
