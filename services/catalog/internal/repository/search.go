package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	meili_go "github.com/meilisearch/meilisearch-go"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
)

const searchTimeout = 5 * time.Second

type SearchRepository struct {
	client *platform.MeiliClient
}

func NewSearchRepository(client *platform.MeiliClient) *SearchRepository {
	return &SearchRepository{client: client}
}

func (r *SearchRepository) Sync(ctx context.Context, restaurants []domain.Restaurant) error {
	if len(restaurants) == 0 {
		return nil
	}

	task, err := r.client.Index("restaurants").AddDocumentsWithContext(ctx, restaurants, nil)
	if err != nil {
		return fmt.Errorf("meilisearch add documents: %w", err)
	}

	if _, err := r.client.WaitForTaskWithContext(ctx, task.TaskUID, searchTimeout); err != nil {
		return fmt.Errorf("wait for meilisearch sync task: %w", err)
	}

	return nil
}

func (r *SearchRepository) Search(
	ctx context.Context,
	params domain.SearchParams,
) (*domain.SearchResult, error) {
	req := &meili_go.SearchRequest{
		Limit:  params.Limit,
		Offset: params.Offset,
		Filter: "is_active = true",
	}

	if params.Cuisine != "" {
		req.Filter = fmt.Sprintf("is_active = true AND cuisine = %q", params.Cuisine)
	}

	if params.Sort != "" {
		req.Sort = buildSort(params.Sort)
	}

	searchCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	raw, err := r.client.Index("restaurants").SearchRawWithContext(searchCtx, params.Query, req)
	if err != nil {
		return nil, fmt.Errorf("meilisearch search: %w", err)
	}

	var resp struct {
		Hits               []domain.Restaurant `json:"hits"`
		TotalHits          int64               `json:"totalHits"`
		EstimatedTotalHits int64               `json:"estimatedTotalHits"`
	}
	if err := json.Unmarshal(*raw, &resp); err != nil {
		return nil, fmt.Errorf("decode search response: %w", err)
	}

	total := resp.TotalHits
	if total == 0 {
		total = resp.EstimatedTotalHits
	}

	return &domain.SearchResult{
		Items: resp.Hits,
		Total: total,
	}, nil
}

func buildSort(sort string) []string {
	switch sort {
	case "rating":
		return []string{"rating:desc"}
	case "delivery_time":
		return []string{"delivery_time_min:asc"}
	case "price":
		return []string{"delivery_fee:asc"}
	default:
		return nil
	}
}
