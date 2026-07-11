package enttenant

import "errors"

var (
	// ErrNilQuery reports that a nil Ent query was passed to the tenant filter.
	ErrNilQuery = errors.New("gotenancy/ent: nil query")

	// ErrNilMutation reports that a nil Ent mutation was passed to the tenant filter.
	ErrNilMutation = errors.New("gotenancy/ent: nil mutation")

	// ErrUnsupportedMutation reports that an Ent mutation does not expose storage predicates.
	ErrUnsupportedMutation = errors.New("gotenancy/ent: unsupported mutation")

	// ErrTenantFieldNotFound reports that the configured tenant field does not exist on the mutation.
	ErrTenantFieldNotFound = errors.New("gotenancy/ent: tenant field not found")

	// ErrTenantFieldUpdate reports an attempt to update the tenant partition key from a tenant context.
	ErrTenantFieldUpdate = errors.New("gotenancy/ent: tenant field cannot be updated in tenant context")
)
