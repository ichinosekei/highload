package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meilisearch/meilisearch-go"
)

type RestaurantDoc struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	Cuisine         []string `json:"cuisine"`
	Rating          float64  `json:"rating"`
	DeliveryTimeMin int      `json:"delivery_time_min"`
	DeliveryFee     int      `json:"delivery_fee"`
	IsActive        bool     `json:"is_active"`
}

func main() {
	ctx := context.Background()

	// Connect to Catalog DB
	catalogConnStr := os.Getenv("CATALOG_DB_URL")
	if catalogConnStr == "" {
		catalogConnStr = "postgres://postgres_user:postgres_password@127.0.0.1:5432/catalog_db?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, catalogConnStr)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	// Connect to Meilisearch
	meiliHost := os.Getenv("MEILI_HOST")
	if meiliHost == "" {
		meiliHost = "http://127.0.0.1:7700"
	}
	meiliKey := os.Getenv("MEILI_MASTER_KEY")
	if meiliKey == "" {
		meiliKey = "masterKeyRequired16Chars"
	}
	meiliClient := meilisearch.New(meiliHost, meilisearch.WithAPIKey(meiliKey))

	fmt.Println("Seeding database with realistic data matching schema...")

	// 1. Create Restaurants
	restaurantsCount := 1000
	fmt.Printf("Creating %d restaurants...\n", restaurantsCount)

	cuisines := []string{"italian", "japanese", "fast_food", "american", "chinese", "asian", "mexican", "french", "bakery", "vegan", "healthy"}
	restaurantIDs := make([]uuid.UUID, restaurantsCount)
	meiliDocs := make([]RestaurantDoc, 0, restaurantsCount)

	for i := 0; i < restaurantsCount; i++ {
		id := uuid.New()
		restaurantIDs[i] = id
		name := fmt.Sprintf("Restaurant %d", i)

		// Random cuisines (1-3)
		randCuisines := []string{}
		for j := 0; j < 1+rand.Intn(3); j++ {
			randCuisines = append(randCuisines, cuisines[rand.Intn(len(cuisines))])
		}

		addressJSON := fmt.Sprintf(`{"address_text": "ул. Тестовая, д. %d", "lat": 55.75, "lon": 37.62}`, i)
		rating := 3.0 + rand.Float64()*2.0
		deliveryTime := 15 + rand.Intn(45)
		deliveryFee := rand.Intn(500)

		_, err := pool.Exec(ctx, `
			INSERT INTO restaurants (id, name, cuisine, rating, delivery_time_min, delivery_fee, is_active, address, image_url)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT DO NOTHING`,
			id, name, randCuisines, rating, deliveryTime, deliveryFee, true, addressJSON, "")
		if err != nil {
			log.Printf("Error inserting restaurant: %v", err)
		}

		meiliDocs = append(meiliDocs, RestaurantDoc{
			ID:              id.String(),
			Name:            name,
			Cuisine:         randCuisines,
			Rating:          rating,
			DeliveryTimeMin: deliveryTime,
			DeliveryFee:     deliveryFee,
			IsActive:        true,
		})
	}

	fmt.Println("Syncing restaurants to Meilisearch...")
	task, err := meiliClient.Index("restaurants").AddDocuments(meiliDocs, nil)
	if err != nil {
		log.Printf("Meilisearch sync error: %v", err)
	} else {
		fmt.Printf("Meilisearch sync task started: %d\n", task.TaskUID)
	}

	// 2. Create Menu Items
	itemsCount := 100000 // 100k items
	fmt.Printf("Creating %d menu items...\n", itemsCount)

	categories := []string{"pizza", "rolls", "burgers", "noodles", "tacos", "bakery", "drinks", "desserts"}

	for i := 0; i < itemsCount; i++ {
		id := uuid.New()
		restID := restaurantIDs[rand.Intn(len(restaurantIDs))]
		category := categories[rand.Intn(len(categories))]
		name := fmt.Sprintf("Dish %d", i)
		price := (rand.Intn(100) + 10) * 100 // 10.00 - 110.00 in minor units

		_, err := pool.Exec(ctx, `
			INSERT INTO menu_items (id, restaurant_id, name, description, price, category, is_available, image_urls, options)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT DO NOTHING`,
			id, restID, name, "Delicious food description", price, category, true, []string{}, "[]")

		if i%10000 == 0 && i > 0 {
			fmt.Printf("Inserted %d items...\n", i)
		}

		if err != nil {
			log.Printf("Error inserting item: %v", err)
		}
	}

	fmt.Println("Seeding completed successfully!")
}
