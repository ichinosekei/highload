package app_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	core_logger "github.com/ichinosekei/highload/internal/logger"
	"github.com/ichinosekei/highload/services/notification/internal/app"
	"github.com/ichinosekei/highload/services/notification/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestService_ProcessOrderCreated(t *testing.T) {
	mockSender := new(domain.MockNotificationSender)
	logger := core_logger.NewLogger("local", "test")
	svc := app.NewService(mockSender, logger)

	userID := uuid.New()
	orderID := uuid.New()
	payload := domain.OrderCreatedPayload{
		OrderID: orderID,
		UserID:  userID,
		Total:   1500,
	}

	mockSender.On("Send", mock.Anything, mock.MatchedBy(func(n *domain.Notification) bool {
		return n.UserID == userID && n.Title == "Order Created"
	})).Return(nil)

	err := svc.ProcessOrderCreated(context.Background(), payload)
	assert.NoError(t, err)
	mockSender.AssertExpectations(t)
}
