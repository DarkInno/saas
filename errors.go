package saas

import "errors"

var (
	// ErrNoTenant reports that a tenant-scoped operation was called without a tenant context.
	ErrNoTenant = errors.New("saas: no tenant in context")

	// ErrForbidden reports that the current tenant or side is not allowed to perform an operation.
	ErrForbidden = errors.New("saas: forbidden")

	// ErrInvalidState reports an invalid tenant lifecycle transition or unsupported state.
	ErrInvalidState = errors.New("saas: invalid state")

	// ErrHostRequired reports that an operation requires explicit host-side context.
	ErrHostRequired = errors.New("saas: host context required")

	// ErrTenantMismatch reports that a resource belongs to a different tenant than the active context.
	ErrTenantMismatch = errors.New("saas: tenant mismatch")
)
