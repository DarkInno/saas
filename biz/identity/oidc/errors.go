package oidc

import "errors"

var (
	ErrInvalidConfig    = errors.New("gotenancy/identity/oidc: invalid config")
	ErrInvalidCallback  = errors.New("gotenancy/identity/oidc: invalid callback")
	ErrStateMismatch    = errors.New("gotenancy/identity/oidc: state mismatch")
	ErrNonceMismatch    = errors.New("gotenancy/identity/oidc: nonce mismatch")
	ErrTokenMissing     = errors.New("gotenancy/identity/oidc: token missing")
	ErrSubjectMismatch  = errors.New("gotenancy/identity/oidc: subject mismatch")
	ErrEmailMissing     = errors.New("gotenancy/identity/oidc: email missing")
	ErrProviderRejected = errors.New("gotenancy/identity/oidc: provider rejected callback")
)
