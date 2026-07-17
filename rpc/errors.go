package rpc

import "errors"

var (
	ErrNoTenantMetadata = errors.New("saas/rpc: no tenant metadata")
	ErrInvalidCarrier   = errors.New("saas/rpc: invalid carrier")
)
