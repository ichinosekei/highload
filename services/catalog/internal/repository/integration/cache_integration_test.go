package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
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
	if err != nil {
		t.Fatalf("start redis container: %v", err)
	}

	t.Cleanup(func() {
		if errTerm := redisContainer.Terminate(ctx); errTerm != nil {
			t.Logf("terminate redis container: %v", errTerm)
		}
	})

	addr, err := redisContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("redis connection string: %v", err)
	}

	client, err := platform.NewRedisClient(ctx, addr)
	if err != nil {
		t.Fatalf("create redis client: %v", err)
	}

	return client
}

type mockSearcher struct {
	calls int
}

func (m *mockSearcher) Search(_ context.Context, _ domain.SearchParams) (*domain.SearchResult, error) {
	m.calls++
	return &domain.SearchResult{
		Items: []domain.Restaurant{{Name: "Mocked"}},
		Total: 1,
	}, nil
}

func TestSearchCacheDecorator_Integration(t *testing.T) {
	t.Parallel()
	rdb := setupTestRedis(t)
	ttl := 1 * time.Minute
	ctx := context.Background()

	t.Run("Caches results", func(t *testing.T) {
		t.Parallel()
		mock := &mockSearcher{}
		decorator := repository.NewSearchCacheDecorator(mock, rdb, ttl)
		params := domain.SearchParams{Query: "test_" + uuid.NewString()}

		// First call - should hit mock
		res1, err := decorator.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search 1: %v", err)
		}
		if mock.calls != 1 {
			t.Errorf("expected 1 call to mock, got %d", mock.calls)
		}

		// Second call - should hit cache
		res2, err := decorator.Search(ctx, params)
		if err != nil {
			t.Fatalf("Search 2: %v", err)
		}
		if mock.calls != 1 {
			t.Errorf("expected still 1 call to mock (cached), got %d", mock.calls)
		}

		if res1.Items[0].Name != res2.Items[0].Name {
			t.Error("cached result mismatch")
		}
	})
}

type mockMenuRestaurantReader struct {
	resCalls  int
	menuCalls int
}

func (m *mockMenuRestaurantReader) List(_ context.Context, _, _ int64) ([]domain.Restaurant, error) {
	m.resCalls++
	return nil, nil
}

func (m *mockMenuRestaurantReader) GetByID(_ context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	m.resCalls++
	return &domain.Restaurant{ID: id, Name: "Mocked Restaurant"}, nil
}

func (m *mockMenuRestaurantReader) ListByRestaurant(_ context.Context, _ uuid.UUID) ([]domain.MenuItem, error) {
	m.menuCalls++
	return []domain.MenuItem{{Name: "Mocked Item"}}, nil
}

//nolint:gocognit // complex integration test
func TestRestaurantCacheDecorator_Integration(t *testing.T) {
	t.Parallel()
	rdb := setupTestRedis(t)
	ttl := 1 * time.Minute
	ctx := context.Background()

	t.Run("Caches GetByID", func(t *testing.T) {
		t.Parallel()
		id := uuid.New()
		mock := &mockMenuRestaurantReader{}
		decorator := repository.NewRestaurantCacheDecorator(mock, rdb, ttl)

		// First call - should hit mock
		_, err := decorator.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("GetByID 1: %v", err)
		}
		if mock.resCalls != 1 {
			t.Errorf("expected 1 call, got %d", mock.resCalls)
		}

		// Second call - should hit cache
		_, err = decorator.GetByID(ctx, id)
		if err != nil {
			t.Fatalf("GetByID 2: %v", err)
		}
		if mock.resCalls != 1 {
			t.Errorf("expected still 1 call (cached), got %d", mock.resCalls)
		}
	})

	t.Run("No caches List", func(t *testing.T) {
		t.Parallel()
		mock := &mockMenuRestaurantReader{}
		decorator := repository.NewRestaurantCacheDecorator(mock, rdb, ttl)
		limit, offset := int64(50), int64(0)

		// First call - should hit mock
		_, err := decorator.List(ctx, limit, offset)
		if err != nil {
			t.Fatalf("List 1: %v", err)
		}
		if mock.resCalls != 1 {
			t.Errorf("expected 1 call, got %d", mock.resCalls)
		}

		// Second call - should hit mock again (no cache for List)
		_, err = decorator.List(ctx, limit, offset)
		if err != nil {
			t.Fatalf("List 2: %v", err)
		}
		if mock.resCalls != 2 {
			t.Errorf("expected 2 calls total (not cached), got %d", mock.resCalls)
		}
	})

	t.Run("Caches ListByRestaurant", func(t *testing.T) {
		t.Parallel()
		id := uuid.New()
		mock := &mockMenuRestaurantReader{}
		decorator := repository.NewRestaurantCacheDecorator(mock, rdb, ttl)

		// First call - should hit mock
		_, err := decorator.ListByRestaurant(ctx, id)
		if err != nil {
			t.Fatalf("ListByRestaurant 1: %v", err)
		}
		if mock.menuCalls != 1 {
			t.Errorf("expected 1 call, got %d", mock.menuCalls)
		}

		// Second call - should hit cache
		_, err = decorator.ListByRestaurant(ctx, id)
		if err != nil {
			t.Fatalf("ListByRestaurant 2: %v", err)
		}
		if mock.menuCalls != 1 {
			t.Errorf("expected still 1 call (cached), got %d", mock.menuCalls)
		}
	})
}
