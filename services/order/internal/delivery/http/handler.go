package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ichinosekei/highload/internal/resilience"
	"github.com/ichinosekei/highload/services/order/internal/domain"
)

const (
	defaultDeliveryFee       = 149
	estimatedDeliveryMinutes = 45
)

type Handler struct {
	orders    domain.OrderRepository
	cart      domain.CartRepository
	publisher domain.EventPublisher
	logger    *slog.Logger
}

func NewHandler(
	orders domain.OrderRepository,
	cart domain.CartRepository,
	publisher domain.EventPublisher,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		orders:    orders,
		cart:      cart,
		publisher: publisher,
		logger:    logger,
	}
}

// RegisterRoutes mounts all order routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/orders", h.CreateOrder)
	r.Post("/orders/{orderID}/status", h.UpdateOrderStatus)
	r.Get("/orders/{orderID}/track", h.TrackOrder)
}

func (h *Handler) writeJSON(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.ErrorContext(r.Context(), "encode json response", "error", err)
	}
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, status int, message string) {
	h.writeJSON(w, r, status, map[string]string{"error": message})
}

// CreateOrder handles POST /api/v1/orders.
func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		h.writeError(w, r, http.StatusBadRequest, "Idempotency-Key header is required")
		return
	}

	parsedKey, errKey := uuid.Parse(idempotencyKey)
	if errKey != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid Idempotency-Key format")
		return
	}

	var req domain.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateCreateOrder(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// Calculate total.
	totalAmount := calculateTotal(req.Items)

	order := &domain.Order{
		ID:              uuid.New(),
		UserID:          uuid.New(), // In a real system this would come from auth.
		RestaurantID:    req.RestaurantID,
		Status:          domain.StatusCreated,
		Items:           req.Items,
		TotalAmount:     totalAmount,
		DeliveryFee:     defaultDeliveryFee,
		DeliveryAddress: req.DeliveryAddress,
		Comment:         req.Comment,
		IdempotencyKey:  parsedKey,
	}

	if err := h.orders.Create(r.Context(), order); err != nil {
		if errors.Is(err, domain.ErrIdempotencyConflict) {
			existing, errGet := h.orders.GetByIdempotencyKey(r.Context(), parsedKey)
			if errGet != nil {
				h.logger.ErrorContext(r.Context(), "get order by idempotency key", "error", errGet)
				h.writeError(w, r, http.StatusInternalServerError, "internal server error")
				return
			}
			h.writeJSON(w, r, http.StatusConflict, existing)
			return
		}
		h.logger.ErrorContext(r.Context(), "create order", "error", err)
		h.writeError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	// Publish event via NATS (best-effort; outbox is the source of truth).
	h.publishOrderCreated(r, order)

	h.writeJSON(w, r, http.StatusCreated, order)
}

// UpdateOrderStatus handles POST /api/v1/orders/{orderID}/status.
func (h *Handler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	orderIDStr := chi.URLParam(r, "orderID")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid order_id format")
		return
	}

	var req domain.UpdateStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	newStatus := domain.OrderStatus(req.Status)
	if !isValidStatus(newStatus) {
		h.writeError(w, r, http.StatusBadRequest, "invalid status value")
		return
	}

	if err := h.orders.UpdateStatus(r.Context(), orderID, newStatus); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.writeError(w, r, http.StatusNotFound, "order not found")
			return
		}
		h.logger.ErrorContext(r.Context(), "update order status", "error", err)
		h.writeError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	h.writeJSON(w, r, http.StatusOK, map[string]any{
		"order_id":   orderID,
		"status":     newStatus,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	})
}

// TrackOrder handles GET /api/v1/orders/{orderID}/track.
func (h *Handler) TrackOrder(w http.ResponseWriter, r *http.Request) {
	orderIDStr := chi.URLParam(r, "orderID")
	orderID, err := uuid.Parse(orderIDStr)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid order_id format")
		return
	}

	order, err := h.orders.GetByID(r.Context(), orderID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			h.writeError(w, r, http.StatusNotFound, "order not found")
			return
		}
		h.logger.ErrorContext(r.Context(), "get order for tracking", "error", err)
		h.writeError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	resp := domain.TrackingResponse{
		OrderID: order.ID,
		Status:  order.Status,
		StatusHistory: []domain.StatusHistory{
			{Status: domain.StatusCreated, At: order.CreatedAt},
		},
		EstimatedDelivery: order.CreatedAt.Add(estimatedDeliveryMinutes * time.Minute).UTC().Format(time.RFC3339),
	}

	h.writeJSON(w, r, http.StatusOK, resp)
}

func (h *Handler) publishOrderCreated(r *http.Request, order *domain.Order) {
	payload, err := json.Marshal(map[string]any{
		"order_id": order.ID,
		"user_id":  order.UserID,
		"total":    order.TotalAmount,
	})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "marshal order event", "error", err)
		return
	}

	if err := resilience.Retry(r.Context(), func() error {
		return h.publisher.Publish(r.Context(), domain.EventOrderCreated, payload)
	}); err != nil {
		h.logger.WarnContext(r.Context(), "publish order.created event", "error", err)
	}
}

func validateCreateOrder(req *domain.CreateOrderRequest) error {
	if req.RestaurantID == uuid.Nil {
		return fmt.Errorf("restaurant_id is required: %w", domain.ErrInvalidInput)
	}
	if len(req.Items) == 0 {
		return fmt.Errorf("items cannot be empty: %w", domain.ErrInvalidInput)
	}
	for i, item := range req.Items {
		if item.MenuItemID == uuid.Nil {
			return fmt.Errorf("items[%d].menu_item_id is required: %w", i, domain.ErrInvalidInput)
		}
		if item.Quantity <= 0 {
			return fmt.Errorf("items[%d].quantity must be positive: %w", i, domain.ErrInvalidInput)
		}
	}
	return nil
}

func calculateTotal(items []domain.OrderItem) int {
	const defaultPrice = 50000 // PoC: fixed price per item.
	total := 0
	for _, item := range items {
		total += defaultPrice * item.Quantity
	}
	return total
}

func isValidStatus(status domain.OrderStatus) bool {
	switch status {
	case domain.StatusCreated, domain.StatusAccepted, domain.StatusCooking,
		domain.StatusReady, domain.StatusCourierAssigned, domain.StatusOnTheWay,
		domain.StatusDelivered, domain.StatusCancelled, domain.StatusPaymentFailed:
		return true
	default:
		return false
	}
}
