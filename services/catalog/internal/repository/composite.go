package repository

import "github.com/ichinosekei/highload/services/catalog/internal/domain"

type MenuRestaurantComposite struct {
	domain.MenuReader
	domain.RestaurantReader
}
