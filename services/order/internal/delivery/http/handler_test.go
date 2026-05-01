package http_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	order_http "github.com/ichinosekei/highload/services/order/internal/delivery/http"
	"github.com/ichinosekei/highload/services/order/internal/domain"
)

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func setupRouter(h *order_http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", h.RegisterRoutes)
	return r
}

func noopPublisher() *domain.MockEventPublisher {
	m := new(domain.MockEventPublisher)
	m.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	return m
}

func noopCart() *domain.MockCartRepository {
	m := new(domain.MockCartRepository)
	m.On("Get", mock.Anything, mock.Anything).Return([]domain.OrderItem(nil), nil)
	m.On("Set", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	m.On("Delete", mock.Anything, mock.Anything).Return(nil)
	return m
}

// --- CreateOrder Tests ---

func TestCreateOrder_OK(t *testing.T) {
	t.Parallel()

	mockRepo := new(domain.MockOrderRepository)
	mockRepo.
		On("Create", mock.Anything, mock.Anything).
		Return(nil)

	h := order_http.NewHandler(
		mockRepo,
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"restaurant_id":"0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901","items":[{"menu_item_id":"0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001","quantity":2}],"delivery_address":{"address_text":"Test","lat":55.75,"lon":37.62}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestCreateOrder_MissingIdempotencyKey(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		new(domain.MockOrderRepository),
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"restaurant_id":"0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901","items":[{"menu_item_id":"0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001","quantity":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrder_EmptyItems(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		new(domain.MockOrderRepository),
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"restaurant_id":"0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901","items":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestCreateOrder_IdempotencyConflict(t *testing.T) {
	t.Parallel()

	existingOrder := &domain.Order{
		ID:     uuid.New(),
		Status: domain.StatusCreated,
	}
	idempotencyKey := uuid.New()

	mockRepo := new(domain.MockOrderRepository)
	mockRepo.
		On("Create", mock.Anything, mock.Anything).
		Return(domain.ErrIdempotencyConflict)
	mockRepo.
		On("GetByIdempotencyKey", mock.Anything, idempotencyKey).
		Return(existingOrder, nil)

	h := order_http.NewHandler(
		mockRepo,
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"restaurant_id":"0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901","items":[{"menu_item_id":"0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001","quantity":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", idempotencyKey.String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestCreateOrder_InternalError(t *testing.T) {
	t.Parallel()

	mockRepo := new(domain.MockOrderRepository)
	mockRepo.
		On("Create", mock.Anything, mock.Anything).
		Return(errTest)

	h := order_http.NewHandler(
		mockRepo,
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"restaurant_id":"0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901","items":[{"menu_item_id":"0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001","quantity":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	mockRepo.AssertExpectations(t)
}

// --- UpdateOrderStatus Tests ---

func TestUpdateOrderStatus_OK(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()
	mockRepo := new(domain.MockOrderRepository)
	mockRepo.
		On("UpdateStatus", mock.Anything, orderID, domain.StatusCooking).
		Return(nil)

	h := order_http.NewHandler(
		mockRepo,
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"status":"cooking"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/orders/"+orderID.String()+"/status",
		bytes.NewBufferString(body),
	)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockRepo.AssertExpectations(t)
}

func TestUpdateOrderStatus_InvalidID(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		new(domain.MockOrderRepository),
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"status":"cooking"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/not-a-uuid/status", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateOrderStatus_InvalidStatus(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		new(domain.MockOrderRepository),
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"status":"invalid_status"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/orders/"+uuid.New().String()+"/status",
		bytes.NewBufferString(body),
	)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateOrderStatus_NotFound(t *testing.T) {
	t.Parallel()

	mockRepo := new(domain.MockOrderRepository)
	mockRepo.
		On("UpdateStatus", mock.Anything, mock.Anything, mock.Anything).
		Return(domain.ErrNotFound)

	h := order_http.NewHandler(
		mockRepo,
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"status":"cooking"}`
	req := httptest.NewRequest(
		http.MethodPost,
		"/api/v1/orders/"+uuid.New().String()+"/status",
		bytes.NewBufferString(body),
	)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockRepo.AssertExpectations(t)
}

// --- TrackOrder Tests ---

func TestTrackOrder_OK(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()
	testOrder := &domain.Order{
		ID:     orderID,
		Status: domain.StatusCooking,
	}

	mockRepo := new(domain.MockOrderRepository)
	mockRepo.
		On("GetByID", mock.Anything, orderID).
		Return(testOrder, nil)

	h := order_http.NewHandler(
		mockRepo,
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String()+"/track", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp domain.TrackingResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, domain.StatusCooking, resp.Status)

	mockRepo.AssertExpectations(t)
}

func TestTrackOrder_NotFound(t *testing.T) {
	t.Parallel()

	mockRepo := new(domain.MockOrderRepository)
	mockRepo.
		On("GetByID", mock.Anything, mock.Anything).
		Return(nil, domain.ErrNotFound)

	h := order_http.NewHandler(
		mockRepo,
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+uuid.New().String()+"/track", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	mockRepo.AssertExpectations(t)
}
