package platform

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

const defaultMeiliTimeout = 5 * time.Second

type MeiliClient struct {
	meilisearch.ServiceManager
}

func NewMeiliClient(host, apiKey string) (*MeiliClient, error) {
	client := meilisearch.New(host, meilisearch.WithAPIKey(apiKey))

	if _, err := client.Health(); err != nil {
		return nil, fmt.Errorf("meili connection: %w", err)
	}

	return &MeiliClient{client}, nil
}

func (c *MeiliClient) InitIndices(ctx context.Context) error {
	index := c.Index("restaurants")

	filterable := []any{"cuisine", "rating", "is_active"}
	taskFilterable, err := index.UpdateFilterableAttributesWithContext(ctx, &filterable)
	if err != nil {
		return fmt.Errorf("update filterable attributes: %w", err)
	}
	if _, err = c.WaitForTaskWithContext(ctx, taskFilterable.TaskUID, defaultMeiliTimeout); err != nil {
		return fmt.Errorf("wait for filterable attributes: %w", err)
	}

	sortable := []string{"rating", "delivery_time_min", "delivery_fee"}
	taskSortable, err := index.UpdateSortableAttributesWithContext(ctx, &sortable)
	if err != nil {
		return fmt.Errorf("update sortable attributes: %w", err)
	}
	if _, err = c.WaitForTaskWithContext(ctx, taskSortable.TaskUID, defaultMeiliTimeout); err != nil {
		return fmt.Errorf("wait for sortable attributes: %w", err)
	}

	return nil
}
