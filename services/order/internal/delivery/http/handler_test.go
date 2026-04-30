package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	order_http "github.com/ichinosekei/highload/services/order/internal/delivery/http"
	"github.com/ichinosekei/highload/services/order/internal/domain"
)

// --- Mock implementations ---

type mockOrderRepository struct {
	createFn              func(ctx context.Context, order *domain.Order) error
	getByIDFn             func(ctx context.Context, id uuid.UUID) (*domain.Order, error)
	updateStatusFn        func(ctx context.Context, id uuid.UUID, status domain.OrderStatus) error
	getByIdempotencyKeyFn func(ctx context.Context, key uuid.UUID) (*domain.Order, error)
}

func (m *mockOrderRepository) Create(ctx context.Context, order *domain.Order) error {
	return m.createFn(ctx, order)
}

func (m *mockOrderRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Order, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockOrderRepository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status domain.OrderStatus,
) error {
	return m.updateStatusFn(ctx, id, status)
}

func (m *mockOrderRepository) GetByIdempotencyKey(
	ctx context.Context,
	key uuid.UUID,
) (*domain.Order, error) {
	return m.getByIdempotencyKeyFn(ctx, key)
}

type mockCartRepository struct {
	getFn    func(ctx context.Context, userID uuid.UUID) ([]domain.OrderItem, error)
	setFn    func(ctx context.Context, userID uuid.UUID, items []domain.OrderItem) error
	deleteFn func(ctx context.Context, userID uuid.UUID) error
}

func (m *mockCartRepository) Get(ctx context.Context, userID uuid.UUID) ([]domain.OrderItem, error) {
	return m.getFn(ctx, userID)
}

func (m *mockCartRepository) Set(
	ctx context.Context,
	userID uuid.UUID,
	items []domain.OrderItem,
) error {
	return m.setFn(ctx, userID, items)
}

func (m *mockCartRepository) Delete(ctx context.Context, userID uuid.UUID) error {
	return m.deleteFn(ctx, userID)
}

type mockEventPublisher struct {
	publishFn func(ctx context.Context, subject string, data []byte) error
}

func (m *mockEventPublisher) Publish(ctx context.Context, subject string, data []byte) error {
	return m.publishFn(ctx, subject, data)
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func setupRouter(h *order_http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", h.RegisterRoutes)
	return r
}

func noopPublisher() *mockEventPublisher {
	return &mockEventPublisher{
		publishFn: func(_ context.Context, _ string, _ []byte) error { return nil },
	}
}

func noopCart() *mockCartRepository {
	return &mockCartRepository{
		getFn:    func(_ context.Context, _ uuid.UUID) ([]domain.OrderItem, error) { return nil, nil },
		setFn:    func(_ context.Context, _ uuid.UUID, _ []domain.OrderItem) error { return nil },
		deleteFn: func(_ context.Context, _ uuid.UUID) error { return nil },
	}
}

// --- CreateOrder Tests ---

func TestCreateOrder_OK(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		&mockOrderRepository{
			createFn: func(_ context.Context, _ *domain.Order) error {
				return nil
			},
		},
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"restaurant_id":"0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901","items":[{"menu_item_id":"0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001","quantity":2}],"delivery_address":{"address_text":"Test","lat":55.75,"lon":37.62}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d; want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}
}

func TestCreateOrder_MissingIdempotencyKey(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		&mockOrderRepository{},
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"restaurant_id":"0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901","items":[{"menu_item_id":"0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001","quantity":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateOrder_EmptyItems(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		&mockOrderRepository{},
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"restaurant_id":"0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901","items":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateOrder_IdempotencyConflict(t *testing.T) {
	t.Parallel()

	existingOrder := &domain.Order{
		ID:     uuid.New(),
		Status: domain.StatusCreated,
	}
	idempotencyKey := uuid.New()

	h := order_http.NewHandler(
		&mockOrderRepository{
			createFn: func(_ context.Context, _ *domain.Order) error {
				return domain.ErrIdempotencyConflict
			},
			getByIdempotencyKeyFn: func(_ context.Context, _ uuid.UUID) (*domain.Order, error) {
				return existingOrder, nil
			},
		},
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"restaurant_id":"0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901","items":[{"menu_item_id":"0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001","quantity":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", idempotencyKey.String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("got status %d; want %d; body: %s", w.Code, http.StatusConflict, w.Body.String())
	}
}

func TestCreateOrder_InternalError(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		&mockOrderRepository{
			createFn: func(_ context.Context, _ *domain.Order) error {
				return errTest
			},
		},
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"restaurant_id":"0196ca5b-8fd3-7c09-b2f4-a4f3b6c8d901","items":[{"menu_item_id":"0196ca5b-8fd3-7c09-b2f4-b4f3b6c8d001","quantity":1}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusInternalServerError)
	}
}

// --- UpdateOrderStatus Tests ---

func TestUpdateOrderStatus_OK(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()

	h := order_http.NewHandler(
		&mockOrderRepository{
			updateStatusFn: func(_ context.Context, id uuid.UUID, status domain.OrderStatus) error {
				if id != orderID {
					t.Errorf("got id %s; want %s", id, orderID)
				}
				if status != domain.StatusCooking {
					t.Errorf("got status %q; want %q", status, domain.StatusCooking)
				}
				return nil
			},
		},
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

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestUpdateOrderStatus_InvalidID(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		&mockOrderRepository{},
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"status":"cooking"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orders/not-a-uuid/status", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateOrderStatus_InvalidStatus(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		&mockOrderRepository{},
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

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusBadRequest)
	}
}

func TestUpdateOrderStatus_NotFound(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		&mockOrderRepository{
			updateStatusFn: func(_ context.Context, _ uuid.UUID, _ domain.OrderStatus) error {
				return domain.ErrNotFound
			},
		},
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

	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusNotFound)
	}
}

// --- TrackOrder Tests ---

func TestTrackOrder_OK(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()
	testOrder := &domain.Order{
		ID:     orderID,
		Status: domain.StatusCooking,
	}

	h := order_http.NewHandler(
		&mockOrderRepository{
			getByIDFn: func(_ context.Context, id uuid.UUID) (*domain.Order, error) {
				if id != orderID {
					t.Errorf("got id %s; want %s", id, orderID)
				}
				return testOrder, nil
			},
		},
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+orderID.String()+"/track", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	var resp domain.TrackingResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != domain.StatusCooking {
		t.Errorf("got status %q; want %q", resp.Status, domain.StatusCooking)
	}
}

func TestTrackOrder_NotFound(t *testing.T) {
	t.Parallel()

	h := order_http.NewHandler(
		&mockOrderRepository{
			getByIDFn: func(_ context.Context, _ uuid.UUID) (*domain.Order, error) {
				return nil, domain.ErrNotFound
			},
		},
		noopCart(),
		noopPublisher(),
		newTestLogger(),
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders/"+uuid.New().String()+"/track", nil)
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusNotFound)
	}
}
