package app

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/ichinosekei/highload/services/order/internal/domain"
	"github.com/ichinosekei/highload/services/order/internal/platform"
)

type OutboxRelay struct {
	db        *platform.PostgresDB
	publisher domain.EventPublisher
	logger    *slog.Logger
	interval  time.Duration
}

const (
	defaultRelayInterval = 5 * time.Second
	defaultRelayLimit    = 100
)

func NewOutboxRelay(db *platform.PostgresDB, publisher domain.EventPublisher, logger *slog.Logger) *OutboxRelay {
	return &OutboxRelay{
		db:        db,
		publisher: publisher,
		logger:    logger,
		interval:  defaultRelayInterval,
	}
}

func (r *OutboxRelay) Start(ctx context.Context) {
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	r.logger.InfoContext(ctx, "outbox relay started")

	for {
		select {
		case <-ctx.Done():
			r.logger.InfoContext(ctx, "outbox relay stopping")
			return
		case <-ticker.C:
			if err := r.processEvents(ctx); err != nil {
				r.logger.ErrorContext(ctx, "process outbox events", "error", err)
			}
		}
	}
}

func (r *OutboxRelay) processEvents(ctx context.Context) error {
	rows, err := r.db.Pool.Query(ctx, fmt.Sprintf(`
		SELECT id, event_type, payload
		FROM outbox_events
		WHERE published = false
		ORDER BY created_at ASC
		LIMIT %d
	`, defaultRelayLimit))
	if err != nil {
		return fmt.Errorf("query unpublished events: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id int64
		var eventType string
		var payload []byte

		if err := rows.Scan(&id, &eventType, &payload); err != nil {
			return fmt.Errorf("scan outbox event: %w", err)
		}

		if err := r.publisher.Publish(ctx, eventType, payload); err != nil {
			r.logger.WarnContext(ctx, "relay publish event", "event_id", id, "error", err)
			continue
		}

		if _, err := r.db.Pool.Exec(ctx, "UPDATE outbox_events SET published = true WHERE id = $1", id); err != nil {
			r.logger.ErrorContext(ctx, "mark event as published", "event_id", id, "error", err)
		}
	}

	return nil
}
