package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ichinosekei/highload/services/payment/internal/platform"
)

// OrderRepository provides cross-entity access to orders within the same DB.
type OrderRepository struct {
	db *platform.PostgresDB
}

func NewOrderRepository(db *platform.PostgresDB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status string) error {
	query := `UPDATE orders SET status = $1, updated_at = now() WHERE id = $2`
	tag, err := r.db.Pool.Exec(ctx, query, status, orderID)
	if err != nil {
		return fmt.Errorf("update order status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("order %s not found for status update", orderID)
	}
	return nil
}

func (r *OrderRepository) GetOrderAmount(ctx context.Context, orderID uuid.UUID) (int, error) {
	var amount int
	err := r.db.Pool.QueryRow(ctx, "SELECT total_amount FROM orders WHERE id = $1", orderID).Scan(&amount)
	if err != nil {
		return 0, fmt.Errorf("get order amount: %w", err)
	}
	return amount, nil
}
