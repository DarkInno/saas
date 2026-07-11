package sqlxtenant

import "errors"

var (
	ErrUnsafeSQL = errors.New("gotenancy/sqlx: unsafe sql")

	// ErrTenantFieldUpdate reports an attempt to update the tenant partition key from a tenant context.
	ErrTenantFieldUpdate = errors.New("gotenancy/sqlx: tenant field cannot be updated in tenant context")
)
