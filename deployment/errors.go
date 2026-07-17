package deployment

import "errors"

var (
	// ErrNilStore reports that a deployment service has no backing store.
	ErrNilStore = errors.New("saas/deployment: nil store")

	// ErrNilDB reports that a SQL store was created with a nil database handle.
	ErrNilDB = errors.New("saas/deployment: nil db")

	// ErrInvalidTableName reports an unsafe SQL table name.
	ErrInvalidTableName = errors.New("saas/deployment: invalid table name")

	// ErrUnsupportedSQLDialect reports an unsupported SQL store dialect.
	ErrUnsupportedSQLDialect = errors.New("saas/deployment: unsupported sql dialect")

	// ErrInvalidDeploymentUnit reports invalid deployment unit metadata.
	ErrInvalidDeploymentUnit = errors.New("saas/deployment: invalid deployment unit")

	// ErrDeploymentUnitNotFound reports a missing deployment unit.
	ErrDeploymentUnitNotFound = errors.New("saas/deployment: deployment unit not found")

	// ErrDeploymentUnitAlreadyExists reports a duplicate deployment unit ID.
	ErrDeploymentUnitAlreadyExists = errors.New("saas/deployment: deployment unit already exists")

	// ErrDeploymentUnitConflict reports a concurrent deployment unit replacement.
	ErrDeploymentUnitConflict = errors.New("saas/deployment: deployment unit conflict")

	// ErrDeploymentUnitUnavailable reports a unit that cannot accept or resolve
	// tenant traffic because it is not active.
	ErrDeploymentUnitUnavailable = errors.New("saas/deployment: deployment unit unavailable")

	// ErrDeploymentUnitInUse reports an attempt to disable or delete a unit
	// referenced by a current assignment or prepared move.
	ErrDeploymentUnitInUse = errors.New("saas/deployment: deployment unit in use")

	// ErrInvalidAssignment reports an invalid tenant deployment assignment.
	ErrInvalidAssignment = errors.New("saas/deployment: invalid assignment")

	// ErrAssignmentNotFound reports a tenant without a deployment assignment.
	ErrAssignmentNotFound = errors.New("saas/deployment: assignment not found")

	// ErrAssignmentAlreadyExists reports an attempt to create a second initial
	// assignment for a tenant.
	ErrAssignmentAlreadyExists = errors.New("saas/deployment: assignment already exists")

	// ErrAssignmentConflict reports that an assignment changed after it was read.
	ErrAssignmentConflict = errors.New("saas/deployment: assignment conflict")

	// ErrInvalidMove reports an invalid deployment move request.
	ErrInvalidMove = errors.New("saas/deployment: invalid move")

	// ErrMoveNotFound reports a tenant without a prepared move.
	ErrMoveNotFound = errors.New("saas/deployment: move not found")

	// ErrMoveAlreadyExists reports an attempt to prepare a second move for a tenant.
	ErrMoveAlreadyExists = errors.New("saas/deployment: move already exists")

	// ErrMoveConflict reports an assignment that no longer matches a prepared move.
	ErrMoveConflict = errors.New("saas/deployment: move conflict")

	// ErrPolicyDenied reports a host policy rejecting a deployment unit.
	ErrPolicyDenied = errors.New("saas/deployment: policy denied")
)
