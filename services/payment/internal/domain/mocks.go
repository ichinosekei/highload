package domain

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type MockPaymentRepository struct {
	mock.Mock
}

func (m *MockPaymentRepository) Create(ctx context.Context, payment *Payment) error {
	args := m.Called(ctx, payment)
	return args.Error(0)
}

func (m *MockPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*Payment, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Payment), args.Error(1) //nolint:errcheck // mock type assertion
}

func (m *MockPaymentRepository) GetByIdempotencyKey(ctx context.Context, key uuid.UUID) (*Payment, error) {
	args := m.Called(ctx, key)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Payment), args.Error(1) //nolint:errcheck // mock type assertion
}

func (m *MockPaymentRepository) GetByPaymentIntentID(ctx context.Context, intentID string) (*Payment, error) {
	args := m.Called(ctx, intentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Payment), args.Error(1) //nolint:errcheck // mock type assertion
}

func (m *MockPaymentRepository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status PaymentStatus,
	failureReason *string,
) error {
	args := m.Called(ctx, id, status, failureReason)
	return args.Error(0)
}

func (m *MockPaymentRepository) UpdatePaymentIntent(ctx context.Context, id uuid.UUID, intentID string) error {
	args := m.Called(ctx, id, intentID)
	return args.Error(0)
}

type MockOrderStatusUpdater struct {
	mock.Mock
}

func (m *MockOrderStatusUpdater) UpdateOrderStatus(ctx context.Context, orderID uuid.UUID, status string) error {
	args := m.Called(ctx, orderID, status)
	return args.Error(0)
}

func (m *MockOrderStatusUpdater) GetOrderAmount(ctx context.Context, orderID uuid.UUID) (int, error) {
	args := m.Called(ctx, orderID)
	return args.Int(0), args.Error(1)
}

type MockPSPClient struct {
	mock.Mock
}

func (m *MockPSPClient) InitiatePayment(ctx context.Context, amount int, returnURL string) (*PSPResponse, error) {
	args := m.Called(ctx, amount, returnURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PSPResponse), args.Error(1) //nolint:errcheck // mock type assertion
}

type MockEventPublisher struct {
	mock.Mock
}

func (m *MockEventPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	args := m.Called(ctx, subject, data)
	return args.Error(0)
}
