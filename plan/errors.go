package plan

import "errors"

var (
	// ErrPlanNotFound reports that a plan does not exist.
	ErrPlanNotFound = errors.New("saas/plan: plan not found")

	// ErrPlanAlreadyExists reports that a plan already exists.
	ErrPlanAlreadyExists = errors.New("saas/plan: plan already exists")

	// ErrPlanConflict reports that a concurrent writer replaced a plan during update.
	ErrPlanConflict = errors.New("saas/plan: plan update conflict")

	// ErrInvalidPlan reports invalid plan metadata.
	ErrInvalidPlan = errors.New("saas/plan: invalid plan")

	// ErrInvalidListFilter reports an invalid plan list filter.
	ErrInvalidListFilter = errors.New("saas/plan: invalid list filter")

	// ErrNilDB reports that a SQL store was created with a nil database handle.
	ErrNilDB = errors.New("saas/plan: nil db")

	// ErrInvalidTableName reports an unsafe SQL table name.
	ErrInvalidTableName = errors.New("saas/plan: invalid table name")

	// ErrUnsupportedSQLDialect reports an unsupported SQLStore dialect.
	ErrUnsupportedSQLDialect = errors.New("saas/plan: unsupported sql dialect")
)
