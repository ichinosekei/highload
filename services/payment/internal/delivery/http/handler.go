package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ichinosekei/highload/services/payment/internal/domain"
)

type Handler struct {
	payments  domain.PaymentRepository
	orders    domain.OrderStatusUpdater
	psp       domain.PSPClient
	publisher domain.EventPublisher
	logger    *slog.Logger
}

func NewHandler(
	payments domain.PaymentRepository,
	orders domain.OrderStatusUpdater,
	psp domain.PSPClient,
	publisher domain.EventPublisher,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		payments:  payments,
		orders:    orders,
		psp:       psp,
		publisher: publisher,
		logger:    logger,
	}
}

// RegisterRoutes mounts all payment routes onto the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Post("/payments", h.CreatePayment)
	r.Post("/payments/webhook", h.HandleWebhook)
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

// CreatePayment handles POST /api/v1/payments.
func (h *Handler) CreatePayment(w http.ResponseWriter, r *http.Request) {
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

	var req domain.CreatePaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OrderID == uuid.Nil {
		h.writeError(w, r, http.StatusBadRequest, "order_id is required")
		return
	}

	// Get order amount.
	amount, errAmount := h.orders.GetOrderAmount(r.Context(), req.OrderID)
	if errAmount != nil {
		h.logger.ErrorContext(r.Context(), "get order amount", "error", errAmount)
		h.writeError(w, r, http.StatusBadRequest, "order not found")
		return
	}

	paymentMethod := req.PaymentMethod
	if paymentMethod == "" {
		paymentMethod = "card"
	}

	payment := &domain.Payment{
		ID:             uuid.New(),
		OrderID:        req.OrderID,
		Status:         domain.PaymentStatusProcessing,
		Amount:         amount,
		PaymentMethod:  paymentMethod,
		IdempotencyKey: parsedKey,
	}

	if errCreate := h.payments.Create(r.Context(), payment); errCreate != nil {
		if errors.Is(errCreate, domain.ErrIdempotencyConflict) {
			existing, errGet := h.payments.GetByIdempotencyKey(r.Context(), parsedKey)
			if errGet != nil {
				h.logger.ErrorContext(r.Context(), "get payment by idempotency key", "error", errGet)
				h.writeError(w, r, http.StatusInternalServerError, "internal server error")
				return
			}
			h.writeJSON(w, r, http.StatusConflict, existing)
			return
		}
		h.logger.ErrorContext(r.Context(), "create payment", "error", errCreate)
		h.writeError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	// Initiate payment with PSP.
	pspResp, errPSP := h.psp.InitiatePayment(r.Context(), amount, req.ReturnURL)
	if errPSP != nil {
		h.logger.ErrorContext(r.Context(), "psp initiate payment", "error", errPSP)
		// Payment is created but PSP failed — mark as failed.
		reason := "PSP unavailable"
		_ = h.payments.UpdateStatus(r.Context(), payment.ID, domain.PaymentStatusFailed, &reason)
		h.writeError(w, r, http.StatusServiceUnavailable, "payment provider unavailable")
		return
	}

	// Store payment intent ID.
	if errIntent := h.payments.UpdatePaymentIntent(r.Context(), payment.ID, pspResp.PaymentIntentID); errIntent != nil {
		h.logger.ErrorContext(r.Context(), "update payment intent", "error", errIntent)
	}

	resp := domain.CreatePaymentResponse{
		PaymentID:       payment.ID,
		Status:          payment.Status,
		Amount:          amount,
		PaymentIntentID: pspResp.PaymentIntentID,
		RedirectURL:     pspResp.RedirectURL,
	}

	h.writeJSON(w, r, http.StatusCreated, resp)
}

// HandleWebhook handles POST /api/v1/payments/webhook.
func (h *Handler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	var req domain.WebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.PaymentIntentID == "" {
		h.writeError(w, r, http.StatusBadRequest, "payment_intent_id is required")
		return
	}

	payment, errGet := h.payments.GetByPaymentIntentID(r.Context(), req.PaymentIntentID)
	if errGet != nil {
		if errors.Is(errGet, domain.ErrNotFound) {
			h.writeError(w, r, http.StatusNotFound, "payment not found")
			return
		}
		h.logger.ErrorContext(r.Context(), "get payment by intent", "error", errGet)
		h.writeError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	// Idempotency: if already in terminal state, return OK.
	if payment.Status == domain.PaymentStatusSucceeded || payment.Status == domain.PaymentStatusFailed {
		h.writeJSON(w, r, http.StatusOK, map[string]string{"status": string(payment.Status)})
		return
	}

	var newStatus domain.PaymentStatus
	var failureReason *string
	var orderStatus string

	switch req.Status {
	case "succeeded":
		newStatus = domain.PaymentStatusSucceeded
		orderStatus = "accepted"
	case "failed":
		newStatus = domain.PaymentStatusFailed
		reason := req.FailureReason
		failureReason = &reason
		orderStatus = "payment_failed"
	default:
		h.writeError(w, r, http.StatusBadRequest, fmt.Sprintf("unknown webhook status: %s", req.Status))
		return
	}

	// Update payment status (+ outbox).
	if errStatus := h.payments.UpdateStatus(r.Context(), payment.ID, newStatus, failureReason); errStatus != nil {
		h.logger.ErrorContext(r.Context(), "update payment status", "error", errStatus)
		h.writeError(w, r, http.StatusInternalServerError, "internal server error")
		return
	}

	// Update order status.
	if errOrder := h.orders.UpdateOrderStatus(r.Context(), payment.OrderID, orderStatus); errOrder != nil {
		h.logger.ErrorContext(r.Context(), "update order status from webhook", "error", errOrder)
	}

	// Publish event.
	h.publishPaymentEvent(r, payment, newStatus)

	h.writeJSON(w, r, http.StatusOK, map[string]string{"status": string(newStatus)})
}

func (h *Handler) publishPaymentEvent(r *http.Request, payment *domain.Payment, status domain.PaymentStatus) {
	eventType := domain.EventPaymentSucceeded
	if status == domain.PaymentStatusFailed {
		eventType = domain.EventPaymentFailed
	}

	payload, err := json.Marshal(map[string]any{
		"payment_id": payment.ID,
		"order_id":   payment.OrderID,
		"status":     status,
	})
	if err != nil {
		h.logger.ErrorContext(r.Context(), "marshal payment event", "error", err)
		return
	}

	if errPub := h.publisher.Publish(r.Context(), eventType, payload); errPub != nil {
		h.logger.WarnContext(r.Context(), "publish payment event", "error", errPub)
	}
}
