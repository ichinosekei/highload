package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
)

type MenuItemRepository struct {
	db *platform.PostgresDB
}

func NewMenuItemRepository(db *platform.PostgresDB) *MenuItemRepository {
	return &MenuItemRepository{db: db}
}

func (r *MenuItemRepository) ListByRestaurant(
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
	rows, errQuery := r.db.Pool.Query(ctx, query, restaurantID)
	if errQuery != nil {
		return nil, fmt.Errorf("list menu items for restaurant %s: %w", restaurantID, errQuery)
	}
	defer rows.Close()

	var items []domain.MenuItem
	for rows.Next() {
		var item domain.MenuItem
		errScan := rows.Scan(
			&item.ID,
			&item.RestaurantID,
			&item.Name,
			&item.Description,
			&item.Price,
			&item.Category,
			&item.IsAvailable,
			&item.ImageURLs,
			&item.Options,
		)
		if errScan != nil {
			return nil, fmt.Errorf("scan menu item row: %w", errScan)
		}
		items = append(items, item)
	}

	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("iterate menu item rows: %w", errRows)
	}

	return items, nil
}
