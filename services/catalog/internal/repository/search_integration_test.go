package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/meilisearch"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
	"github.com/ichinosekei/highload/services/catalog/internal/repository"
)

func setupTestMeili(t *testing.T) *platform.MeiliClient {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	meiliContainer, err := meilisearch.Run(ctx, "getmeili/meilisearch:v1.12", meilisearch.WithMasterKey("test_key"))
	if err != nil {
		t.Fatalf("start meilisearch container: %v", err)
	}

	t.Cleanup(func() {
		if errTerm := meiliContainer.Terminate(context.Background()); errTerm != nil {
			t.Logf("terminate meilisearch container: %v", errTerm)
		}
	})

	meiliURL, err := meiliContainer.Address(ctx)
	if err != nil {
		t.Fatalf("get meilisearch connection string: %v", err)
	}

	client, err := platform.NewMeiliClient(meiliURL, "test_key")
	if err != nil {
		t.Fatalf("create meilisearch client: %v", err)
	}

	if err := client.InitIndices(); err != nil {
		t.Fatalf("init meilisearch indices: %v", err)
	}

	return client
}

func TestSearchRepository_Integration(t *testing.T) {
	client := setupTestMeili(t)
	repo := repository.NewSearchRepository(client)
	ctx := context.Background()

	restaurants := []domain.Restaurant{
		{
			ID:       uuid.New(),
			Name:     "Pizza Place",
			Cuisine:  []string{"italian"},
			Rating:   4.5,
			IsActive: true,
		},
		{
			ID:       uuid.New(),
			Name:     "Burger Joint",
			Cuisine:  []string{"american"},
			Rating:   4.0,
			IsActive: true,
		},
		{
			ID:       uuid.New(),
			Name:     "Hidden Sushi",
			Cuisine:  []string{"japanese"},
			Rating:   4.8,
			IsActive: false,
		},
	}

	err := repo.Sync(ctx, restaurants)
	if err != nil {
		t.Fatalf("Sync: %v", err)
	}

	t.Run("search all active", func(t *testing.T) {
		res, err := repo.Search(ctx, domain.SearchParams{Limit: 10})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}

		if res.Total < 2 {
			t.Errorf("got total %d; want at least 2", res.Total)
		}
	})

	t.Run("search by query", func(t *testing.T) {
		res, err := repo.Search(ctx, domain.SearchParams{Query: "pizza", Limit: 10})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}

		if len(res.Items) != 1 || res.Items[0].Name != "Pizza Place" {
			t.Errorf("got %v; want Pizza Place", res.Items)
		}
	})

	t.Run("search by cuisine", func(t *testing.T) {
		res, err := repo.Search(ctx, domain.SearchParams{Cuisine: "american", Limit: 10})
		if err != nil {
			t.Fatalf("Search: %v", err)
		}

		if len(res.Items) != 1 || res.Items[0].Name != "Burger Joint" {
			t.Errorf("got %v; want Burger Joint", res.Items)
		}
	})
}
