package domain

import (
	"context"

	"github.com/google/uuid"
)

// RestaurantReader provides read access to restaurant data.
type RestaurantReader interface {
	List(ctx context.Context, limit, offset int64) ([]Restaurant, error)
	GetByID(ctx context.Context, id uuid.UUID) (*Restaurant, error)
}

// MenuItemReader provides read access to menu items.
type MenuItemReader interface {
	ListByRestaurant(ctx context.Context, restaurantID uuid.UUID) ([]MenuItem, error)
}

// RestaurantSearcher provides search capabilities for restaurants.
type RestaurantSearcher interface {
	Search(ctx context.Context, params SearchParams) (*SearchResult, error)
}
