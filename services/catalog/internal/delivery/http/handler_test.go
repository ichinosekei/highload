package http_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"

	catalog_http "github.com/ichinosekei/highload/services/catalog/internal/delivery/http"
)

// --- Mock implementations ---

type mockRestaurantReader struct {
	listFn    func(ctx context.Context, limit, offset int64) ([]domain.Restaurant, error)
	getByIDFn func(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error)
}

func (m *mockRestaurantReader) List(ctx context.Context, limit, offset int64) ([]domain.Restaurant, error) {
	return m.listFn(ctx, limit, offset)
}

func (m *mockRestaurantReader) GetByID(ctx context.Context, id uuid.UUID) (*domain.Restaurant, error) {
	return m.getByIDFn(ctx, id)
}

type mockMenuItemReader struct {
	listByRestaurantFn func(ctx context.Context, restaurantID uuid.UUID) ([]domain.MenuItem, error)
}

func (m *mockMenuItemReader) ListByRestaurant(
	ctx context.Context,
	restaurantID uuid.UUID,
) ([]domain.MenuItem, error) {
	return m.listByRestaurantFn(ctx, restaurantID)
}

type mockSearcher struct {
	searchFn func(ctx context.Context, params domain.SearchParams) (*domain.SearchResult, error)
}

func (m *mockSearcher) Search(ctx context.Context, params domain.SearchParams) (*domain.SearchResult, error) {
	return m.searchFn(ctx, params)
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// setupRouter creates a chi router with the handler mounted at /api/v1.
func setupRouter(h *catalog_http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", h.RegisterRoutes)
	return r
}

// --- Test data ---

//nolint:gochecknoglobals // test data identifiers
var (
	testRestaurantID = uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901")
	testRestaurant   = domain.Restaurant{
		ID:              testRestaurantID,
		Name:            "Pizza House",
		Cuisine:         []string{"italian", "fast_food"},
		Rating:          4.7,
		DeliveryTimeMin: 30,
		DeliveryFee:     149,
		IsActive:        true,
		Address: domain.Address{
			AddressText: "ул. Пушкина, д. 1",
			Lat:         55.751,
			Lon:         37.618,
		},
		ImageURL:  "https://cdn.example.com/restaurants/pizza-house/cover.jpg",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	testMenuItems = []domain.MenuItem{
		{
			ID:           uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001"),
			RestaurantID: testRestaurantID,
			Name:         "Маргарита",
			Description:  "Классическая пицца",
			Price:        49900,
			Category:     "pizza",
			IsAvailable:  true,
			ImageURLs:    []string{"https://cdn.example.com/items/margherita.jpg"},
			Options:      []domain.MenuOption{{Name: "extra_cheese", Price: 5000}},
		},
		{
			ID:           uuid.MustParse("0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d002"),
			RestaurantID: testRestaurantID,
			Name:         "Пепперони",
			Description:  "Пицца с острой пепперони",
			Price:        59900,
			Category:     "pizza",
			IsAvailable:  true,
			ImageURLs:    []string{"https://cdn.example.com/items/pepperoni.jpg"},
		},
	}
)

// --- ListRestaurants Tests ---

func TestListRestaurants_OK(t *testing.T) {
	t.Parallel()

	restaurants := []domain.Restaurant{testRestaurant}
	h := catalog_http.NewHandler(
		&mockRestaurantReader{
			listFn: func(_ context.Context, limit, offset int64) ([]domain.Restaurant, error) {
				if limit != 20 || offset != 0 {
					t.Errorf("unexpected pagination: limit=%d, offset=%d", limit, offset)
				}
				return restaurants, nil
			},
		},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusOK)
	}

	var got []domain.Restaurant
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d restaurants; want 1", len(got))
	}
	if got[0].Name != "Pizza House" {
		t.Errorf("got name %q; want %q", got[0].Name, "Pizza House")
	}
}

func TestListRestaurants_EmptyResult(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{
			listFn: func(_ context.Context, _, _ int64) ([]domain.Restaurant, error) {
				return nil, nil
			},
		},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if body != "[]\n" {
		t.Errorf("got body %q; want empty JSON array", body)
	}
}

func TestListRestaurants_CustomPagination(t *testing.T) {
	t.Parallel()

	var capturedLimit, capturedOffset int64
	h := catalog_http.NewHandler(
		&mockRestaurantReader{
			listFn: func(_ context.Context, limit, offset int64) ([]domain.Restaurant, error) {
				capturedLimit = limit
				capturedOffset = offset
				return []domain.Restaurant{}, nil
			},
		},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants?limit=50&offset=10", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusOK)
	}
	if capturedLimit != 50 {
		t.Errorf("got limit %d; want 50", capturedLimit)
	}
	if capturedOffset != 10 {
		t.Errorf("got offset %d; want 10", capturedOffset)
	}
}

func TestListRestaurants_LimitExceeded(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants?limit=200", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusBadRequest)
	}
}

func TestListRestaurants_InvalidLimit(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	tests := []struct {
		name  string
		query string
	}{
		{"negative limit", "?limit=-1"},
		{"zero limit", "?limit=0"},
		{"non-numeric limit", "?limit=abc"},
		{"negative offset", "?offset=-5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants"+tt.query, nil)
			w := httptest.NewRecorder()

			setupRouter(h).ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("got status %d; want %d for query %s", w.Code, http.StatusBadRequest, tt.query)
			}
		})
	}
}

