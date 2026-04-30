package nats

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ichinosekei/highload/services/notification/internal/domain"
	"github.com/ichinosekei/highload/services/notification/internal/platform"
	"github.com/nats-io/nats.go/jetstream"
)

type Consumer struct {
	client  *platform.NatsClient
	service domain.NotificationService
	logger  *slog.Logger
}

func NewConsumer(client *platform.NatsClient, service domain.NotificationService, logger *slog.Logger) *Consumer {
	return &Consumer{
		client:  client,
		service: service,
		logger:  logger,
	}
}

func (c *Consumer) Start(ctx context.Context) error {
	js := c.client.JS

	// Create or update consumer
	cons, err := js.CreateOrUpdateConsumer(ctx, "NOTIFICATIONS", jetstream.ConsumerConfig{
		Durable:   "notification-service",
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return fmt.Errorf("create nats consumer: %w", err)
	}

	c.logger.InfoContext(ctx, "nats consumer started", "stream", "NOTIFICATIONS", "durable", "notification-service")

	// Consume messages
	_, errConsume := cons.Consume(func(msg jetstream.Msg) {
		subject := msg.Subject()
		data := msg.Data()

		c.logger.DebugContext(ctx, "received nats message", "subject", subject)

		var errProc error
		switch subject {
		case domain.EventOrderCreated:
			var payload domain.OrderCreatedPayload
			if errProc = json.Unmarshal(data, &payload); errProc == nil {
				errProc = c.service.ProcessOrderCreated(ctx, payload)
			}
		case domain.EventPaymentSucceeded:
			var payload domain.PaymentSucceededPayload
			if errProc = json.Unmarshal(data, &payload); errProc == nil {
				errProc = c.service.ProcessPaymentSucceeded(ctx, payload)
			}
		case domain.EventPaymentFailed:
			var payload domain.PaymentFailedPayload
			if errProc = json.Unmarshal(data, &payload); errProc == nil {
				errProc = c.service.ProcessPaymentFailed(ctx, payload)
			}
		case domain.EventOrderStatusUpdated:
			var payload domain.OrderStatusUpdatedPayload
			if errProc = json.Unmarshal(data, &payload); errProc == nil {
				errProc = c.service.ProcessOrderStatusUpdated(ctx, payload)
			}
		default:
			c.logger.WarnContext(ctx, "received message with unknown subject", "subject", subject)
		}

		if errProc != nil {
			c.logger.ErrorContext(ctx, "failed to process message", "subject", subject, "error", errProc)
			// Depending on the error, we might want to Nak or Term
			_ = msg.Nak()
		} else {
			_ = msg.Ack()
		}
	})

	if errConsume != nil {
		return fmt.Errorf("consume nats messages: %w", errConsume)
	}

	return nil
}
