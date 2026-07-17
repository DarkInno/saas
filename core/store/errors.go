package store

import "errors"

var (
	// ErrTenantNotFound reports that a tenant does not exist in the store.
	ErrTenantNotFound = errors.New("saas/store: tenant not found")

	// ErrTenantAlreadyExists reports that a tenant already exists in the store.
	ErrTenantAlreadyExists = errors.New("saas/store: tenant already exists")

	// ErrTenantConflict reports that tenant metadata changed after it was read.
	ErrTenantConflict = errors.New("saas/store: tenant update conflict")

	// ErrInvalidTenant reports that tenant metadata is not valid for persistence.
	ErrInvalidTenant = errors.New("saas/store: invalid tenant")

	// ErrInvalidListFilter reports an invalid tenant list filter.
	ErrInvalidListFilter = errors.New("saas/store: invalid list filter")

	// ErrNilStore reports that a store dependency is nil.
	ErrNilStore = errors.New("saas/store: nil store")

	// ErrNilCache reports that a cache dependency is nil.
	ErrNilCache = errors.New("saas/store: nil cache")

	// ErrNilDB reports that a SQL store was created with a nil database handle.
	ErrNilDB = errors.New("saas/store: nil db")

	// ErrInvalidTableName reports an unsafe SQL table name.
	ErrInvalidTableName = errors.New("saas/store: invalid table name")

	// ErrUnsupportedSQLDialect reports an unsupported SQLStore dialect.
	ErrUnsupportedSQLDialect = errors.New("saas/store: unsupported sql dialect")

	// ErrInvalidCacheSize reports an invalid bounded memory cache size.
	ErrInvalidCacheSize = errors.New("saas/store: invalid cache size")
)
