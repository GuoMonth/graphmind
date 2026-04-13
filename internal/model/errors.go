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

// HintedError wraps an error with an AI-friendly hint for next steps.
type HintedError struct {
	Err  error
	Hint string
}

func (h *HintedError) Error() string { return h.Err.Error() }
func (h *HintedError) Unwrap() error { return h.Err }

// WithHint attaches an AI-friendly hint to an error.
func WithHint(err error, hint string) error {
	return &HintedError{Err: err, Hint: hint}
}

// GetHint extracts the hint from an error chain, or returns "".
func GetHint(err error) string {
	var h *HintedError
	if errors.As(err, &h) {
		return h.Hint
	}
	return ""
}
