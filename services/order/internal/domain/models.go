package domain

import (
	"time"

	"github.com/google/uuid"
)

// OrderStatus represents the lifecycle state of an order.
type OrderStatus string

const (
	StatusCreated         OrderStatus = "created"
	StatusAccepted        OrderStatus = "accepted"
	StatusCooking         OrderStatus = "cooking"
	StatusReady           OrderStatus = "ready"
	StatusCourierAssigned OrderStatus = "courier_assigned"
	StatusOnTheWay        OrderStatus = "on_the_way"
	StatusDelivered       OrderStatus = "delivered"
	StatusCancelled       OrderStatus = "cancelled"
	StatusPaymentFailed   OrderStatus = "payment_failed"
)

type Order struct {
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	Comment         string          `json:"comment"`
	Status          OrderStatus     `json:"status"`
	Items           []OrderItem     `json:"items"`
	DeliveryAddress DeliveryAddress `json:"delivery_address"`
	TotalAmount     int             `json:"total_amount"`
	DeliveryFee     int             `json:"delivery_fee"`
	ID              uuid.UUID       `json:"id"`
	UserID          uuid.UUID       `json:"user_id"`
	RestaurantID    uuid.UUID       `json:"restaurant_id"`
	IdempotencyKey  uuid.UUID       `json:"idempotency_key"`
}

type OrderItem struct {
	Options    []string  `json:"options,omitempty"`
	MenuItemID uuid.UUID `json:"menu_item_id"`
	Quantity   int       `json:"quantity"`
}

type DeliveryAddress struct {
	AddressText string  `json:"address_text"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

// StatusHistory represents a single status change entry for order tracking.
type StatusHistory struct {
	At     time.Time   `json:"at"`
	Status OrderStatus `json:"status"`
}

// OutboxEvent represents an event stored in the transactional outbox.
type OutboxEvent struct {
	CreatedAt     time.Time `json:"created_at"`
	Payload       any       `json:"payload"`
	AggregateType string    `json:"aggregate_type"`
	EventType     string    `json:"event_type"`
	ID            int64     `json:"id"`
	AggregateID   uuid.UUID `json:"aggregate_id"`
	Published     bool      `json:"published"`
}

// Event type constants for NATS subjects.
const (
	EventOrderCreated       = "order.created"
	EventOrderStatusUpdated = "order.status_updated"
)

// CreateOrderRequest represents the incoming request to create an order.
type CreateOrderRequest struct {
	Comment         string          `json:"comment,omitempty"`
	Items           []OrderItem     `json:"items"`
	DeliveryAddress DeliveryAddress `json:"delivery_address"`
	RestaurantID    uuid.UUID       `json:"restaurant_id"`
}

// UpdateStatusRequest represents the incoming request to update order status.
type UpdateStatusRequest struct {
	Status string `json:"status"`
}

// TrackingResponse is the response for the tracking endpoint.
type TrackingResponse struct {
	Status            OrderStatus     `json:"status"`
	EstimatedDelivery string          `json:"estimated_delivery"`
	StatusHistory     []StatusHistory `json:"status_history"`
	OrderID           uuid.UUID       `json:"order_id"`
}
