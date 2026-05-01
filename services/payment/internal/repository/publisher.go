package repository

import (
	"context"
	"fmt"

	"github.com/ichinosekei/highload/services/payment/internal/platform"
)

type NatsPublisher struct {
	client *platform.NatsClient
}

func NewNatsPublisher(client *platform.NatsClient) *NatsPublisher {
	return &NatsPublisher{client: client}
}

func (p *NatsPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	_, err := p.client.JS.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("publish nats event %s: %w", subject, err)
	}
	return nil
}
