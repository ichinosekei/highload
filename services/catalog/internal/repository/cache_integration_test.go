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

	addr, _ := redisContainer.ConnectionString(ctx)
	client, err := platform.NewRedisClient(ctx, addr, "")
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
	rdb := setupTestRedis(t)
	mock := &mockSearcher{}
	ttl := 1 * time.Minute
	decorator := repository.NewSearchCacheDecorator(mock, rdb, ttl)
	ctx := context.Background()
	params := domain.SearchParams{Query: "test"}

	t.Run("Caches results", func(t *testing.T) {
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

type mockRestaurantReader struct {
	calls int
}

func (m *mockRestaurantReader) List(_ context.Context, _, _ int64) ([]domain.Restaurant, error) {
	return nil, nil
}

func (m *mockRestaurantReader) GetByID(_ context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	m.calls++
	return &domain.Restaurant{ID: id, Name: "Mocked Restaurant"}, nil
}

type mockMenuReader struct {
	calls int
}

func (m *mockMenuReader) ListByRestaurant(_ context.Context, _ uuid.UUID) ([]domain.MenuItem, error) {
	m.calls++
	return []domain.MenuItem{{Name: "Mocked Item"}}, nil
}

func TestRestaurantCacheDecorator_Integration(t *testing.T) {
	rdb := setupTestRedis(t)
	mockRes := &mockRestaurantReader{}
	mockMenu := &mockMenuReader{}
	ttl := 1 * time.Minute
	decorator := repository.NewRestaurantCacheDecorator(mockRes, mockMenu, rdb, ttl)
	ctx := context.Background()
	id := uuid.New()

	t.Run("Caches GetByID", func(t *testing.T) {
		_, _ = decorator.GetByID(ctx, id)
		if mockRes.calls != 1 {
			t.Errorf("expected 1 call, got %d", mockRes.calls)
		}

		_, _ = decorator.GetByID(ctx, id)
		if mockRes.calls != 1 {
			t.Errorf("expected still 1 call (cached), got %d", mockRes.calls)
		}
	})

	t.Run("Caches ListByRestaurant", func(t *testing.T) {
		_, _ = decorator.ListByRestaurant(ctx, id)
		if mockMenu.calls != 1 {
			t.Errorf("expected 1 call, got %d", mockMenu.calls)
		}

		_, _ = decorator.ListByRestaurant(ctx, id)
		if mockMenu.calls != 1 {
			t.Errorf("expected still 1 call (cached), got %d", mockMenu.calls)
		}
	})
}
