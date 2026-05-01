package http_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	shared_logger "github.com/ichinosekei/highload/internal/logger"
	payment_http "github.com/ichinosekei/highload/services/payment/internal/delivery/http"
	"github.com/ichinosekei/highload/services/payment/internal/domain"
)

func newTestLogger() *slog.Logger {
	return shared_logger.NewLogger("test", "test")
}

func setupRouter(h *payment_http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", h.RegisterRoutes)
	return r
}

func noopPublisher() *domain.MockEventPublisher {
	m := new(domain.MockEventPublisher)
	m.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return m
}

// --- CreatePayment Tests ---

func TestCreatePayment_OK(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()
	mockRepo := new(domain.MockPaymentRepository)
	mockStatusUpdater := new(domain.MockOrderStatusUpdater)
	mockPSP := new(domain.MockPSPClient)

	mockRepo.
		On("Create", mock.Anything, mock.Anything).
		Return(nil)
	mockRepo.
		On("UpdatePaymentIntent", mock.Anything, mock.Anything, "pi_test123").
		Return(nil)

	mockStatusUpdater.
		On("GetOrderAmount", mock.Anything, orderID).
		Return(100000, nil)

	mockPSP.
		On("InitiatePayment", mock.Anything, 100000, "https://app.example.com/callback").
		Return(&domain.PSPResponse{
			PaymentIntentID: "pi_test123",
			RedirectURL:     "https://psp.example.com/pay/pi_test123",
			Status:          "requires_action",
		}, nil)

	h := payment_http.NewHandler(
		mockRepo,
		mockStatusUpdater,
		mockPSP,
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"order_id":"` + orderID.String() + `","payment_method":"card","return_url":"https://app.example.com/callback"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp domain.CreatePaymentResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "pi_test123", resp.PaymentIntentID)

	mockRepo.AssertExpectations(t)
	mockStatusUpdater.AssertExpectations(t)
	mockPSP.AssertExpectations(t)
}

func TestCreatePayment_MissingIdempotencyKey(t *testing.T) {
	t.Parallel()

	h := payment_http.NewHandler(
		new(domain.MockPaymentRepository),
		new(domain.MockOrderStatusUpdater),
		new(domain.MockPSPClient),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"order_id":"` + uuid.New().String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreatePayment_PSPFailure(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()
	mockRepo := new(domain.MockPaymentRepository)
	mockStatusUpdater := new(domain.MockOrderStatusUpdater)
	mockPSP := new(domain.MockPSPClient)

	mockRepo.
		On("Create", mock.Anything, mock.Anything).
		Return(nil)
	mockRepo.
		On("UpdateStatus", mock.Anything, mock.Anything, domain.PaymentStatusFailed, mock.Anything).
		Return(nil)

	mockStatusUpdater.
		On("GetOrderAmount", mock.Anything, orderID).
		Return(50000, nil)

	mockPSP.
		On("InitiatePayment", mock.Anything, 50000, mock.Anything).
		Return(nil, domain.ErrPSPUnavailable)

	h := payment_http.NewHandler(
		mockRepo,
		mockStatusUpdater,
		mockPSP,
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"order_id":"` + orderID.String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	mockRepo.AssertExpectations(t)
	mockStatusUpdater.AssertExpectations(t)
	mockPSP.AssertExpectations(t)
}

// --- HandleWebhook Tests ---

func TestHandleWebhook_Succeeded(t *testing.T) {
	t.Parallel()

	paymentID := uuid.New()
	orderID := uuid.New()

	mockRepo := new(domain.MockPaymentRepository)
	mockStatusUpdater := new(domain.MockOrderStatusUpdater)

	mockRepo.
		On("GetByPaymentIntentID", mock.Anything, "pi_test").
		Return(&domain.Payment{
			ID:      paymentID,
			OrderID: orderID,
			Status:  domain.PaymentStatusProcessing,
		}, nil)
	mockRepo.
		On("UpdateStatus", mock.Anything, paymentID, domain.PaymentStatusSucceeded, (*string)(nil)).
		Return(nil)

	mockStatusUpdater.
		On("UpdateOrderStatus", mock.Anything, orderID, "accepted").
		Return(nil)

	h := payment_http.NewHandler(
		mockRepo,
		mockStatusUpdater,
		new(domain.MockPSPClient),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"payment_intent_id":"pi_test","status":"succeeded"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
	mockStatusUpdater.AssertExpectations(t)
}

func TestHandleWebhook_Failed(t *testing.T) {
	t.Parallel()

	paymentID := uuid.New()
	orderID := uuid.New()
	failureReason := "insufficient_funds"

	mockRepo := new(domain.MockPaymentRepository)
	mockStatusUpdater := new(domain.MockOrderStatusUpdater)

	mockRepo.
		On("GetByPaymentIntentID", mock.Anything, "pi_test").
		Return(&domain.Payment{
			ID:      paymentID,
			OrderID: orderID,
			Status:  domain.PaymentStatusProcessing,
		}, nil)
	mockRepo.
		On("UpdateStatus", mock.Anything, paymentID, domain.PaymentStatusFailed, &failureReason).
		Return(nil)

	mockStatusUpdater.
		On("UpdateOrderStatus", mock.Anything, orderID, "payment_failed").
		Return(nil)

	h := payment_http.NewHandler(
		mockRepo,
		mockStatusUpdater,
		new(domain.MockPSPClient),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"payment_intent_id":"pi_test","status":"failed","failure_reason":"insufficient_funds"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
	mockStatusUpdater.AssertExpectations(t)
}

func TestHandleWebhook_AlreadyTerminal(t *testing.T) {
	t.Parallel()

	mockRepo := new(domain.MockPaymentRepository)

	mockRepo.
		On("GetByPaymentIntentID", mock.Anything, "pi_test").
		Return(&domain.Payment{
			ID:     uuid.New(),
			Status: domain.PaymentStatusSucceeded,
		}, nil)

	h := payment_http.NewHandler(
		mockRepo,
		new(domain.MockOrderStatusUpdater),
		new(domain.MockPSPClient),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"payment_intent_id":"pi_test","status":"succeeded"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestHandleWebhook_NotFound(t *testing.T) {
	t.Parallel()

	mockRepo := new(domain.MockPaymentRepository)

	mockRepo.
		On("GetByPaymentIntentID", mock.Anything, "pi_nonexistent").
		Return(nil, domain.ErrNotFound)

	h := payment_http.NewHandler(
		mockRepo,
		new(domain.MockOrderStatusUpdater),
		new(domain.MockPSPClient),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"payment_intent_id":"pi_nonexistent","status":"succeeded"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockRepo.AssertExpectations(t)
}
