package domain

import (
	"context"

	"github.com/google/uuid"
)

// PaymentRepository provides data access for payments.
type PaymentRepository interface {
	Create(ctx context.Context, payment *Payment) error
	GetByID(ctx context.Context, id uuid.UUID) (*Payment, error)
	GetByIdempotencyKey(ctx context.Context, key uuid.UUID) (*Payment, error)
	GetByPaymentIntentID(ctx context.Context, intentID string) (*Payment, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status PaymentStatus, failureReason *string) error
	UpdatePaymentIntent(ctx context.Context, id uuid.UUID, intentID string) error
}

// OrderStatusUpdater updates order status (cross-entity in same DB).
type OrderStatusUpdater interface {
	UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status string) error
	GetOrderAmount(ctx context.Context, orderID uuid.UUID) (int, error)
}

// PSPClient communicates with the external payment service provider.
type PSPClient interface {
	InitiatePayment(ctx context.Context, amount int, returnURL string) (*PSPResponse, error)
}

// EventPublisher publishes domain events to a message broker.
type EventPublisher interface {
	Publish(ctx context.Context, subject string, data []byte) error
}
