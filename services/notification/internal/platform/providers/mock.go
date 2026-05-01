package providers

import (
	"context"
	"log/slog"

	"github.com/ichinosekei/highload/services/notification/internal/domain"
)

type MockSender struct {
	logger *slog.Logger
}

func NewMockSender(logger *slog.Logger) *MockSender {
	return &MockSender{
		logger: logger,
	}
}

func (s *MockSender) Send(ctx context.Context, notification *domain.Notification) error {
	s.logger.InfoContext(ctx, "sending notification (MOCK)",
		"type", notification.Type,
		"user_id", notification.UserID,
		"target", notification.Target,
		"title", notification.Title,
		"message", notification.Message,
	)
	return nil
}
