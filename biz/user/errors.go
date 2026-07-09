package user

import "errors"

var (
	ErrInvalidUser    = errors.New("gotenancy/user: invalid user")
	ErrUserNotFound   = errors.New("gotenancy/user: user not found")
	ErrUserExists     = errors.New("gotenancy/user: user already exists")
	ErrMemberNotFound = errors.New("gotenancy/user: member not found")
	ErrMemberExists   = errors.New("gotenancy/user: member already exists")
	ErrTenantMismatch = errors.New("gotenancy/user: tenant mismatch")

	ErrNilDB = errors.New("gotenancy/user: nil db")

	ErrInvalidTableName = errors.New("gotenancy/user: invalid table name")

	ErrUnsupportedSQLDialect = errors.New("gotenancy/user: unsupported sql dialect")
)
