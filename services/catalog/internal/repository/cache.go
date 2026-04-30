package repository

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
)

// SearchCacheDecorator wraps a searcher with Redis caching.
type SearchCacheDecorator struct {
	base domain.RestaurantSearcher
	rdb  *platform.RedisClient
	ttl  time.Duration
}

func NewSearchCacheDecorator(
	base domain.RestaurantSearcher,
	rdb *platform.RedisClient,
	ttl time.Duration,
) *SearchCacheDecorator {
	return &SearchCacheDecorator{base: base, rdb: rdb, ttl: ttl}
}

func (d *SearchCacheDecorator) Search(ctx context.Context, params domain.SearchParams) (*domain.SearchResult, error) {
	key := fmt.Sprintf("catalog:search:%x", hashSearchParams(params))

	// Try cache
	val, errGet := d.rdb.Get(ctx, key).Result()
	if errGet == nil {
		var res domain.SearchResult
		if err := json.Unmarshal([]byte(val), &res); err == nil {
			return &res, nil
		}
	}

	// Fallback to base
	res, errSearch := d.base.Search(ctx, params)
	if errSearch != nil {
		return nil, errSearch
	}

	// Save to cache
	if data, err := json.Marshal(res); err == nil {
		_ = d.rdb.Set(ctx, key, data, d.ttl).Err()
	}

	return res, nil
}

// RestaurantCacheDecorator wraps restaurant and menu readers with Redis caching.
type RestaurantCacheDecorator struct {
	resReader  domain.RestaurantReader
	menuReader domain.MenuItemReader
	rdb        *platform.RedisClient
	ttl        time.Duration
}

func NewRestaurantCacheDecorator(
	resReader domain.RestaurantReader,
	menuReader domain.MenuItemReader,
	rdb *platform.RedisClient,
	ttl time.Duration,
) *RestaurantCacheDecorator {
	return &RestaurantCacheDecorator{
		resReader:  resReader,
		menuReader: menuReader,
		rdb:        rdb,
		ttl:        ttl,
	}
}

func (d *RestaurantCacheDecorator) List(ctx context.Context, limit, offset int64) ([]domain.Restaurant, error) {
	// Usually we don't cache lists with high cardinality or pagination unless necessary.
	// For this PoC, we'll delegate to base reader.
	return d.resReader.List(ctx, limit, offset)
}

func (d *RestaurantCacheDecorator) GetByID(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	key := fmt.Sprintf("catalog:restaurant:%s", id)

	val, errGet := d.rdb.Get(ctx, key).Result()
	if errGet == nil {
		var res domain.Restaurant
		if err := json.Unmarshal([]byte(val), &res); err == nil {
			return &res, nil
		}
	}

	res, errGetByID := d.resReader.GetByID(ctx, id)
	if errGetByID != nil {
		return nil, errGetByID
	}

	if data, err := json.Marshal(res); err == nil {
		_ = d.rdb.Set(ctx, key, data, d.ttl).Err()
	}

	return res, nil
}

func (d *RestaurantCacheDecorator) ListByRestaurant(
	ctx context.Context,
	restaurantID uuid.UUID,
) ([]domain.MenuItem, error) {
	key := fmt.Sprintf("catalog:menu:%s", restaurantID)

	val, errGet := d.rdb.Get(ctx, key).Result()
	if errGet == nil {
		var res []domain.MenuItem
		if err := json.Unmarshal([]byte(val), &res); err == nil {
			return res, nil
		}
	}

	res, errList := d.menuReader.ListByRestaurant(ctx, restaurantID)
	if errList != nil {
		return nil, errList
	}

	if data, err := json.Marshal(res); err == nil {
		_ = d.rdb.Set(ctx, key, data, d.ttl).Err()
	}

	return res, nil
}

func hashSearchParams(p domain.SearchParams) [32]byte {
	return sha256.Sum256([]byte(fmt.Sprintf("%v", p)))
}
