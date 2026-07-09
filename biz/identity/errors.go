package identity

import "errors"

var (
	ErrInvalidIdentity    = errors.New("gotenancy/identity: invalid identity")
	ErrIdentityNotFound   = errors.New("gotenancy/identity: identity not found")
	ErrIdentityConflict   = errors.New("gotenancy/identity: identity conflict")
	ErrProviderNotAllowed = errors.New("gotenancy/identity: provider not allowed")
	ErrUnverifiedEmail    = errors.New("gotenancy/identity: email is not verified")

	ErrNilDB = errors.New("gotenancy/identity: nil db")

	ErrInvalidTableName = errors.New("gotenancy/identity: invalid table name")

	ErrUnsupportedSQLDialect = errors.New("gotenancy/identity: unsupported sql dialect")
)