func TestListRestaurants_InternalError(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{
			listFn: func(_ context.Context, _, _ int64) ([]domain.Restaurant, error) {
				return nil, errTest
			},
		},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusInternalServerError)
	}
}

// --- GetRestaurantMenu Tests ---

func TestGetRestaurantMenu_OK(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{
			getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Restaurant, error) {
				if id != testRestaurantID {
					t.Errorf("got restaurant id %s; want %s", id, testRestaurantID)
				}
				return &testRestaurant, nil
			},
		},
		&mockMenuItemReader{
			listByRestaurantFn: func(_ context.Context, _ uuid.UUID) ([]domain.MenuItem, error) {
				return testMenuItems, nil
			},
		},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/catalog/restaurants/"+testRestaurantID.String()+"/menu", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp struct {
		Items      []domain.MenuItem `json:"items"`
		Restaurant domain.Restaurant `json:"restaurant"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Restaurant.Name != "Pizza House" {
		t.Errorf("got restaurant name %q; want %q", resp.Restaurant.Name, "Pizza House")
	}
	if len(resp.Items) != 2 {
		t.Errorf("got %d menu items; want 2", len(resp.Items))
	}
}

func TestGetRestaurantMenu_InvalidID(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants/not-a-uuid/menu", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetRestaurantMenu_NotFound(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Restaurant, error) {
				return nil, domain.ErrNotFound
			},
		},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	fakeID := uuid.New()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/catalog/restaurants/"+fakeID.String()+"/menu", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusNotFound)
	}
}

func TestGetRestaurantMenu_EmptyMenu(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Restaurant, error) {
				return &testRestaurant, nil
			},
		},
		&mockMenuItemReader{
			listByRestaurantFn: func(_ context.Context, _ uuid.UUID) ([]domain.MenuItem, error) {
				return nil, nil
			},
		},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/catalog/restaurants/"+testRestaurantID.String()+"/menu", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		Items []domain.MenuItem `json:"items"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Items) != 0 {
		t.Errorf("got %d items; want 0", len(resp.Items))
	}
}

// --- Search Tests ---

func TestSearch_OK(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{},
		&mockMenuItemReader{},
		&mockSearcher{
			searchFn: func(_ context.Context, params domain.SearchParams) (*domain.SearchResult, error) {
				if params.Query != "пицца" {
					t.Errorf("got query %q; want %q", params.Query, "пицца")
				}
				if params.Limit != 20 {
					t.Errorf("got limit %d; want 20", params.Limit)
				}
				return &domain.SearchResult{
					Items: []domain.Restaurant{testRestaurant},
					Total: 1,
				}, nil
			},
		},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=пицца", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusOK)
	}

	var resp domain.SearchResult
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Total != 1 {
		t.Errorf("got total %d; want 1", resp.Total)
	}
	if len(resp.Items) != 1 {
		t.Errorf("got %d items; want 1", len(resp.Items))
	}
}

func TestSearch_WithCuisineAndSort(t *testing.T) {
	t.Parallel()

	var captured domain.SearchParams
	h := catalog_http.NewHandler(
		&mockRestaurantReader{},
		&mockMenuItemReader{},
		&mockSearcher{
			searchFn: func(_ context.Context, params domain.SearchParams) (*domain.SearchResult, error) {
				captured = params
				return &domain.SearchResult{
					Items: []domain.Restaurant{},
					Total: 0,
				}, nil
			},
		},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/search?q=pizza&cuisine=italian&sort=rating&limit=10&offset=5", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusOK)
	}

	if captured.Cuisine != "italian" {
		t.Errorf("got cuisine %q; want %q", captured.Cuisine, "italian")
	}
	if captured.Sort != "rating" {
		t.Errorf("got sort %q; want %q", captured.Sort, "rating")
	}
	if captured.Limit != 10 {
		t.Errorf("got limit %d; want 10", captured.Limit)
	}
	if captured.Offset != 5 {
		t.Errorf("got offset %d; want 5", captured.Offset)
	}
}

func TestSearch_InvalidSort(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&sort=invalid", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSearch_LimitExceeded(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&limit=101", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusBadRequest)
	}
}

func TestSearch_InternalError(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{},
		&mockMenuItemReader{},
		&mockSearcher{
			searchFn: func(_ context.Context, _ domain.SearchParams) (*domain.SearchResult, error) {
				return nil, errTest
			},
		},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusInternalServerError)
	}
}

// --- Health Check ---

func TestHealthCheck(t *testing.T) {
	t.Parallel()

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "OK" {
		t.Errorf("got body %q; want %q", w.Body.String(), "OK")
	}
}

// --- JSON response format ---

func TestWriteJSON_ContentType(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&mockRestaurantReader{
			listFn: func(_ context.Context, _, _ int64) ([]domain.Restaurant, error) {
				return []domain.Restaurant{}, nil
			},
		},
		&mockMenuItemReader{},
		&mockSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("got Content-Type %q; want %q", ct, "application/json")
	}
}
