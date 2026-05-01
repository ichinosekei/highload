package platform

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ichinosekei/highload/services/payment/internal/domain"
)

type MockPSPClient struct{}

func NewMockPSPClient() *MockPSPClient {
	return &MockPSPClient{}
}

func (c *MockPSPClient) InitiatePayment(
	_ context.Context,
	_ int,
	returnURL string,
) (*domain.PSPResponse, error) {
	intentID := fmt.Sprintf("pi_%s", uuid.New().String()[:8])
	return &domain.PSPResponse{
		PaymentIntentID: intentID,
		RedirectURL:     fmt.Sprintf("https://psp.example.com/pay/%s?return=%s", intentID, returnURL),
		Status:          "requires_action",
	}, nil
}
