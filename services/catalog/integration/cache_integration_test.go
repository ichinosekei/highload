package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
	"github.com/ichinosekei/highload/services/catalog/internal/repository"
)

func setupTestRedis(t *testing.T) *platform.RedisClient {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()
	redisContainer, err := redis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err, "start redis container")

	t.Cleanup(func() {
		if errTerm := redisContainer.Terminate(ctx); errTerm != nil {
			t.Logf("terminate redis container: %v", errTerm)
		}
	})

	addr, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err, "redis connection string")

	client, err := platform.NewRedisClient(ctx, addr, "")
	require.NoError(t, err, "create redis client")

	return client
}

func TestSearchCacheDecorator_Integration(t *testing.T) {
	t.Parallel()
	rdb := setupTestRedis(t)
	ttl := 1 * time.Minute
	ctx := context.Background()

	t.Run("Caches results", func(t *testing.T) {
		t.Parallel()
		mockSearcher := new(domain.MockRestaurantSearcher)
		decorator := repository.NewSearchCacheDecorator(mockSearcher, rdb, ttl)
		params := domain.SearchParams{Query: "test_" + uuid.NewString()}

		mockSearcher.
			On("Search", mock.Anything, params).
			Return(&domain.SearchResult{
				Items: []domain.Restaurant{{Name: "Mocked"}},
				Total: 1,
			}, nil).
			Once()

		// First call - should hit mock
		res1, err := decorator.Search(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, "Mocked", res1.Items[0].Name)

		// Second call - should hit cache
		res2, err := decorator.Search(ctx, params)
		require.NoError(t, err)
		assert.Equal(t, "Mocked", res2.Items[0].Name)

		mockSearcher.AssertExpectations(t)
	})
}

func TestRestaurantCacheDecorator_Integration(t *testing.T) {
	t.Parallel()
	rdb := setupTestRedis(t)
	ttl := 1 * time.Minute
	ctx := context.Background()

	t.Run("Caches GetByID", func(t *testing.T) {
		t.Parallel()
		id := uuid.New()
		mockReader := new(domain.MockRestaurantReader)
		decorator := repository.NewRestaurantCacheDecorator(&domain.MockMenuRestaurantReader{
			MockRestaurantReader: mockReader,
		}, rdb, ttl)

		mockReader.
			On("GetByID", mock.Anything, id).
			Return(&domain.Restaurant{ID: id, Name: "Mocked Restaurant"}, nil).
			Once()

		// First call - should hit mock
		_, err := decorator.GetByID(ctx, id)
		require.NoError(t, err)

		// Second call - should hit cache
		_, err = decorator.GetByID(ctx, id)
		require.NoError(t, err)

		mockReader.AssertExpectations(t)
	})

	t.Run("No caches List", func(t *testing.T) {
		t.Parallel()
		mockReader := new(domain.MockRestaurantReader)
		decorator := repository.NewRestaurantCacheDecorator(&domain.MockMenuRestaurantReader{
			MockRestaurantReader: mockReader,
		}, rdb, ttl)
		limit, offset := int64(50), int64(0)

		mockReader.
			On("List", mock.Anything, limit, offset).
			Return([]domain.Restaurant{}, nil).
			Twice()

		// First call - should hit mock
		_, err := decorator.List(ctx, limit, offset)
		require.NoError(t, err)

		// Second call - should hit mock again (no cache for List)
		_, err = decorator.List(ctx, limit, offset)
		require.NoError(t, err)

		mockReader.AssertExpectations(t)
	})

	t.Run("Caches ListByRestaurant", func(t *testing.T) {
		t.Parallel()
		id := uuid.New()
		mockMenuReader := new(domain.MockMenuReader)
		decorator := repository.NewRestaurantCacheDecorator(&domain.MockMenuRestaurantReader{
			MockMenuReader: mockMenuReader,
		}, rdb, ttl)

		mockMenuReader.
			On("ListByRestaurant", mock.Anything, id).
			Return([]domain.MenuItem{{Name: "Mocked Item"}}, nil).
			Once()

		// First call - should hit mock
		_, err := decorator.ListByRestaurant(ctx, id)
		require.NoError(t, err)

		// Second call - should hit cache
		_, err = decorator.ListByRestaurant(ctx, id)
		require.NoError(t, err)

		mockMenuReader.AssertExpectations(t)
	})
}
