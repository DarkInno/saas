package audit

import "errors"

var (
	ErrInvalidEvent = errors.New("gotenancy/audit: invalid event")

	ErrInvalidListFilter = errors.New("gotenancy/audit: invalid list filter")

	ErrNilDB = errors.New("gotenancy/audit: nil db")

	ErrInvalidTableName = errors.New("gotenancy/audit: invalid table name")

	ErrUnsupportedSQLDialect = errors.New("gotenancy/audit: unsupported sql dialect")
)
