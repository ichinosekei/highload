package resilience

import (
	"context"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/sony/gobreaker/v2"
)

const (
	defaultMaxElapsedTime = 10 * time.Second
	defaultMaxRequests    = 3
	defaultInterval       = 10 * time.Second
	defaultTimeout        = 30 * time.Second
	minRequestsToTrip     = 5
	failureRatioThreshold = 0.5
)

// Retry executes a function with exponential backoff.
func Retry(ctx context.Context, fn func() error) error {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = defaultMaxElapsedTime

	return backoff.Retry(fn, backoff.WithContext(b, ctx))
}

// NewDefaultCircuitBreaker creates a circuit breaker with default settings.
func NewDefaultCircuitBreaker[T any](name string) *gobreaker.CircuitBreaker[T] {
	return gobreaker.NewCircuitBreaker[T](gobreaker.Settings{
		Name:        name,
		MaxRequests: defaultMaxRequests,
		Interval:    defaultInterval,
		Timeout:     defaultTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= minRequestsToTrip && failureRatio >= failureRatioThreshold
		},
	})
}
