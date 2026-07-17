package user

import "errors"

var (
	ErrInvalidUser    = errors.New("saas/user: invalid user")
	ErrUserNotFound   = errors.New("saas/user: user not found")
	ErrUserExists     = errors.New("saas/user: user already exists")
	ErrMemberNotFound = errors.New("saas/user: member not found")
	ErrMemberExists   = errors.New("saas/user: member already exists")
	ErrTenantMismatch = errors.New("saas/user: tenant mismatch")

	ErrInvalidListFilter = errors.New("saas/user: invalid list filter")

	ErrNilDB = errors.New("saas/user: nil db")

	ErrInvalidTableName = errors.New("saas/user: invalid table name")

	ErrUnsupportedSQLDialect = errors.New("saas/user: unsupported sql dialect")
)
