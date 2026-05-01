package domain

import (
	"time"

	"github.com/google/uuid"
)

type NotificationType string

const (
	TypePush NotificationType = "push"
	TypeSMS  NotificationType = "sms"
)

type NotificationStatus string

const (
	StatusPending NotificationStatus = "pending"
	StatusSent    NotificationStatus = "sent"
	StatusFailed  NotificationStatus = "failed"
)

type Notification struct {
	CreatedAt time.Time          `json:"created_at"`
	SentAt    *time.Time         `json:"sent_at,omitempty"`
	Type      NotificationType   `json:"type"`
	Target    string             `json:"target"`
	Title     string             `json:"title"`
	Message   string             `json:"message"`
	Status    NotificationStatus `json:"status"`
	ID        uuid.UUID          `json:"id"`
	UserID    uuid.UUID          `json:"user_id"`
}

// Event types.
const (
	EventOrderCreated       = "order.created"
	EventPaymentSucceeded   = "payment.succeeded"
	EventPaymentFailed      = "payment.failed"
	EventOrderStatusUpdated = "order.status_updated"
)

// EventPayloads (simplified for PoC)

type OrderCreatedPayload struct {
	OrderID uuid.UUID `json:"order_id"`
	UserID  uuid.UUID `json:"user_id"`
	Total   int       `json:"total"`
}

type PaymentSucceededPayload struct {
	OrderID   uuid.UUID `json:"order_id"`
	UserID    uuid.UUID `json:"user_id"`
	PaymentID uuid.UUID `json:"payment_id"`
}

type PaymentFailedPayload struct {
	Reason  string    `json:"reason"`
	OrderID uuid.UUID `json:"order_id"`
	UserID  uuid.UUID `json:"user_id"`
}

type OrderStatusUpdatedPayload struct {
	Status  string    `json:"status"`
	OrderID uuid.UUID `json:"order_id"`
	UserID  uuid.UUID `json:"user_id"`
}
