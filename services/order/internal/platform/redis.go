package platform

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type RedisClient struct {
	*redis.Client
}

func NewRedisClient(ctx context.Context, addr, password string) (*RedisClient, error) {
	var opts *redis.Options
	var err error

	if len(addr) > 8 && addr[:8] == "redis://" {
		opts, err = redis.ParseURL(addr)
		if err != nil {
			return nil, fmt.Errorf("invalid redis url: %w", err)
		}
	} else {
		opts = &redis.Options{
			Addr:     addr,
			Password: password,
			DB:       0,
		}
	}

	rdb := redis.NewClient(opts)

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis connection: %w", err)
	}

	return &RedisClient{rdb}, nil
}
