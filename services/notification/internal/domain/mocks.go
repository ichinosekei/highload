package domain

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type MockNotificationSender struct {
	mock.Mock
}

func (m *MockNotificationSender) Send(ctx context.Context, notification *Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

type MockNotificationService struct {
	mock.Mock
}

func (m *MockNotificationService) ProcessOrderCreated(ctx context.Context, payload OrderCreatedPayload) error {
	args := m.Called(ctx, payload)
	return args.Error(0)
}

func (m *MockNotificationService) ProcessPaymentSucceeded(ctx context.Context, payload PaymentSucceededPayload) error {
	args := m.Called(ctx, payload)
	return args.Error(0)
}

func (m *MockNotificationService) ProcessPaymentFailed(ctx context.Context, payload PaymentFailedPayload) error {
	args := m.Called(ctx, payload)
	return args.Error(0)
}

func (m *MockNotificationService) ProcessOrderStatusUpdated(
	ctx context.Context,
	payload OrderStatusUpdatedPayload,
) error {
	args := m.Called(ctx, payload)
	return args.Error(0)
}
