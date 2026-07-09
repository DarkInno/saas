package oidc

import "errors"

var (
	ErrInvalidConfig         = errors.New("gotenancy/identity/oidc: invalid config")
	ErrInvalidCallback       = errors.New("gotenancy/identity/oidc: invalid callback")
	ErrStateMismatch         = errors.New("gotenancy/identity/oidc: state mismatch")
	ErrNonceMismatch         = errors.New("gotenancy/identity/oidc: nonce mismatch")
	ErrTokenMissing          = errors.New("gotenancy/identity/oidc: token missing")
	ErrSubjectMismatch       = errors.New("gotenancy/identity/oidc: subject mismatch")
	ErrEmailMissing          = errors.New("gotenancy/identity/oidc: email missing")
	ErrProviderRejected      = errors.New("gotenancy/identity/oidc: provider rejected callback")
	ErrInsecureURL           = errors.New("gotenancy/identity/oidc: insecure url")
	ErrDuplicateParam        = errors.New("gotenancy/identity/oidc: duplicate callback parameter")
	ErrLoginNotFound         = errors.New("gotenancy/identity/oidc: login not found")
	ErrLoginExpired          = errors.New("gotenancy/identity/oidc: login expired")
	ErrLoginStoreFull        = errors.New("gotenancy/identity/oidc: login store full")
	ErrNilDB                 = errors.New("gotenancy/identity/oidc: nil db")
	ErrInvalidTableName      = errors.New("gotenancy/identity/oidc: invalid table name")
	ErrUnsupportedSQLDialect = errors.New("gotenancy/identity/oidc: unsupported sql dialect")
)
