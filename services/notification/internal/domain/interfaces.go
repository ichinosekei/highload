package domain

import (
	"context"
)

// NotificationSender defines the interface for sending notifications.
type NotificationSender interface {
	Send(ctx context.Context, notification *Notification) error
}

// NotificationService defines the application logic for notifications.
type NotificationService interface {
	ProcessOrderCreated(ctx context.Context, payload OrderCreatedPayload) error
	ProcessPaymentSucceeded(ctx context.Context, payload PaymentSucceededPayload) error
	ProcessPaymentFailed(ctx context.Context, payload PaymentFailedPayload) error
	ProcessOrderStatusUpdated(ctx context.Context, payload OrderStatusUpdatedPayload) error
}
