package domain

import (
	"time"

	"github.com/google/uuid"
)

type Restaurant struct {
	CreatedAt       time.Time `json:"created_at"`
	Name            string    `json:"name"`
	ImageURL        string    `json:"image_url,omitempty"`
	Cuisine         []string  `json:"cuisine"`
	Address         Address   `json:"address"`
	Rating          float64   `json:"rating"`
	DeliveryFee     int       `json:"delivery_fee"`
	DeliveryTimeMin int       `json:"delivery_time_min"`
	ID              uuid.UUID `json:"id"`
	IsActive        bool      `json:"is_active"`
}

type Address struct {
	AddressText string  `json:"address_text"`
	Lat         float64 `json:"lat"`
	Lon         float64 `json:"lon"`
}

type MenuItem struct {
	Name         string       `json:"name"`
	Description  string       `json:"description"`
	Category     string       `json:"category"`
	ImageURLs    []string     `json:"image_urls"`
	Options      []MenuOption `json:"options,omitempty"`
	Price        int          `json:"price"`
	ID           uuid.UUID    `json:"id"`
	RestaurantID uuid.UUID    `json:"restaurant_id"`
	IsAvailable  bool         `json:"is_available"`
}

type MenuOption struct {
	Name  string `json:"name"`
	Price int    `json:"price"`
}

// SearchParams contains parameters for restaurant search.
type SearchParams struct {
	Query   string
	Cuisine string
	Sort    string
	Limit   int64
	Offset  int64
}

// SearchResult contains search results with total count.
type SearchResult struct {
	Items []Restaurant `json:"items"`
	Total int64        `json:"total"`
}
