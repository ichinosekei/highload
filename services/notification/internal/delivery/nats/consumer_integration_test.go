package nats_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	core_logger "github.com/ichinosekei/highload/internal/logger"
	"github.com/ichinosekei/highload/services/notification/internal/delivery/nats"
	"github.com/ichinosekei/highload/services/notification/internal/domain"
	"github.com/ichinosekei/highload/services/notification/internal/platform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/testcontainers/testcontainers-go"
	nats_mod "github.com/testcontainers/testcontainers-go/modules/nats"
)

const testTimeout = 30 * time.Second

func TestConsumer_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
	defer cancel()

	// 1. Start NATS container
	natsContainer, err := nats_mod.Run(ctx, "nats:2.10-alpine", testcontainers.WithWaitStrategy(nil))
	assert.NoError(t, err)
	defer func() {
		_ = natsContainer.Terminate(context.Background())
	}()

	natsURL, err := natsContainer.ConnectionString(ctx)
	assert.NoError(t, err)

	// 2. Initialize NatsClient
	natsClient, err := platform.NewNatsClient(natsURL)
	assert.NoError(t, err)
	defer natsClient.Close()

	// Ensure stream
	err = natsClient.EnsureStream(ctx, "NOTIFICATIONS", []string{"order.*", "payment.*"})
	assert.NoError(t, err)

	// 3. Initialize Service and Consumer
	mockService := new(domain.MockNotificationService)
	logger := core_logger.NewLogger("local", "test")
	consumer := nats.NewConsumer(natsClient, mockService, logger)

	err = consumer.Start(ctx)
	assert.NoError(t, err)

	// 4. Test order.created event
	orderID := uuid.New()
	userID := uuid.New()
	payload := domain.OrderCreatedPayload{
		OrderID: orderID,
		UserID:  userID,
		Total:   1000,
	}
	payloadBytes, _ := json.Marshal(payload)

	// Expect service call
	done := make(chan bool, 1)
	mockService.On("ProcessOrderCreated", mock.Anything, payload).Return(nil).Run(func(args mock.Arguments) {
		done <- true
	})

	// Publish message
	_, err = natsClient.JS.Publish(ctx, domain.EventOrderCreated, payloadBytes)
	assert.NoError(t, err)

	// Wait for processing
	select {
	case <-done:
		// Success
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for message processing")
	}

	mockService.AssertExpectations(t)
}
