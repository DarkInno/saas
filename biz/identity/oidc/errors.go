package oidc

import "errors"

var (
	ErrInvalidConfig         = errors.New("saas/identity/oidc: invalid config")
	ErrInvalidCallback       = errors.New("saas/identity/oidc: invalid callback")
	ErrStateMismatch         = errors.New("saas/identity/oidc: state mismatch")
	ErrNonceMismatch         = errors.New("saas/identity/oidc: nonce mismatch")
	ErrTokenMissing          = errors.New("saas/identity/oidc: token missing")
	ErrSubjectMismatch       = errors.New("saas/identity/oidc: subject mismatch")
	ErrEmailMissing          = errors.New("saas/identity/oidc: email missing")
	ErrProviderRejected      = errors.New("saas/identity/oidc: provider rejected callback")
	ErrInsecureURL           = errors.New("saas/identity/oidc: insecure url")
	ErrDuplicateParam        = errors.New("saas/identity/oidc: duplicate callback parameter")
	ErrLoginNotFound         = errors.New("saas/identity/oidc: login not found")
	ErrLoginExpired          = errors.New("saas/identity/oidc: login expired")
	ErrLoginStoreFull        = errors.New("saas/identity/oidc: login store full")
	ErrNilDB                 = errors.New("saas/identity/oidc: nil db")
	ErrInvalidTableName      = errors.New("saas/identity/oidc: invalid table name")
	ErrUnsupportedSQLDialect = errors.New("saas/identity/oidc: unsupported sql dialect")
)
