package onboarding

import "errors"

var (
	// ErrInvalidInput reports missing required onboarding dependencies or input fields.
	ErrInvalidInput = errors.New("saas/onboarding: invalid input")
)
