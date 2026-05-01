package domain

import (
	"context"

	"github.com/google/uuid"
)

// OrderRepository provides data access for orders.
type OrderRepository interface {
	Create(ctx context.Context, order *Order) error
	GetByID(ctx context.Context, id uuid.UUID) (*Order, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status OrderStatus) error
	GetByIdempotencyKey(ctx context.Context, key uuid.UUID) (*Order, error)
}

// CartRepository provides access to user shopping carts stored in Redis.
type CartRepository interface {
	Get(ctx context.Context, userID uuid.UUID) ([]OrderItem, error)
	Set(ctx context.Context, userID uuid.UUID, items []OrderItem) error
	Delete(ctx context.Context, userID uuid.UUID) error
}

// EventPublisher publishes domain events to a message broker.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}
