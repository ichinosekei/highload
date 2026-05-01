package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/meilisearch"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
	"github.com/ichinosekei/highload/services/catalog/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestMeili(t *testing.T) *platform.MeiliClient {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	meiliContainer, err := meilisearch.Run(ctx, "getmeili/meilisearch:v1.12", meilisearch.WithMasterKey("test_key"))
	require.NoError(t, err, "start meilisearch container")

	t.Cleanup(func() {
		if errTerm := meiliContainer.Terminate(context.Background()); errTerm != nil {
			t.Logf("terminate meilisearch container: %v", errTerm)
		}
	})

	meiliURL, err := meiliContainer.Address(ctx)
	require.NoError(t, err, "get meilisearch connection string")

	client, err := platform.NewMeiliClient(meiliURL, "test_key")
	require.NoError(t, err, "create meilisearch client")

	err = client.InitIndices(ctx)
	require.NoError(t, err, "init meilisearch indices")

	return client
}

func TestSearchRepository_Integration(t *testing.T) {
	t.Parallel()
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
	require.NoError(t, err)

	t.Run("search all active", func(t *testing.T) {
		t.Parallel()
		res, err := repo.Search(ctx, domain.SearchParams{Limit: 10})
		require.NoError(t, err)

		assert.GreaterOrEqual(t, res.Total, int64(2))
	})

	t.Run("search by query", func(t *testing.T) {
		t.Parallel()
		res, err := repo.Search(ctx, domain.SearchParams{Query: "pizza", Limit: 10})
		require.NoError(t, err)

		require.Len(t, res.Items, 1)
		assert.Equal(t, "Pizza Place", res.Items[0].Name)
	})

	t.Run("search by cuisine", func(t *testing.T) {
		t.Parallel()
		res, err := repo.Search(ctx, domain.SearchParams{Cuisine: "american", Limit: 10})
		require.NoError(t, err)

		require.Len(t, res.Items, 1)
		assert.Equal(t, "Burger Joint", res.Items[0].Name)
	})
}
