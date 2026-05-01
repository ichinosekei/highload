package domain

import (
	"time"

	"github.com/google/uuid"
)

// PaymentStatus represents the lifecycle state of a payment.
type PaymentStatus string

const (
	PaymentStatusProcessing PaymentStatus = "processing"
	PaymentStatusSucceeded  PaymentStatus = "succeeded"
	PaymentStatusFailed     PaymentStatus = "failed"
	PaymentStatusRefunding  PaymentStatus = "refunding"
	PaymentStatusRefunded   PaymentStatus = "refunded"
)

type Payment struct {
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	FailureReason   *string       `json:"failure_reason,omitempty"`
	PaymentIntentID *string       `json:"payment_intent_id,omitempty"`
	PaymentMethod   string        `json:"payment_method"`
	Status          PaymentStatus `json:"status"`
	Amount          int           `json:"amount"`
	ID              uuid.UUID     `json:"id"`
	OrderID         uuid.UUID     `json:"order_id"`
	IdempotencyKey  uuid.UUID     `json:"idempotency_key"`
}

// PSPResponse represents a response from the payment service provider (mocked).
type PSPResponse struct {
	PaymentIntentID string `json:"payment_intent_id"`
	RedirectURL     string `json:"redirect_url"`
	Status          string `json:"status"`
}

// Event type constants for NATS subjects.
const (
	EventPaymentSucceeded = "payment.succeeded"
	EventPaymentFailed    = "payment.failed"
)

// CreatePaymentRequest is the incoming API request.
type CreatePaymentRequest struct {
	PaymentMethod string    `json:"payment_method"`
	ReturnURL     string    `json:"return_url"`
	OrderID       uuid.UUID `json:"order_id"`
}

// CreatePaymentResponse is the API response for payment initiation.
type CreatePaymentResponse struct {
	PaymentIntentID string        `json:"payment_intent_id,omitempty"`
	RedirectURL     string        `json:"redirect_url,omitempty"`
	Status          PaymentStatus `json:"status"`
	Amount          int           `json:"amount"`
	PaymentID       uuid.UUID     `json:"payment_id"`
}

// WebhookRequest represents an incoming PSP webhook callback.
type WebhookRequest struct {
	PaymentIntentID string `json:"payment_intent_id"`
	Status          string `json:"status"` // "succeeded" or "failed"
	FailureReason   string `json:"failure_reason,omitempty"`
}
