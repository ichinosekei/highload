package repository_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
	"github.com/ichinosekei/highload/services/catalog/internal/repository"
)

const testTimeout = 120 * time.Second

// setupTestDB starts a PostgreSQL container via testcontainers-go,
// creates the schema and seed data, and returns a connected [platform.PostgresDB].
// The container is automatically terminated when the test completes.
func setupTestDB(t *testing.T) *platform.PostgresDB {
	t.Helper()

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	t.Cleanup(cancel)

	_, currentFile, _, _ := runtime.Caller(0) //nolint:dogsled // need only file path
	testdataDir := filepath.Join(filepath.Dir(currentFile), "testdata")

	pgContainer, errContainer := postgres.Run(ctx,
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
	if errContainer != nil {
		t.Fatalf("start postgres container: %v", errContainer)
	}

	t.Cleanup(func() {
		if errTerminate := testcontainers.TerminateContainer(pgContainer); errTerminate != nil {
			t.Logf("terminate postgres container: %v", errTerminate)
		}
	})

	connStr, errConn := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if errConn != nil {
		t.Fatalf("get connection string: %v", errConn)
	}

	pool, errPool := pgxpool.New(ctx, connStr)
	if errPool != nil {
		t.Fatalf("connect to test database: %v", errPool)
	}
	t.Cleanup(pool.Close)

	return &platform.PostgresDB{Pool: pool}
}

// --- Test Data IDs ---

//nolint:gochecknoglobals // test data identifiers for integration tests
var (
	pizzaHouseID     = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901")
	sushiMasterID    = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d902")
	closedRestID     = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d904")
	margheritaItemID = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001")
)

// --- RestaurantRepository Integration Tests ---

//nolint:gocognit // complex integration test with many assertions
func TestRestaurantRepository_List(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewRestaurantRepository(db)
	ctx := context.Background()

	t.Run("returns only active restaurants", func(t *testing.T) {
		restaurants, err := repo.List(ctx, 100, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		// Seed has 3 active + 1 inactive → expect 3.
		if len(restaurants) != 3 {
			t.Fatalf("got %d restaurants; want 3 (only active)", len(restaurants))
		}

		for _, r := range restaurants {
			if !r.IsActive {
				t.Errorf("restaurant %s is not active", r.ID)
			}
		}
	})

	t.Run("ordered by rating DESC", func(t *testing.T) {
		restaurants, err := repo.List(ctx, 100, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		for i := 1; i < len(restaurants); i++ {
			if restaurants[i-1].Rating < restaurants[i].Rating {
				t.Errorf("restaurants not ordered by rating DESC at index %d: %.1f < %.1f",
					i, restaurants[i-1].Rating, restaurants[i].Rating)
			}
		}
	})

	t.Run("respects pagination limit", func(t *testing.T) {
		restaurants, err := repo.List(ctx, 1, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		if len(restaurants) != 1 {
			t.Errorf("got %d restaurants; want 1 (limit=1)", len(restaurants))
		}
	})

	t.Run("respects pagination offset", func(t *testing.T) {
		allRestaurants, errAll := repo.List(ctx, 100, 0)
		if errAll != nil {
			t.Fatalf("List all: %v", errAll)
		}

		offsetRestaurants, errOffset := repo.List(ctx, 100, 1)
		if errOffset != nil {
			t.Fatalf("List with offset: %v", errOffset)
		}

		if len(offsetRestaurants) != len(allRestaurants)-1 {
			t.Errorf("got %d restaurants with offset=1; want %d",
				len(offsetRestaurants), len(allRestaurants)-1)
		}
	})

	t.Run("scans all fields correctly", func(t *testing.T) {
		restaurants, err := repo.List(ctx, 100, 0)
		if err != nil {
			t.Fatalf("List: %v", err)
		}

		var pizzaHouse *domain.Restaurant
		for i := range restaurants {
			if restaurants[i].ID == pizzaHouseID {
				pizzaHouse = &restaurants[i]
				break
			}
		}
		if pizzaHouse == nil {
			t.Fatal("Pizza House not found in results")
		}

		if pizzaHouse.Name != "Pizza House" {
			t.Errorf("name = %q; want Pizza House", pizzaHouse.Name)
		}
		if len(pizzaHouse.Cuisine) != 2 {
			t.Errorf("cuisine len = %d; want 2", len(pizzaHouse.Cuisine))
		}
		if pizzaHouse.Rating != 4.7 {
			t.Errorf("rating = %.1f; want 4.7", pizzaHouse.Rating)
		}
		if pizzaHouse.DeliveryTimeMin != 30 {
			t.Errorf("delivery_time_min = %d; want 30", pizzaHouse.DeliveryTimeMin)
		}
		if pizzaHouse.DeliveryFee != 149 {
			t.Errorf("delivery_fee = %d; want 149", pizzaHouse.DeliveryFee)
		}
		if pizzaHouse.Address.Lat == 0 {
			t.Error("address lat is zero")
		}
		if pizzaHouse.ImageURL == "" {
			t.Error("image_url is empty")
		}
		if pizzaHouse.CreatedAt.IsZero() {
			t.Error("created_at is zero")
		}
	})
}

func TestRestaurantRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewRestaurantRepository(db)
	ctx := context.Background()

	t.Run("returns existing restaurant", func(t *testing.T) {
		restaurant, err := repo.GetByID(ctx, pizzaHouseID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}

		if restaurant.ID != pizzaHouseID {
			t.Errorf("id = %s; want %s", restaurant.ID, pizzaHouseID)
		}
		if restaurant.Name != "Pizza House" {
			t.Errorf("name = %q; want Pizza House", restaurant.Name)
		}
	})

	t.Run("returns not found for non-existent ID", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound in chain, got: %v", err)
		}
	})

	t.Run("returns inactive restaurant by ID", func(t *testing.T) {
		restaurant, err := repo.GetByID(ctx, closedRestID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if restaurant.IsActive {
			t.Error("expected inactive restaurant")
		}
	})
}

// --- MenuItemRepository Integration Tests ---

//nolint:gocognit // complex integration test with many assertions
func TestMenuItemRepository_ListByRestaurant(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewMenuItemRepository(db)
	ctx := context.Background()

	t.Run("returns only available items for restaurant", func(t *testing.T) {
		items, err := repo.ListByRestaurant(ctx, pizzaHouseID)
		if err != nil {
			t.Fatalf("ListByRestaurant: %v", err)
		}

		// Pizza House has 3 items in seed, but Тирамису is_available=false → 2.
		if len(items) != 2 {
			t.Fatalf("got %d items; want 2 (only available)", len(items))
		}

		for _, item := range items {
			if !item.IsAvailable {
				t.Errorf("item %s is not available", item.ID)
			}
			if item.RestaurantID != pizzaHouseID {
				t.Errorf("item restaurant_id = %s; want %s", item.RestaurantID, pizzaHouseID)
			}
		}
	})

	t.Run("returns items for restaurant with menu", func(t *testing.T) {
		items, err := repo.ListByRestaurant(ctx, sushiMasterID)
		if err != nil {
			t.Fatalf("ListByRestaurant: %v", err)
		}

		// Sushi Master has 2 available items.
		if len(items) != 2 {
			t.Fatalf("got %d items; want 2", len(items))
		}
	})

	t.Run("returns empty for non-existent restaurant", func(t *testing.T) {
		items, err := repo.ListByRestaurant(ctx, uuid.New())
		if err != nil {
			t.Fatalf("ListByRestaurant: %v", err)
		}
		if len(items) != 0 {
			t.Errorf("got %d items; want 0", len(items))
		}
	})

	t.Run("scans all fields correctly", func(t *testing.T) {
		items, err := repo.ListByRestaurant(ctx, pizzaHouseID)
		if err != nil {
			t.Fatalf("ListByRestaurant: %v", err)
		}

		var margherita *domain.MenuItem
		for i := range items {
			if items[i].ID == margheritaItemID {
				margherita = &items[i]
				break
			}
		}
		if margherita == nil {
			t.Fatal("Маргарита not found in results")
		}

		if margherita.Name != "Маргарита" {
			t.Errorf("name = %q; want Маргарита", margherita.Name)
		}
		if margherita.Price != 49900 {
			t.Errorf("price = %d; want 49900", margherita.Price)
		}
		if margherita.Category != "pizza" {
			t.Errorf("category = %q; want pizza", margherita.Category)
		}
		if len(margherita.ImageURLs) != 1 {
			t.Errorf("image_urls len = %d; want 1", len(margherita.ImageURLs))
		}
		if len(margherita.Options) != 2 {
			t.Errorf("options len = %d; want 2", len(margherita.Options))
		} else {
			if margherita.Options[0].Name != "extra_cheese" {
				t.Errorf("option name = %q; want extra_cheese", margherita.Options[0].Name)
			}
			if margherita.Options[0].Price != 5000 {
				t.Errorf("option price = %d; want 5000", margherita.Options[0].Price)
			}
		}
	})

	t.Run("ordered by category then name", func(t *testing.T) {
		items, err := repo.ListByRestaurant(ctx, pizzaHouseID)
		if err != nil {
			t.Fatalf("ListByRestaurant: %v", err)
		}

		for i := 1; i < len(items); i++ {
			if items[i-1].Category > items[i].Category {
				t.Errorf("items not ordered by category: %q > %q at index %d",
					items[i-1].Category, items[i].Category, i)
			}
		}
	})
}
