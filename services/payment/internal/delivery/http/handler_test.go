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

	payment_http "github.com/ichinosekei/highload/services/payment/internal/delivery/http"
	"github.com/ichinosekei/highload/services/payment/internal/domain"
)

// --- Mock implementations ---

type mockPaymentRepository struct {
	createFn              func(ctx context.Context, payment *domain.Payment) error
	getByIDFn             func(ctx context.Context, id uuid.UUID) (*domain.Payment, error)
	getByIdempotencyKeyFn func(ctx context.Context, key uuid.UUID) (*domain.Payment, error)
	getByPaymentIntentFn  func(ctx context.Context, intentID string) (*domain.Payment, error)
	updateStatusFn        func(ctx context.Context, id uuid.UUID, status domain.PaymentStatus, reason *string) error
	updatePaymentIntentFn func(ctx context.Context, id uuid.UUID, intentID string) error
}

func (m *mockPaymentRepository) Create(ctx context.Context, payment *domain.Payment) error {
	return m.createFn(ctx, payment)
}

func (m *mockPaymentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Payment, error) {
	return m.getByIDFn(ctx, id)
}

func (m *mockPaymentRepository) GetByIdempotencyKey(
	ctx context.Context,
	key uuid.UUID,
) (*domain.Payment, error) {
	return m.getByIdempotencyKeyFn(ctx, key)
}

func (m *mockPaymentRepository) GetByPaymentIntentID(
	ctx context.Context,
	intentID string,
) (*domain.Payment, error) {
	return m.getByPaymentIntentFn(ctx, intentID)
}

func (m *mockPaymentRepository) UpdateStatus(
	ctx context.Context,
	id uuid.UUID,
	status domain.PaymentStatus,
	reason *string,
) error {
	return m.updateStatusFn(ctx, id, status, reason)
}

func (m *mockPaymentRepository) UpdatePaymentIntent(
	ctx context.Context,
	id uuid.UUID,
	intentID string,
) error {
	return m.updatePaymentIntentFn(ctx, id, intentID)
}

type mockOrderStatusUpdater struct {
	updateOrderStatusFn func(ctx context.Context, orderID uuid.UUID, status string) error
	getOrderAmountFn    func(ctx context.Context, orderID uuid.UUID) (int, error)
}

func (m *mockOrderStatusUpdater) UpdateOrderStatus(
	ctx context.Context,
	orderID uuid.UUID,
	status string,
) error {
	return m.updateOrderStatusFn(ctx, orderID, status)
}

func (m *mockOrderStatusUpdater) GetOrderAmount(ctx context.Context, orderID uuid.UUID) (int, error) {
	return m.getOrderAmountFn(ctx, orderID)
}

type mockPSPClient struct {
	initiateFn func(ctx context.Context, amount int, returnURL string) (*domain.PSPResponse, error)
}

func (m *mockPSPClient) InitiatePayment(
	ctx context.Context,
	amount int,
	returnURL string,
) (*domain.PSPResponse, error) {
	return m.initiateFn(ctx, amount, returnURL)
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

func setupRouter(h *payment_http.Handler) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1", h.RegisterRoutes)
	return r
}

func noopPublisher() *mockEventPublisher {
	return &mockEventPublisher{
		publishFn: func(_ context.Context, _ string, _ []byte) error { return nil },
	}
}

// --- CreatePayment Tests ---

