package domain

import (
	"context"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

type MockRestaurantReader struct {
	mock.Mock
}

func (m *MockRestaurantReader) List(ctx context.Context, limit, offset int64) ([]Restaurant, error) {
	args := m.Called(ctx, limit, offset)
	res := args.Get(0)
	if res == nil {
		return nil, args.Error(1)
	}
	return res.([]Restaurant), args.Error(1) //nolint:errcheck // mock type assertion
}

func (m *MockRestaurantReader) GetByID(ctx context.Context, id uuid.UUID) (*Restaurant, error) {
	args := m.Called(ctx, id)
	res := args.Get(0)
	if res == nil {
		return nil, args.Error(1)
	}
	return res.(*Restaurant), args.Error(1) //nolint:errcheck // mock type assertion
}

type MockMenuReader struct {
	mock.Mock
}

func (m *MockMenuReader) ListByRestaurant(ctx context.Context, restaurantID uuid.UUID) ([]MenuItem, error) {
	args := m.Called(ctx, restaurantID)
	res := args.Get(0)
	if res == nil {
		return nil, args.Error(1)
	}
	return res.([]MenuItem), args.Error(1) //nolint:errcheck // mock type assertion
}

type MockRestaurantSearcher struct {
	mock.Mock
}

func (m *MockRestaurantSearcher) Search(ctx context.Context, params SearchParams) (*SearchResult, error) {
	args := m.Called(ctx, params)
	res := args.Get(0)
	if res == nil {
		return nil, args.Error(1)
	}
	return res.(*SearchResult), args.Error(1) //nolint:errcheck // mock type assertion
}

type MockMenuRestaurantReader struct {
	*MockRestaurantReader
	*MockMenuReader
}
