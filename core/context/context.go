package tenantctx

import (
	"context"

	"github.com/DarkInno/saas/core/types"
)

type state struct {
	side   side
	tenant types.Tenant
}

// WithTenant returns a child context scoped to tenant.
func WithTenant(ctx context.Context, tenant types.Tenant) context.Context {
	return context.WithValue(ctx, stateKey{}, state{
		side:   sideTenant,
		tenant: cloneTenant(tenant),
	})
}

// FromContext returns the tenant stored in ctx.
func FromContext(ctx context.Context) (types.Tenant, bool) {
	current, ok := ctx.Value(stateKey{}).(state)
	if !ok || current.side != sideTenant {
		return types.Tenant{}, false
	}
	return cloneTenant(current.tenant), true
}

// WithHost returns a child context explicitly marked as host-side.
func WithHost(ctx context.Context) context.Context {
	return context.WithValue(ctx, stateKey{}, state{side: sideHost})
}

// IsHost reports whether ctx is explicitly marked as host-side.
func IsHost(ctx context.Context) bool {
	current, ok := ctx.Value(stateKey{}).(state)
	return ok && current.side == sideHost
}

func cloneTenant(tenant types.Tenant) types.Tenant {
	if tenant.Config == nil {
		return tenant
	}

	cloned := make(map[string]string, len(tenant.Config))
	for key, value := range tenant.Config {
		cloned[key] = value
	}
	tenant.Config = cloned
	return tenant
}
