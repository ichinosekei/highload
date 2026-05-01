package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/ichinosekei/highload/services/notification/internal/domain"
)

type Service struct {
	sender domain.NotificationSender
	logger *slog.Logger
}

func NewService(sender domain.NotificationSender, logger *slog.Logger) *Service {
	return &Service{
		sender: sender,
		logger: logger,
	}
}

func (s *Service) ProcessOrderCreated(ctx context.Context, payload domain.OrderCreatedPayload) error {
	notif := &domain.Notification{
		ID:        uuid.New(),
		UserID:    payload.UserID,
		Type:      domain.TypePush,
		Target:    "dummy-push-token",
		Title:     "Order Created",
		Message:   fmt.Sprintf("Your order #%s for %d is being processed!", payload.OrderID, payload.Total),
		Status:    domain.StatusPending,
		CreatedAt: time.Now(),
	}

	return s.sender.Send(ctx, notif)
}

func (s *Service) ProcessPaymentSucceeded(ctx context.Context, payload domain.PaymentSucceededPayload) error {
	notif := &domain.Notification{
		ID:        uuid.New(),
		UserID:    payload.UserID,
		Type:      domain.TypePush,
		Target:    "dummy-push-token",
		Title:     "Payment Succeeded",
		Message:   fmt.Sprintf("Payment for order #%s was successful!", payload.OrderID),
		Status:    domain.StatusPending,
		CreatedAt: time.Now(),
	}

	return s.sender.Send(ctx, notif)
}

func (s *Service) ProcessPaymentFailed(ctx context.Context, payload domain.PaymentFailedPayload) error {
	notif := &domain.Notification{
		ID:        uuid.New(),
		UserID:    payload.UserID,
		Type:      domain.TypePush,
		Target:    "dummy-push-token",
		Title:     "Payment Failed",
		Message:   fmt.Sprintf("Payment for order #%s failed: %s", payload.OrderID, payload.Reason),
		Status:    domain.StatusPending,
		CreatedAt: time.Now(),
	}

	return s.sender.Send(ctx, notif)
}

func (s *Service) ProcessOrderStatusUpdated(ctx context.Context, payload domain.OrderStatusUpdatedPayload) error {
	var message string
	switch payload.Status {
	case "cooking":
		message = "Your order is being cooked! 🍳"
	case "ready":
		message = "Your order is ready and waiting for a courier!"
	case "on_the_way":
		message = "Courier is on the way! 🚴"
	case "delivered":
		message = "Order delivered! Enjoy your meal! 🎉"
	default:
		message = fmt.Sprintf("Order #%s status updated to: %s", payload.OrderID, payload.Status)
	}

	notif := &domain.Notification{
		ID:        uuid.New(),
		UserID:    payload.UserID,
		Type:      domain.TypePush,
		Target:    "dummy-push-token",
		Title:     "Order Update",
		Message:   message,
		Status:    domain.StatusPending,
		CreatedAt: time.Now(),
	}

	return s.sender.Send(ctx, notif)
}
