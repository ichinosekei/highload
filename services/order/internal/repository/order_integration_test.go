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

	"github.com/ichinosekei/highload/services/order/internal/domain"
	"github.com/ichinosekei/highload/services/order/internal/platform"
	"github.com/ichinosekei/highload/services/order/internal/repository"
)

const testTimeout = 120 * time.Second

// setupTestDB starts a PostgreSQL container via testcontainers-go,
// creates the schema and seed data, and returns a connected [platform.PostgresDB].
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
		postgres.WithDatabase("orders_test"),
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
	testOrderID1      = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-c4f3b6c8d001")
	testOrderID2      = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-c4f3b6c8d002")
	testUserID1       = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-d4f3b6c8d001")
	testRestaurantID1 = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901")
	testIdempotency1  = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-e4f3b6c8d001")
)

// --- OrderRepository Integration Tests ---

func TestOrderRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewOrderRepository(db)
	ctx := context.Background()

	t.Run("creates order successfully", func(t *testing.T) {
		order := &domain.Order{
			ID:              uuid.New(),
			UserID:          uuid.New(),
			RestaurantID:    testRestaurantID1,
			Status:          domain.StatusCreated,
			Items:           []domain.OrderItem{{MenuItemID: uuid.New(), Quantity: 1}},
			TotalAmount:     50000,
			DeliveryFee:     149,
			DeliveryAddress: domain.DeliveryAddress{AddressText: "Test", Lat: 55.75, Lon: 37.62},
			Comment:         "test comment",
			IdempotencyKey:  uuid.New(),
		}

		err := repo.Create(ctx, order)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		// Verify it's persisted.
		got, errGet := repo.GetByID(ctx, order.ID)
		if errGet != nil {
			t.Fatalf("GetByID after Create: %v", errGet)
		}
		if got.TotalAmount != 50000 {
			t.Errorf("total_amount = %d; want 50000", got.TotalAmount)
		}
		if got.Status != domain.StatusCreated {
			t.Errorf("status = %q; want %q", got.Status, domain.StatusCreated)
		}
	})

	t.Run("rejects duplicate idempotency key", func(t *testing.T) {
		order := &domain.Order{
			ID:              uuid.New(),
			UserID:          uuid.New(),
			RestaurantID:    testRestaurantID1,
			Status:          domain.StatusCreated,
			Items:           []domain.OrderItem{{MenuItemID: uuid.New(), Quantity: 1}},
			TotalAmount:     30000,
			DeliveryFee:     100,
			DeliveryAddress: domain.DeliveryAddress{AddressText: "Dup", Lat: 55.75, Lon: 37.62},
			IdempotencyKey:  testIdempotency1, // Same as seed data
		}

		err := repo.Create(ctx, order)
		if err == nil {
			t.Fatal("expected error for duplicate idempotency key, got nil")
		}
		if !errors.Is(err, domain.ErrIdempotencyConflict) {
			t.Errorf("expected ErrIdempotencyConflict in chain, got: %v", err)
		}
	})

	t.Run("creates outbox event in same transaction", func(t *testing.T) {
		orderID := uuid.New()
		order := &domain.Order{
			ID:              orderID,
			UserID:          uuid.New(),
			RestaurantID:    testRestaurantID1,
			Status:          domain.StatusCreated,
			Items:           []domain.OrderItem{{MenuItemID: uuid.New(), Quantity: 2}},
			TotalAmount:     80000,
			DeliveryFee:     200,
			DeliveryAddress: domain.DeliveryAddress{AddressText: "Outbox test", Lat: 55.75, Lon: 37.62},
			IdempotencyKey:  uuid.New(),
		}

		err := repo.Create(ctx, order)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		// Verify outbox event was created.
		var count int
		errQuery := db.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = $1 AND event_type = $2",
			orderID, domain.EventOrderCreated,
		).Scan(&count)
		if errQuery != nil {
			t.Fatalf("query outbox: %v", errQuery)
		}
		if count != 1 {
			t.Errorf("outbox event count = %d; want 1", count)
		}
	})
}

func TestOrderRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewOrderRepository(db)
	ctx := context.Background()

	t.Run("returns existing order", func(t *testing.T) {
		order, err := repo.GetByID(ctx, testOrderID1)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}

		if order.ID != testOrderID1 {
			t.Errorf("id = %s; want %s", order.ID, testOrderID1)
		}
		if order.Status != domain.StatusCreated {
			t.Errorf("status = %q; want %q", order.Status, domain.StatusCreated)
		}
		if order.TotalAmount != 159700 {
			t.Errorf("total_amount = %d; want 159700", order.TotalAmount)
		}
		if order.UserID != testUserID1 {
			t.Errorf("user_id = %s; want %s", order.UserID, testUserID1)
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
}

func TestOrderRepository_GetByIdempotencyKey(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewOrderRepository(db)
	ctx := context.Background()

	t.Run("finds order by idempotency key", func(t *testing.T) {
		order, err := repo.GetByIdempotencyKey(ctx, testIdempotency1)
		if err != nil {
			t.Fatalf("GetByIdempotencyKey: %v", err)
		}
		if order.ID != testOrderID1 {
			t.Errorf("id = %s; want %s", order.ID, testOrderID1)
		}
	})

	t.Run("returns not found for unknown key", func(t *testing.T) {
		_, err := repo.GetByIdempotencyKey(ctx, uuid.New())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}

func TestOrderRepository_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewOrderRepository(db)
	ctx := context.Background()

	t.Run("updates status successfully", func(t *testing.T) {
		// Order #2 is in "cooking" status; advance to "ready".
		err := repo.UpdateStatus(ctx, testOrderID2, domain.StatusReady)
		if err != nil {
			t.Fatalf("UpdateStatus: %v", err)
		}

		order, errGet := repo.GetByID(ctx, testOrderID2)
		if errGet != nil {
			t.Fatalf("GetByID: %v", errGet)
		}
		if order.Status != domain.StatusReady {
			t.Errorf("status = %q; want %q", order.Status, domain.StatusReady)
		}
	})

	t.Run("returns not found for non-existent order", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, uuid.New(), domain.StatusCooking)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}
