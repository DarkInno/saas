package plan

import "errors"

var (
	// ErrPlanNotFound reports that a plan does not exist.
	ErrPlanNotFound = errors.New("gotenancy/plan: plan not found")

	// ErrPlanAlreadyExists reports that a plan already exists.
	ErrPlanAlreadyExists = errors.New("gotenancy/plan: plan already exists")

	// ErrPlanConflict reports that a concurrent writer replaced a plan during update.
	ErrPlanConflict = errors.New("gotenancy/plan: plan update conflict")

	// ErrInvalidPlan reports invalid plan metadata.
	ErrInvalidPlan = errors.New("gotenancy/plan: invalid plan")

	// ErrInvalidListFilter reports an invalid plan list filter.
	ErrInvalidListFilter = errors.New("gotenancy/plan: invalid list filter")

	// ErrNilDB reports that a SQL store was created with a nil database handle.
	ErrNilDB = errors.New("gotenancy/plan: nil db")

	// ErrInvalidTableName reports an unsafe SQL table name.
	ErrInvalidTableName = errors.New("gotenancy/plan: invalid table name")

	// ErrUnsupportedSQLDialect reports an unsupported SQLStore dialect.
	ErrUnsupportedSQLDialect = errors.New("gotenancy/plan: unsupported sql dialect")
)
