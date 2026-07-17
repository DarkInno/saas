package feature

import "errors"

var (
	// ErrInvalidFeature reports invalid feature input.
	ErrInvalidFeature = errors.New("saas/feature: invalid feature")

	// ErrFeatureNotFound reports a missing feature flag.
	ErrFeatureNotFound = errors.New("saas/feature: feature not found")

	// ErrNilDB reports that a SQL store was created with a nil database handle.
	ErrNilDB = errors.New("saas/feature: nil db")

	// ErrInvalidTableName reports an unsafe SQL table name.
	ErrInvalidTableName = errors.New("saas/feature: invalid table name")

	// ErrUnsupportedSQLDialect reports an unsupported SQLStore dialect.
	ErrUnsupportedSQLDialect = errors.New("saas/feature: unsupported sql dialect")
)
