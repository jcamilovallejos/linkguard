// Package domain holds the entities, ports (interfaces), and sentinel
// errors shared across linkguard's business logic. It has no dependency on
// any framework or infrastructure detail, which is what lets the usecase
// layer be unit-tested without a real Postgres or Redis instance.
package domain

import "errors"

var (
	// ErrNotFound is returned when a requested short code does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict is returned when an operation collides with existing state.
	ErrConflict = errors.New("conflict")
	// ErrInvalidInput is returned when caller-provided input fails validation.
	ErrInvalidInput = errors.New("invalid input")
)
