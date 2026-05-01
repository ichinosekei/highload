package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ichinosekei/highload/services/catalog/internal/domain"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type Handler struct {
	menuRes domain.MenuRestaurantReader
	search  domain.RestaurantSearcher
	logger  *slog.Logger
}

func NewHandler(
	menuRes domain.MenuRestaurantReader,
	search domain.RestaurantSearcher,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		menuRes: menuRes,
		search:  search,
		logger:  logger,
	}
}

// RegisterRoutes mounts all catalog routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/catalog/restaurants", h.ListRestaurants)
	r.Get("/catalog/restaurants/{restaurantID}/menu", h.GetRestaurantMenu)
	r.Get("/search", h.Search)
}

// writeJSON writes a JSON response.
func (h *Handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.ErrorContext(r.Context(), "encode json response", "error", err)
	}
}

// writeError writes a JSON error response.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.writeJSON(w, r, status, map[string]string{"error": message})
}

// ListRestaurants handles GET /api/v1/catalog/restaurants.
func (h *Handler) ListRestaurants(w http.ResponseWriter, r *http.Request) {
	limit, offset, errParse := parsePagination(r)
	if errParse != nil {
		h.logger.DebugContext(r.Context(), "parse pagination", "error", errParse)
		h.writeError(w, r, http.StatusBadRequest, errParse.Error())
		return
	}

	restaurants, errList := h.menuRes.List(r.Context(), limit, offset)
	if errList != nil {
		h.logger.ErrorContext(r.Context(), "list restaurants", "error", errList)
		h.writeError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	if restaurants == nil {
		restaurants = []domain.Restaurant{}
	}

	h.writeJSON(w, r, http.StatusOK, restaurants)
}

// GetRestaurantMenu handles GET /api/v1/catalog/restaurants/{restaurantID}/menu.
func (h *Handler) GetRestaurantMenu(w http.ResponseWriter, r *http.Request) {
	restaurantIDStr := chi.URLParam(r, "restaurantID")
	restaurantID, errParse := uuid.Parse(restaurantIDStr)
	if errParse != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid restaurant_id format")
		return
	}

	// Verify restaurant exists.
	restaurant, errGet := h.menuRes.GetByID(r.Context(), restaurantID)
	if errGet != nil {
		if errors.Is(errGet, domain.ErrNotFound) {
			h.writeError(w, r, http.StatusNotFound, "restaurant not found")
			return
		}
		h.logger.ErrorContext(r.Context(), "get restaurant by id", "error", errGet, "restaurant_id", restaurantID)
		h.writeError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	items, errList := h.menuRes.ListByRestaurant(r.Context(), restaurantID)
	if errList != nil {
		h.logger.ErrorContext(r.Context(), "list menu items", "error", errList, "restaurant_id", restaurantID)
		h.writeError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	if items == nil {
		items = []domain.MenuItem{}
	}

	h.writeJSON(w, r, http.StatusOK, map[string]any{
		"restaurant": restaurant,
		"items":      items,
	})
}

// Search handles GET /api/v1/search.
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	limit, offset, errPagination := parsePagination(r)
	if errPagination != nil {
		h.writeError(w, r, http.StatusBadRequest, errPagination.Error())
		return
	}

	sort := r.URL.Query().Get("sort")
	if sort != "" && !isValidSort(sort) {
		h.writeError(w, r, http.StatusBadRequest, "invalid sort parameter; allowed: rating, delivery_time, price")
		return
	}

	params := domain.SearchParams{
		Query:   r.URL.Query().Get("q"),
		Cuisine: r.URL.Query().Get("cuisine"),
		Sort:    sort,
		Limit:   limit,
		Offset:  offset,
	}

	result, errSearch := h.search.Search(r.Context(), params)
	if errSearch != nil {
		h.logger.ErrorContext(r.Context(), "search restaurants", "error", errSearch)
		h.writeError(w, r, http.StatusInternalServerError, "search service unavailable")
		return
	}

	h.writeJSON(w, r, http.StatusOK, result)
}

func parsePagination(r *http.Request) (int64, int64, error) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := int64(defaultLimit)
	var err error
	if limitStr != "" {
		limit, err = strconv.ParseInt(limitStr, 10, 64)
		if err != nil || limit <= 0 {
			return 0, 0, errors.New("invalid limit: must be a positive integer")
		}
	}

	if limit > int64(maxLimit) {
		return 0, 0, fmt.Errorf("limit exceeds maximum allowed value of %d", maxLimit)
	}

	var offset int64
	if offsetStr != "" {
		offset, err = strconv.ParseInt(offsetStr, 10, 64)
		if err != nil || offset < 0 {
			return 0, 0, errors.New("invalid offset: must be a non-negative integer")
		}
	}

	return limit, offset, nil
}

func isValidSort(sort string) bool {
	switch sort {
	case "rating", "delivery_time", "price":
		return true
	default:
		return false
	}
}
