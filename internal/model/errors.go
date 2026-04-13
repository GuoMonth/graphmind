package model

import "errors"

// Sentinel errors for domain-level error handling.
// CLI layer maps these to exit codes.
var (
	ErrNotFound     = errors.New("not found")
	ErrConflict     = errors.New("conflict")
	ErrInvalidInput = errors.New("invalid input")
	ErrInvalidState = errors.New("invalid state")
)
