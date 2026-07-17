package identity

import "errors"

var (
	ErrInvalidIdentity    = errors.New("saas/identity: invalid identity")
	ErrIdentityNotFound   = errors.New("saas/identity: identity not found")
	ErrIdentityConflict   = errors.New("saas/identity: identity conflict")
	ErrProviderNotAllowed = errors.New("saas/identity: provider not allowed")
	ErrUnverifiedEmail    = errors.New("saas/identity: email is not verified")

	ErrNilDB = errors.New("saas/identity: nil db")

	ErrInvalidTableName = errors.New("saas/identity: invalid table name")

	ErrUnsupportedSQLDialect = errors.New("saas/identity: unsupported sql dialect")
)
