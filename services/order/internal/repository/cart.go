package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/ichinosekei/highload/services/order/internal/domain"
	"github.com/ichinosekei/highload/services/order/internal/platform"
)

const cartTTL = 24 * time.Hour

type CartRepository struct {
	rdb *platform.RedisClient
}

func NewCartRepository(rdb *platform.RedisClient) *CartRepository {
	return &CartRepository{rdb: rdb}
}

func (r *CartRepository) Get(ctx context.Context, userID uuid.UUID) ([]domain.OrderItem, error) {
	key := fmt.Sprintf("cart:%s", userID)

	val, err := r.rdb.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return []domain.OrderItem{}, nil
		}
		return nil, fmt.Errorf("get cart: %w", err)
	}

	var items []domain.OrderItem
	if errJSON := json.Unmarshal([]byte(val), &items); errJSON != nil {
		return nil, fmt.Errorf("unmarshal cart: %w", errJSON)
	}

	return items, nil
}

func (r *CartRepository) Set(ctx context.Context, userID uuid.UUID, items []domain.OrderItem) error {
	key := fmt.Sprintf("cart:%s", userID)

	data, err := json.Marshal(items)
	if err != nil {
		return fmt.Errorf("marshal cart: %w", err)
	}

	if errSet := r.rdb.Set(ctx, key, data, cartTTL).Err(); errSet != nil {
		return fmt.Errorf("set cart: %w", errSet)
	}

	return nil
}

func (r *CartRepository) Delete(ctx context.Context, userID uuid.UUID) error {
	key := fmt.Sprintf("cart:%s", userID)

	if err := r.rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("delete cart: %w", err)
	}

	return nil
}