func TestCreatePayment_OK(t *testing.T) {
	t.Parallel()

	orderID := uuid.New()

	h := payment_http.NewHandler(
		&mockPaymentRepository{
			createFn:              func(_ context.Context, _ *domain.Payment) error { return nil },
			updatePaymentIntentFn: func(_ context.Context, _ uuid.UUID, _ string) error { return nil },
		},
		&mockOrderStatusUpdater{
			getOrderAmountFn: func(_ context.Context, _ uuid.UUID) (int, error) { return 100000, nil },
		},
		&mockPSPClient{
			initiateFn: func(_ context.Context, _ int, _ string) (*domain.PSPResponse, error) {
				return &domain.PSPResponse{
					PaymentIntentID: "pi_test123",
					RedirectURL:     "https://psp.example.com/pay/pi_test123",
					Status:          "requires_action",
				}, nil
			},
		},
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"order_id":"` + orderID.String() + `","payment_method":"card","return_url":"https://app.example.com/callback"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("got status %d; want %d; body: %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp domain.CreatePaymentResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.PaymentIntentID != "pi_test123" {
		t.Errorf("payment_intent_id = %q; want %q", resp.PaymentIntentID, "pi_test123")
	}
}

func TestCreatePayment_MissingIdempotencyKey(t *testing.T) {
	t.Parallel()

	h := payment_http.NewHandler(
		&mockPaymentRepository{},
		&mockOrderStatusUpdater{},
		&mockPSPClient{},
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"order_id":"` + uuid.New().String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreatePayment_PSPFailure(t *testing.T) {
	t.Parallel()

	h := payment_http.NewHandler(
		&mockPaymentRepository{
			createFn: func(_ context.Context, _ *domain.Payment) error { return nil },
			updateStatusFn: func(_ context.Context, _ uuid.UUID, _ domain.PaymentStatus, _ *string) error {
				return nil
			},
		},
		&mockOrderStatusUpdater{
			getOrderAmountFn: func(_ context.Context, _ uuid.UUID) (int, error) { return 50000, nil },
		},
		&mockPSPClient{
			initiateFn: func(_ context.Context, _ int, _ string) (*domain.PSPResponse, error) {
				return nil, domain.ErrPSPUnavailable
			},
		},
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"order_id":"` + uuid.New().String() + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments", bytes.NewBufferString(body))
	req.Header.Set("Idempotency-Key", uuid.New().String())
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusServiceUnavailable)
	}
}

// --- HandleWebhook Tests ---

func TestHandleWebhook_Succeeded(t *testing.T) {
	t.Parallel()

	paymentID := uuid.New()
	orderID := uuid.New()

	h := payment_http.NewHandler(
		&mockPaymentRepository{
			getByPaymentIntentFn: func(_ context.Context, _ string) (*domain.Payment, error) {
				return &domain.Payment{
					ID:      paymentID,
					OrderID: orderID,
					Status:  domain.PaymentStatusProcessing,
				}, nil
			},
			updateStatusFn: func(_ context.Context, _ uuid.UUID, _ domain.PaymentStatus, _ *string) error {
				return nil
			},
		},
		&mockOrderStatusUpdater{
			updateOrderStatusFn: func(_ context.Context, _ uuid.UUID, _ string) error { return nil },
		},
		&mockPSPClient{},
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"payment_intent_id":"pi_test","status":"succeeded"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandleWebhook_Failed(t *testing.T) {
	t.Parallel()

	h := payment_http.NewHandler(
		&mockPaymentRepository{
			getByPaymentIntentFn: func(_ context.Context, _ string) (*domain.Payment, error) {
				return &domain.Payment{
					ID:      uuid.New(),
					OrderID: uuid.New(),
					Status:  domain.PaymentStatusProcessing,
				}, nil
			},
			updateStatusFn: func(_ context.Context, _ uuid.UUID, _ domain.PaymentStatus, _ *string) error {
				return nil
			},
		},
		&mockOrderStatusUpdater{
			updateOrderStatusFn: func(_ context.Context, _ uuid.UUID, _ string) error { return nil },
		},
		&mockPSPClient{},
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"payment_intent_id":"pi_test","status":"failed","failure_reason":"insufficient_funds"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandleWebhook_AlreadyTerminal(t *testing.T) {
	t.Parallel()

	h := payment_http.NewHandler(
		&mockPaymentRepository{
			getByPaymentIntentFn: func(_ context.Context, _ string) (*domain.Payment, error) {
				return &domain.Payment{
					ID:     uuid.New(),
					Status: domain.PaymentStatusSucceeded, // Already terminal
				}, nil
			},
		},
		&mockOrderStatusUpdater{},
		&mockPSPClient{},
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"payment_intent_id":"pi_test","status":"succeeded"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusOK)
	}
}

func TestHandleWebhook_NotFound(t *testing.T) {
	t.Parallel()

	h := payment_http.NewHandler(
		&mockPaymentRepository{
			getByPaymentIntentFn: func(_ context.Context, _ string) (*domain.Payment, error) {
				return nil, domain.ErrNotFound
			},
		},
		&mockOrderStatusUpdater{},
		&mockPSPClient{},
		noopPublisher(),
		newTestLogger(),
	)

	body := `{"payment_intent_id":"pi_nonexistent","status":"succeeded"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/payments/webhook", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	setupRouter(h).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("got status %d; want %d", w.Code, http.StatusNotFound)
	}
}
