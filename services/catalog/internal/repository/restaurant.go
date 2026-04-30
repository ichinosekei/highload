package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
	"github.com/ichinosekei/highload/services/catalog/internal/platform"
)

type RestaurantRepository struct {
	db *platform.PostgresDB
}

func NewRestaurantRepository(db *platform.PostgresDB) *RestaurantRepository {
	return &RestaurantRepository{db: db}
}

func (r *RestaurantRepository) List(ctx context.Context, limit, offset int64) ([]domain.Restaurant, error) {
	query := `
		SELECT id, name, cuisine, rating, delivery_time_min, delivery_fee,
		       is_active, address, image_url, created_at
		FROM restaurants
		WHERE is_active = true
		ORDER BY rating DESC
		LIMIT $1 OFFSET $2
	`
	rows, errQuery := r.db.Pool.Query(ctx, query, limit, offset)
	if errQuery != nil {
		return nil, fmt.Errorf("list restaurants: %w", errQuery)
	}
	defer rows.Close()

	var restaurants []domain.Restaurant
	for rows.Next() {
		var res domain.Restaurant
		errScan := rows.Scan(
			&res.ID,
			&res.Name,
			&res.Cuisine,
			&res.Rating,
			&res.DeliveryTimeMin,
			&res.DeliveryFee,
			&res.IsActive,
			&res.Address,
			&res.ImageURL,
			&res.CreatedAt,
		)
		if errScan != nil {
			return nil, fmt.Errorf("scan restaurant row: %w", errScan)
		}
		restaurants = append(restaurants, res)
	}

	if errRows := rows.Err(); errRows != nil {
		return nil, fmt.Errorf("iterate restaurant rows: %w", errRows)
	}

	return restaurants, nil
}

func (r *RestaurantRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	query := `
		SELECT id, name, cuisine, rating, delivery_time_min, delivery_fee,
		       is_active, address, image_url, created_at
		FROM restaurants
		WHERE id = $1
	`
	var res domain.Restaurant
	errScan := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&res.ID,
		&res.Name,
		&res.Cuisine,
		&res.Rating,
		&res.DeliveryTimeMin,
		&res.DeliveryFee,
		&res.IsActive,
		&res.Address,
		&res.ImageURL,
		&res.CreatedAt,
	)
	if errScan != nil {
		if errors.Is(errScan, pgx.ErrNoRows) {
			return nil, fmt.Errorf("restaurant %s: %w", id, domain.ErrNotFound)
		}
		return nil, fmt.Errorf("find restaurant by ID: %w", errScan)
	}

	return &res, nil
}
