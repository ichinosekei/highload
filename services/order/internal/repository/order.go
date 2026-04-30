package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ichinosekei/highload/services/order/internal/domain"
	"github.com/ichinosekei/highload/services/order/internal/platform"
)

type OrderRepository struct {
	db *platform.PostgresDB
}

func NewOrderRepository(db *platform.PostgresDB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Create(ctx context.Context, order *domain.Order) error {
	tx, errTx := r.db.Pool.Begin(ctx)
	if errTx != nil {
		return fmt.Errorf("begin transaction: %w", errTx)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	itemsJSON, errMarshal := json.Marshal(order.Items)
	if errMarshal != nil {
		return fmt.Errorf("marshal items: %w", errMarshal)
	}

	addressJSON, errAddr := json.Marshal(order.DeliveryAddress)
	if errAddr != nil {
		return fmt.Errorf("marshal delivery address: %w", errAddr)
	}

	insertOrder := `
		INSERT INTO orders (id, user_id, restaurant_id, status, items_json, total_amount,
		                    delivery_fee, delivery_address, comment, idempotency_key)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`
	errInsert := tx.QueryRow(ctx, insertOrder,
		order.ID, order.UserID, order.RestaurantID, order.Status,
		itemsJSON, order.TotalAmount, order.DeliveryFee,
		addressJSON, order.Comment, order.IdempotencyKey,
	).Scan(&order.CreatedAt, &order.UpdatedAt)
	if errInsert != nil {
		if isUniqueViolation(errInsert) {
			return fmt.Errorf("order idempotency key %s: %w", order.IdempotencyKey, domain.ErrIdempotencyConflict)
		}
		return fmt.Errorf("insert order: %w", errInsert)
	}

	// Insert outbox event in the same transaction.
	payload, errPayload := json.Marshal(map[string]any{
		"order_id": order.ID,
		"user_id":  order.UserID,
		"total":    order.TotalAmount,
	})
	if errPayload != nil {
		return fmt.Errorf("marshal outbox payload: %w", errPayload)
	}

	insertOutbox := `
		INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		VALUES ($1, $2, $3, $4)
	`
	_, errOutbox := tx.Exec(ctx, insertOutbox, "order", order.ID, domain.EventOrderCreated, payload)
	if errOutbox != nil {
		return fmt.Errorf("insert outbox event: %w", errOutbox)
	}

	if errCommit := tx.Commit(ctx); errCommit != nil {
		return fmt.Errorf("commit transaction: %w", errCommit)
	}

	return nil
}

func (r *OrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	query := `
		SELECT id, user_id, restaurant_id, status, items_json, total_amount,
		       delivery_fee, delivery_address, comment, idempotency_key, created_at, updated_at
		FROM orders
		WHERE id = $1
	`
	return r.scanOrder(ctx, query, id)
}

func (r *OrderRepository) GetByIdempotencyKey(ctx context.Context, key uuid.UUID) (*domain.Order, error) {
	query := `
		SELECT id, user_id, restaurant_id, status, items_json, total_amount,
		       delivery_fee, delivery_address, comment, idempotency_key, created_at, updated_at
		FROM orders
		WHERE idempotency_key = $1
	`
	return r.scanOrder(ctx, query, key)
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error {
	query := `
		UPDATE orders SET status = $1, updated_at = now()
		WHERE id = $2
	`
	tag, err := r.db.Pool.Exec(ctx, query, status, id)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("order %s: %w", id, domain.ErrNotFound)
	}
	return nil
}

func (r *OrderRepository) scanOrder(ctx context.Context, query string, arg any) (*domain.Order, error) {
	var order domain.Order
	var itemsJSON, addressJSON []byte

	errScan := r.db.Pool.QueryRow(ctx, query, arg).Scan(
		&order.ID, &order.UserID, &order.RestaurantID, &order.Status,
		&itemsJSON, &order.TotalAmount, &order.DeliveryFee,
		&addressJSON, &order.Comment, &order.IdempotencyKey,
		&order.CreatedAt, &order.UpdatedAt,
	)
	if errScan != nil {
		if errors.Is(errScan, pgx.ErrNoRows) {
			return nil, fmt.Errorf("order lookup: %w", domain.ErrNotFound)
		}
		return nil, fmt.Errorf("scan order: %w", errScan)
	}

	if errItems := json.Unmarshal(itemsJSON, &order.Items); errItems != nil {
		return nil, fmt.Errorf("unmarshal order items: %w", errItems)
	}
	if errAddr := json.Unmarshal(addressJSON, &order.DeliveryAddress); errAddr != nil {
		return nil, fmt.Errorf("unmarshal delivery address: %w", errAddr)
	}

	return &order, nil
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "23505")
}
