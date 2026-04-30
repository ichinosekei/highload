package domain

import "errors"

var (
	ErrNotFound            = errors.New("not found")
	ErrInvalidInput        = errors.New("invalid input")
	ErrIdempotencyConflict = errors.New("idempotency conflict")
	ErrInvalidOrderStatus  = errors.New("invalid order status for payment")
	ErrPSPUnavailable      = errors.New("payment provider unavailable")
)
