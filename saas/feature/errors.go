package feature

import "errors"

var (
	// ErrInvalidFeature reports invalid feature input.
	ErrInvalidFeature = errors.New("gotenancy/feature: invalid feature")

	// ErrFeatureNotFound reports a missing feature flag.
	ErrFeatureNotFound = errors.New("gotenancy/feature: feature not found")

	// ErrNilDB reports that a SQL store was created with a nil database handle.
	ErrNilDB = errors.New("gotenancy/feature: nil db")

	// ErrInvalidTableName reports an unsafe SQL table name.
	ErrInvalidTableName = errors.New("gotenancy/feature: invalid table name")

	// ErrUnsupportedSQLDialect reports an unsupported SQLStore dialect.
	ErrUnsupportedSQLDialect = errors.New("gotenancy/feature: unsupported sql dialect")
)
