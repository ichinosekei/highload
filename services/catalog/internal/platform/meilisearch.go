package platform

import (
	"fmt"
	"time"

	meili "github.com/meilisearch/meilisearch-go"
)

const defaultMeiliTimeout = 5 * time.Second

type MeiliClient struct {
	meili.ServiceManager
}

func NewMeiliClient(host, apiKey string) (*MeiliClient, error) {
	client := meili.New(host, meili.WithAPIKey(apiKey))

	if _, err := client.Health(); err != nil {
		return nil, fmt.Errorf("meili connection: %w", err)
	}

	return &MeiliClient{client}, nil
}

func (c *MeiliClient) InitIndices() error {
	index := c.Index("restaurants")

	filterable := []any{"cuisine", "rating", "is_active"}
	taskFilterable, errFilterable := index.UpdateFilterableAttributes(&filterable)
	if errFilterable != nil {
		return fmt.Errorf("update filterable attributes: %w", errFilterable)
	}
	if _, err := c.WaitForTask(taskFilterable.TaskUID, defaultMeiliTimeout); err != nil {
		return fmt.Errorf("wait for filterable attributes: %w", err)
	}

	sortable := []string{"rating", "delivery_time_min", "delivery_fee"}
	taskSortable, errSortable := index.UpdateSortableAttributes(&sortable)
	if errSortable != nil {
		return fmt.Errorf("update sortable attributes: %w", errSortable)
	}
	if _, err := c.WaitForTask(taskSortable.TaskUID, defaultMeiliTimeout); err != nil {
		return fmt.Errorf("wait for sortable attributes: %w", err)
	}

	return nil
}
