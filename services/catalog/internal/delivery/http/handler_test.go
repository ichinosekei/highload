package http_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	shared_logger "github.com/ichinosekei/highload/internal/logger"
	catalog_http "github.com/ichinosekei/highload/services/catalog/internal/delivery/http"
	"github.com/ichinosekei/highload/services/catalog/internal/domain"
)

func newTestLogger() *slog.Logger {
	return shared_logger.NewLogger("test", "test")
}

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
	mockReader := new(domain.MockRestaurantReader)
	mockSearcher := new(domain.MockRestaurantSearcher)

	mockReader.
		On("List", mock.Anything, int64(20), int64(0)).
		Return(restaurants, nil)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{
			MockRestaurantReader: mockReader,
		},
		mockSearcher,
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var got []domain.Restaurant
	err := json.NewDecoder(w.Body).Decode(&got)
	require.NoError(t, err)

	assert.Len(t, got, 1)
	assert.Equal(t, "Pizza House", got[0].Name)

	mockReader.AssertExpectations(t)
}

func TestListRestaurants_EmptyResult(t *testing.T) {
	t.Parallel()

	mockReader := new(domain.MockRestaurantReader)
	mockSearcher := new(domain.MockRestaurantSearcher)

	mockReader.
		On("List", mock.Anything, mock.Anything, mock.Anything).
		Return([]domain.Restaurant{}, nil)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{
			MockRestaurantReader: mockReader,
		},
		mockSearcher,
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, "[]\n", w.Body.String())
}

func TestListRestaurants_CustomPagination(t *testing.T) {
	t.Parallel()

	mockReader := new(domain.MockRestaurantReader)
	mockSearcher := new(domain.MockRestaurantSearcher)

	mockReader.
		On("List", mock.Anything, int64(50), int64(10)).
		Return([]domain.Restaurant{}, nil)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{
			MockRestaurantReader: mockReader,
		},
		mockSearcher,
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants?limit=50&offset=10", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockReader.AssertExpectations(t)
}

func TestListRestaurants_LimitExceeded(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{},
		&domain.MockRestaurantSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants?limit=200", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestListRestaurants_InvalidLimit(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{},
		&domain.MockRestaurantSearcher{},
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

			assert.Equal(t, http.StatusBadRequest, w.Code, "query: %s", tt.query)
		})
	}
}

func TestListRestaurants_InternalError(t *testing.T) {
	t.Parallel()

	mockReader := new(domain.MockRestaurantReader)
	mockSearcher := new(domain.MockRestaurantSearcher)

	mockReader.
		On("List", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errTest)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{
			MockRestaurantReader: mockReader,
		},
		mockSearcher,
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// --- GetRestaurantMenu Tests ---

func TestGetRestaurantMenu_OK(t *testing.T) {
	t.Parallel()

	mockReader := new(domain.MockRestaurantReader)
	mockMenuReader := new(domain.MockMenuReader)

	mockReader.
		On("GetByID", mock.Anything, testRestaurantID).
		Return(&testRestaurant, nil)
	mockMenuReader.
		On("ListByRestaurant", mock.Anything, testRestaurantID).
		Return(testMenuItems, nil)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{
			MockRestaurantReader: mockReader,
			MockMenuReader:       mockMenuReader,
		},
		new(domain.MockRestaurantSearcher),
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/catalog/restaurants/"+testRestaurantID.String()+"/menu", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Items      []domain.MenuItem `json:"items"`
		Restaurant domain.Restaurant `json:"restaurant"`
	}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, "Pizza House", resp.Restaurant.Name)
	assert.Len(t, resp.Items, 2)
}

func TestGetRestaurantMenu_InvalidID(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{},
		&domain.MockRestaurantSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants/not-a-uuid/menu", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetRestaurantMenu_NotFound(t *testing.T) {
	t.Parallel()

	mockReader := new(domain.MockRestaurantReader)
	mockReader.
		On("GetByID", mock.Anything, mock.Anything).
		Return(nil, domain.ErrNotFound)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{
			MockRestaurantReader: mockReader,
		},
		&domain.MockRestaurantSearcher{},
		newTestLogger(),
	)

	fakeID := uuid.New()
	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/catalog/restaurants/"+fakeID.String()+"/menu", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestGetRestaurantMenu_EmptyMenu(t *testing.T) {
	t.Parallel()

	mockReader := new(domain.MockRestaurantReader)
	mockMenuReader := new(domain.MockMenuReader)

	mockReader.
		On("GetByID", mock.Anything, mock.Anything).
		Return(&testRestaurant, nil)
	mockMenuReader.
		On("ListByRestaurant", mock.Anything, mock.Anything).
		Return([]domain.MenuItem{}, nil)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{
			MockRestaurantReader: mockReader,
			MockMenuReader:       mockMenuReader,
		},
		&domain.MockRestaurantSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/catalog/restaurants/"+testRestaurantID.String()+"/menu", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Items []domain.MenuItem `json:"items"`
	}
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
}

// --- Search Tests ---

func TestSearch_OK(t *testing.T) {
	t.Parallel()

	mockSearcher := new(domain.MockRestaurantSearcher)
	mockSearcher.
		On("Search", mock.Anything, domain.SearchParams{
			Query: "пицца",
			Limit: 20,
		}).
		Return(&domain.SearchResult{
			Items: []domain.Restaurant{testRestaurant},
			Total: 1,
		}, nil)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{},
		mockSearcher,
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=пицца", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp domain.SearchResult
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)

	assert.Equal(t, int64(1), resp.Total)
	assert.Len(t, resp.Items, 1)
	mockSearcher.AssertExpectations(t)
}

func TestSearch_WithCuisineAndSort(t *testing.T) {
	t.Parallel()

	mockSearcher := new(domain.MockRestaurantSearcher)
	expectedParams := domain.SearchParams{
		Query:   "pizza",
		Cuisine: "italian",
		Sort:    "rating",
		Limit:   10,
		Offset:  5,
	}
	mockSearcher.
		On("Search", mock.Anything, expectedParams).
		Return(&domain.SearchResult{
			Items: []domain.Restaurant{},
			Total: 0,
		}, nil)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{},
		mockSearcher,
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet,
		"/api/v1/search?q=pizza&cuisine=italian&sort=rating&limit=10&offset=5", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockSearcher.AssertExpectations(t)
}

func TestSearch_InvalidSort(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{},
		&domain.MockRestaurantSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&sort=invalid", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearch_LimitExceeded(t *testing.T) {
	t.Parallel()

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{},
		&domain.MockRestaurantSearcher{},
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test&limit=101", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestSearch_InternalError(t *testing.T) {
	t.Parallel()

	mockSearcher := new(domain.MockRestaurantSearcher)
	mockSearcher.
		On("Search", mock.Anything, mock.Anything).
		Return(nil, assert.AnError)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{},
		mockSearcher,
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=test", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp domain.SearchResult
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Empty(t, resp.Items)
	assert.Equal(t, int64(0), resp.Total)
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

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "OK", w.Body.String())
}

// --- JSON response format ---

func TestWriteJSON_ContentType(t *testing.T) {
	t.Parallel()

	mockReader := new(domain.MockRestaurantReader)
	mockReader.
		On("List", mock.Anything, mock.Anything, mock.Anything).
		Return([]domain.Restaurant{}, nil)

	h := catalog_http.NewHandler(
		&domain.MockMenuRestaurantReader{
			MockRestaurantReader: mockReader,
		},
		new(domain.MockRestaurantSearcher),
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog/restaurants", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}
