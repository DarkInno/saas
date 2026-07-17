package rbac

import "errors"

var (
	ErrInvalidRole    = errors.New("saas/rbac: invalid role")
	ErrRoleNotFound   = errors.New("saas/rbac: role not found")
	ErrRoleExists     = errors.New("saas/rbac: role already exists")
	ErrPermissionDeny = errors.New("saas/rbac: permission denied")

	ErrNilDB = errors.New("saas/rbac: nil db")

	ErrInvalidTableName = errors.New("saas/rbac: invalid table name")

	ErrUnsupportedSQLDialect = errors.New("saas/rbac: unsupported sql dialect")
)
