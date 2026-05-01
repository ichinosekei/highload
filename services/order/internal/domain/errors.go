package domain

import "errors"

var (
	ErrNotFound                = errors.New("not found")
	ErrInvalidInput            = errors.New("invalid input")
	ErrInvalidStatusTransition = errors.New("invalid status transition")
	ErrIdempotencyConflict     = errors.New("idempotency conflict")
)
