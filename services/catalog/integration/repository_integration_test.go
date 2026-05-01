package integration_test

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
	"github.com/ichinosekei/highload/services/catalog/internal/repository"
)

const testTimeout = 120 * time.Second

func setupTestDB(t *testing.T) *platform.PostgresDB {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	_, currentFile, _, _ := runtime.Caller(0)
	testdataDir := filepath.Join(filepath.Dir(currentFile), "testdata")

	pgContainer, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("catalog_test"),
		postgres.WithUsername("test_user"),
		postgres.WithPassword("test_password"),
		postgres.WithInitScripts(
			filepath.Join(testdataDir, "schema.sql"),
			filepath.Join(testdataDir, "seed.sql"),
		),
		postgres.WithSQLDriver("pgx"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second)),
	)
	require.NoError(t, err, "start postgres container")

	t.Cleanup(func() {
		if errTerm := testcontainers.TerminateContainer(pgContainer); errTerm != nil {
			t.Logf("terminate postgres container: %v", errTerm)
		}
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err, "get connection string")

	db, err := platform.NewPostgresDB(ctx, connStr)
	require.NoError(t, err, "create postgres client")
	t.Cleanup(db.Close)

	return db
}

// --- Test Data IDs ---

//nolint:gochecknoglobals // test data identifiers
var (
	pizzaHouseID     = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901")
	sushiMasterID    = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d902")
	closedRestID     = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d904")
	margheritaItemID = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001")
)

// --- RestaurantRepository Integration Tests ---

func TestIntegration_RestaurantRepository_List(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewRestaurantRepository(db)
	ctx := context.Background()

	t.Run("returns only active restaurants", func(t *testing.T) {
		t.Parallel()
		restaurants, err := repo.List(ctx, 100, 0)
		require.NoError(t, err)

		// Seed has 3 active + 1 inactive → expect 3.
		assert.Len(t, restaurants, 3)

		for _, r := range restaurants {
			assert.True(t, r.IsActive, "restaurant %s should be active", r.ID)
		}
	})

	t.Run("ordered by rating DESC", func(t *testing.T) {
		t.Parallel()
		restaurants, err := repo.List(ctx, 100, 0)
		require.NoError(t, err)

		for i := 1; i < len(restaurants); i++ {
			assert.GreaterOrEqual(t, restaurants[i-1].Rating, restaurants[i].Rating, "at index %d", i)
		}
	})

	t.Run("respects pagination limit", func(t *testing.T) {
		t.Parallel()
		restaurants, err := repo.List(ctx, 1, 0)
		require.NoError(t, err)

		assert.Len(t, restaurants, 1)
	})

	t.Run("respects pagination offset", func(t *testing.T) {
		t.Parallel()
		allRestaurants, err := repo.List(ctx, 100, 0)
		require.NoError(t, err)

		offsetRestaurants, err := repo.List(ctx, 100, 1)
		require.NoError(t, err)

		assert.Len(t, offsetRestaurants, len(allRestaurants)-1)
	})

	t.Run("scans all fields correctly", func(t *testing.T) {
		t.Parallel()
		restaurants, err := repo.List(ctx, 100, 0)
		require.NoError(t, err)

		var pizzaHouse *domain.Restaurant
		for i := range restaurants {
			if restaurants[i].ID == pizzaHouseID {
				pizzaHouse = &restaurants[i]
				break
			}
		}
		require.NotNil(t, pizzaHouse, "Pizza House not found in results")

		assert.Equal(t, "Pizza House", pizzaHouse.Name)
		assert.Len(t, pizzaHouse.Cuisine, 2)
		assert.InDelta(t, 4.7, pizzaHouse.Rating, 0.001)
		assert.Equal(t, 30, pizzaHouse.DeliveryTimeMin)
		assert.Equal(t, 149, pizzaHouse.DeliveryFee)
		assert.NotEmpty(t, pizzaHouse.Address.Lat)
		assert.NotEmpty(t, pizzaHouse.ImageURL)
		assert.False(t, pizzaHouse.CreatedAt.IsZero())
	})
}

func TestIntegration_RestaurantRepository_GetByID(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewRestaurantRepository(db)
	ctx := context.Background()

	t.Run("returns existing restaurant", func(t *testing.T) {
		t.Parallel()
		restaurant, err := repo.GetByID(ctx, pizzaHouseID)
		require.NoError(t, err)

		assert.Equal(t, pizzaHouseID, restaurant.ID)
		assert.Equal(t, "Pizza House", restaurant.Name)
	})

	t.Run("returns not found for non-existent ID", func(t *testing.T) {
		t.Parallel()
		_, err := repo.GetByID(ctx, uuid.New())
		require.Error(t, err)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("returns inactive restaurant by ID", func(t *testing.T) {
		t.Parallel()
		restaurant, err := repo.GetByID(ctx, closedRestID)
		require.NoError(t, err)
		assert.False(t, restaurant.IsActive)
	})
}

// --- MenuRepository Integration Tests ---

func TestIntegration_MenuRepository_ListByRestaurant(t *testing.T) {
	t.Parallel()
	db := setupTestDB(t)
	repo := repository.NewMenuRepository(db)
	ctx := context.Background()

	t.Run("returns only available items for restaurant", func(t *testing.T) {
		t.Parallel()
		items, err := repo.ListByRestaurant(ctx, pizzaHouseID)
		require.NoError(t, err)

		// Pizza House has 3 items in seed, but Tiramisu is_available=false → 2.
		assert.Len(t, items, 2)

		for _, item := range items {
			assert.True(t, item.IsAvailable)
			assert.Equal(t, pizzaHouseID, item.RestaurantID)
		}
	})

	t.Run("returns items for restaurant with menu", func(t *testing.T) {
		t.Parallel()
		items, err := repo.ListByRestaurant(ctx, sushiMasterID)
		require.NoError(t, err)

		// Sushi Master has 2 available items.
		assert.Len(t, items, 2)
	})

	t.Run("returns empty for non-existent restaurant", func(t *testing.T) {
		t.Parallel()
		items, err := repo.ListByRestaurant(ctx, uuid.New())
		require.NoError(t, err)
		assert.Empty(t, items)
	})

	t.Run("scans all fields correctly", func(t *testing.T) {
		t.Parallel()
		items, err := repo.ListByRestaurant(ctx, pizzaHouseID)
		require.NoError(t, err)

		var margherita *domain.MenuItem
		for i := range items {
			if items[i].ID == margheritaItemID {
				margherita = &items[i]
				break
			}
		}
		require.NotNil(t, margherita, "Маргарита not found in results")

		assert.Equal(t, "Маргарита", margherita.Name)
		assert.Equal(t, 49900, margherita.Price)
		assert.Equal(t, "pizza", margherita.Category)
		assert.Len(t, margherita.ImageURLs, 1)
		require.Len(t, margherita.Options, 2)
		assert.Equal(t, "extra_cheese", margherita.Options[0].Name)
		assert.Equal(t, 5000, margherita.Options[0].Price)
	})

	t.Run("ordered by category then name", func(t *testing.T) {
		t.Parallel()
		items, err := repo.ListByRestaurant(ctx, pizzaHouseID)
		require.NoError(t, err)

		for i := 1; i < len(items); i++ {
			assert.LessOrEqual(t, items[i-1].Category, items[i].Category, "at index %d", i)
		}
	})
}
