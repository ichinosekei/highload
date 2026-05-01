package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ichinosekei/highload/services/payment/internal/domain"
	"github.com/ichinosekei/highload/services/payment/internal/platform"
)

type PaymentRepository struct {
	db *platform.PostgresDB
}

func NewPaymentRepository(db *platform.PostgresDB) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) Create(ctx context.Context, payment *domain.Payment) error {
	tx, errTx := r.db.Pool.Begin(ctx)
	if errTx != nil {
		return fmt.Errorf("begin transaction: %w", errTx)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	insertPayment := `
		INSERT INTO payments (id, order_id, status, amount, payment_method, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at, updated_at
	`
	errInsert := tx.QueryRow(ctx, insertPayment,
		payment.ID, payment.OrderID, payment.Status, payment.Amount,
		payment.PaymentMethod, payment.IdempotencyKey,
	).Scan(&payment.CreatedAt, &payment.UpdatedAt)
	if errInsert != nil {
		if isUniqueViolation(errInsert) {
			return fmt.Errorf("payment idempotency key %s: %w", payment.IdempotencyKey, domain.ErrIdempotencyConflict)
		}
		return fmt.Errorf("insert payment: %w", errInsert)
	}

	if errCommit := tx.Commit(ctx); errCommit != nil {
		return fmt.Errorf("commit transaction: %w", errCommit)
	}

	return nil
}

func (r *PaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	query := `
		SELECT id, order_id, status, amount, payment_method, payment_intent_id,
		       idempotency_key, failure_reason, created_at, updated_at
		FROM payments
		WHERE id = $1
	`
	return r.scanPayment(ctx, query, id)
}

func (r *PaymentRepository) GetByIdempotencyKey(ctx context.Context, key uuid.UUID) (*domain.Payment, error) {
	query := `
		SELECT id, order_id, status, amount, payment_method, payment_intent_id,
		       idempotency_key, failure_reason, created_at, updated_at
		FROM payments
		WHERE idempotency_key = $1
	`
	return r.scanPayment(ctx, query, key)
}

func (r *PaymentRepository) GetByPaymentIntentID(ctx context.Context, intentID string) (*domain.Payment, error) {
	query := `
		SELECT id, order_id, status, amount, payment_method, payment_intent_id,
		       idempotency_key, failure_reason, created_at, updated_at
		FROM payments
		WHERE payment_intent_id = $1
	`
	return r.scanPayment(ctx, query, intentID)
}

func (r *PaymentRepository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status domain.PaymentStatus,
	failureReason *string,
) error {
	tx, errTx := r.db.Pool.Begin(ctx)
	if errTx != nil {
		return fmt.Errorf("begin transaction: %w", errTx)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `UPDATE payments SET status = $1, failure_reason = $2, updated_at = now() WHERE id = $3`
	tag, errExec := tx.Exec(ctx, query, status, failureReason, id)
	if errExec != nil {
		return fmt.Errorf("update payment status: %w", errExec)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("payment %s: %w", id, domain.ErrNotFound)
	}

	// Determine event type and build outbox event.
	eventType := domain.EventPaymentSucceeded
	if status == domain.PaymentStatusFailed {
		eventType = domain.EventPaymentFailed
	}

	// Get order_id for the event payload.
	var orderID uuid.UUID
	errOrder := tx.QueryRow(ctx, "SELECT order_id FROM payments WHERE id = $1", id).Scan(&orderID)
	if errOrder != nil {
		return fmt.Errorf("get order_id for outbox: %w", errOrder)
	}

	payload, errPayload := json.Marshal(map[string]any{
		"payment_id": id,
		"order_id":   orderID,
		"status":     status,
	})
	if errPayload != nil {
		return fmt.Errorf("marshal outbox payload: %w", errPayload)
	}

	insertOutbox := `
		INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
	`
	_, errOutbox := tx.Exec(ctx, insertOutbox, "payment", id, eventType, payload)
	if errOutbox != nil {
		return fmt.Errorf("insert outbox event: %w", errOutbox)
	}

	if errCommit := tx.Commit(ctx); errCommit != nil {
		return fmt.Errorf("commit transaction: %w", errCommit)
	}

	return nil
}

func (r *PaymentRepository) UpdatePaymentIntent(ctx context.Context, id uuid.UUID, intentID string) error {
	query := `UPDATE payments SET payment_intent_id = $1, updated_at = now() WHERE id = $2`
	tag, err := r.db.Pool.Exec(ctx, query, intentID, id)
	if err != nil {
		return fmt.Errorf("update payment intent: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("payment %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r *PaymentRepository) scanPayment(ctx context.Context, query string, arg any) (*domain.Payment, error) {
	var p domain.Payment

	errScan := r.db.Pool.QueryRow(ctx, query, arg).Scan(
		&p.ID, &p.OrderID, &p.Status, &p.Amount, &p.PaymentMethod,
		&p.PaymentIntentID, &p.IdempotencyKey, &p.FailureReason,
		&p.CreatedAt, &p.UpdatedAt,
	)
	if errScan != nil {
		if errors.Is(errScan, pgx.ErrNoRows) {
			return nil, fmt.Errorf("payment lookup: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("scan payment: %w", errScan)
	}

	return &p, nil
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
