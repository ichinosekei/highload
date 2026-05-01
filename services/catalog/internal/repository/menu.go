package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
)

type MenuRepository struct {
	db *platform.PostgresDB
}

func NewMenuRepository(db *platform.PostgresDB) *MenuRepository {
	return &MenuRepository{db: db}
}

func (r *MenuRepository) ListByRestaurant(
	ctx context.Context,
	restaurantID uuid.UUID,
) ([]domain.MenuItem, error) {
	query := `
		SELECT id, restaurant_id, name, description, price,
		       category, is_available, image_urls, options
		FROM menu_items
		WHERE restaurant_id = $1 AND is_available = true
		ORDER BY category, name
	`
	rows, err := r.db.Pool.Query(ctx, query, restaurantID)
	if err != nil {
		return nil, fmt.Errorf("list menu items for restaurant %s: %w", restaurantID, err)
	}
	defer rows.Close()

	var items []domain.MenuItem
	for rows.Next() {
		var item domain.MenuItem
		if err := rows.Scan(
			&item.ID,
			&item.RestaurantID,
			&item.Name,
			&item.Description,
			&item.Price,
			&item.Category,
			&item.IsAvailable,
			&item.ImageURLs,
			&item.Options,
		); err != nil {
			return nil, fmt.Errorf("scan menu item row: %w", err)
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate menu item rows: %w", err)
	}

	return items, nil
}
