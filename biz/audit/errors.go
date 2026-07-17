package audit

import "errors"

var (
	ErrInvalidEvent = errors.New("saas/audit: invalid event")

	ErrInvalidListFilter = errors.New("saas/audit: invalid list filter")

	ErrNilDB = errors.New("saas/audit: nil db")

	ErrInvalidTableName = errors.New("saas/audit: invalid table name")

	ErrUnsupportedSQLDialect = errors.New("saas/audit: unsupported sql dialect")
)
