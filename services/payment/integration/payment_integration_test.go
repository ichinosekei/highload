package integration_test

import (
	"context"
	"errors"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/ichinosekei/highload/services/payment/internal/domain"
	"github.com/ichinosekei/highload/services/payment/internal/platform"
	"github.com/ichinosekei/highload/services/payment/internal/repository"
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

	pgContainer, errContainer := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("payments_test"),
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

//nolint:gochecknoglobals // test data identifiers
var (
	testOrderID1     = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-c4f3b6c8d001")
	testOrderID2     = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-c4f3b6c8d002")
	testPaymentID1   = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-f4f3b6c8d001")
	testIdempotency1 = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-f4f3b6c8e001")
)

// --- PaymentRepository Integration Tests ---

func TestIntegration_PaymentRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewPaymentRepository(db)
	ctx := context.Background()

	t.Run("creates payment successfully", func(t *testing.T) {
		payment := &domain.Payment{
			ID:             uuid.New(),
			OrderID:        testOrderID2,
			Status:         domain.PaymentStatusProcessing,
			Amount:         50000,
			PaymentMethod:  "card",
			IdempotencyKey: uuid.New(),
		}

		err := repo.Create(ctx, payment)
		if err != nil {
			t.Fatalf("Create: %v", err)
		}

		got, errGet := repo.GetByID(ctx, payment.ID)
		if errGet != nil {
			t.Fatalf("GetByID after Create: %v", errGet)
		}
		if got.Amount != 50000 {
			t.Errorf("amount = %d; want 50000", got.Amount)
		}
		if got.Status != domain.PaymentStatusProcessing {
			t.Errorf("status = %q; want %q", got.Status, domain.PaymentStatusProcessing)
		}
	})

	t.Run("rejects duplicate idempotency key", func(t *testing.T) {
		payment := &domain.Payment{
			ID:             uuid.New(),
			OrderID:        testOrderID1,
			Status:         domain.PaymentStatusProcessing,
			Amount:         100000,
			PaymentMethod:  "card",
			IdempotencyKey: testIdempotency1, // Same as seed data
		}

		err := repo.Create(ctx, payment)
		if err == nil {
			t.Fatal("expected error for duplicate idempotency key, got nil")
		}
		if !errors.Is(err, domain.ErrIdempotencyConflict) {
			t.Errorf("expected ErrIdempotencyConflict in chain, got: %v", err)
		}
	})
}

func TestIntegration_PaymentRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewPaymentRepository(db)
	ctx := context.Background()

	t.Run("returns existing payment", func(t *testing.T) {
		payment, err := repo.GetByID(ctx, testPaymentID1)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if payment.Amount != 100000 {
			t.Errorf("amount = %d; want 100000", payment.Amount)
		}
		if payment.OrderID != testOrderID1 {
			t.Errorf("order_id = %s; want %s", payment.OrderID, testOrderID1)
		}
	})

	t.Run("returns not found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, uuid.New())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}

func TestIntegration_PaymentRepository_GetByPaymentIntentID(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewPaymentRepository(db)
	ctx := context.Background()

	t.Run("finds by payment intent", func(t *testing.T) {
		payment, err := repo.GetByPaymentIntentID(ctx, "pi_existing_intent")
		if err != nil {
			t.Fatalf("GetByPaymentIntentID: %v", err)
		}
		if payment.ID != testPaymentID1 {
			t.Errorf("id = %s; want %s", payment.ID, testPaymentID1)
		}
	})

	t.Run("returns not found for unknown intent", func(t *testing.T) {
		_, err := repo.GetByPaymentIntentID(ctx, "pi_nonexistent")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}

func TestIntegration_PaymentRepository_UpdateStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewPaymentRepository(db)
	ctx := context.Background()

	t.Run("updates to succeeded with outbox", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, testPaymentID1, domain.PaymentStatusSucceeded, nil)
		if err != nil {
			t.Fatalf("UpdateStatus: %v", err)
		}

		payment, errGet := repo.GetByID(ctx, testPaymentID1)
		if errGet != nil {
			t.Fatalf("GetByID: %v", errGet)
		}
		if payment.Status != domain.PaymentStatusSucceeded {
			t.Errorf("status = %q; want %q", payment.Status, domain.PaymentStatusSucceeded)
		}

		// Verify outbox event was created.
		var count int
		errQuery := db.Pool.QueryRow(ctx,
			"SELECT COUNT(*) FROM outbox_events WHERE aggregate_id = $1 AND event_type = $2",
			testPaymentID1, domain.EventPaymentSucceeded,
		).Scan(&count)
		if errQuery != nil {
			t.Fatalf("query outbox: %v", errQuery)
		}
		if count != 1 {
			t.Errorf("outbox event count = %d; want 1", count)
		}
	})

	t.Run("returns not found for non-existent payment", func(t *testing.T) {
		err := repo.UpdateStatus(ctx, uuid.New(), domain.PaymentStatusFailed, nil)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, domain.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}

func TestIntegration_PaymentRepository_UpdatePaymentIntent(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewPaymentRepository(db)
	ctx := context.Background()

	t.Run("updates payment intent", func(t *testing.T) {
		// Create a new payment without intent to test the update.
		payment := &domain.Payment{
			ID:             uuid.New(),
			OrderID:        testOrderID2,
			Status:         domain.PaymentStatusProcessing,
			Amount:         50000,
			PaymentMethod:  "card",
			IdempotencyKey: uuid.New(),
		}
		errCreate := repo.Create(ctx, payment)
		if errCreate != nil {
			t.Fatalf("Create: %v", errCreate)
		}

		err := repo.UpdatePaymentIntent(ctx, payment.ID, "pi_new_intent")
		if err != nil {
			t.Fatalf("UpdatePaymentIntent: %v", err)
		}

		got, errGet := repo.GetByPaymentIntentID(ctx, "pi_new_intent")
		if errGet != nil {
			t.Fatalf("GetByPaymentIntentID: %v", errGet)
		}
		if got.ID != payment.ID {
			t.Errorf("id = %s; want %s", got.ID, payment.ID)
		}
	})
}

func TestIntegration_OrderRepository_GetOrderAmount(t *testing.T) {
	db := setupTestDB(t)
	repo := repository.NewOrderRepository(db)
	ctx := context.Background()

	t.Run("returns order amount", func(t *testing.T) {
		amount, err := repo.GetOrderAmount(ctx, testOrderID1)
		if err != nil {
			t.Fatalf("GetOrderAmount: %v", err)
		}
		if amount != 100000 {
			t.Errorf("amount = %d; want 100000", amount)
		}
	})
}
