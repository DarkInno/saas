package quota

import "errors"

var (
	// ErrInvalidQuota reports invalid quota input.
	ErrInvalidQuota = errors.New("gotenancy/quota: invalid quota")

	// ErrQuotaExceeded reports a quota limit violation.
	ErrQuotaExceeded = errors.New("gotenancy/quota: quota exceeded")

	// ErrNilStore reports that a quota service was created with a nil store.
	ErrNilStore = errors.New("gotenancy/quota: nil store")

	// ErrNilDB reports that a SQL store was created with a nil database handle.
	ErrNilDB = errors.New("gotenancy/quota: nil db")

	// ErrInvalidTableName reports an unsafe SQL table name.
	ErrInvalidTableName = errors.New("gotenancy/quota: invalid table name")

	// ErrUnsupportedSQLDialect reports an unsupported SQLStore dialect.
	ErrUnsupportedSQLDialect = errors.New("gotenancy/quota: unsupported sql dialect")
)
