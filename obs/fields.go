package obs

import (
	"context"

	tenantctx "github.com/DarkInno/saas/core/context"
)

const (
	// TenantIDField is the standard observability field for tenant ID.
	TenantIDField = "tenant_id"

	// TenantSideField is the standard observability field for tenant side.
	TenantSideField = "tenant_side"

	hostSide   = "host"
	tenantSide = "tenant"
)

// Fields returns tenant observability fields for ctx.
func Fields(ctx context.Context) map[string]string {
	tenantID, side := fieldValues(ctx)
	if tenantID != "" {
		return map[string]string{
			TenantIDField:   tenantID,
			TenantSideField: side,
		}
	}
	if side != "" {
		return map[string]string{TenantSideField: side}
	}
	return map[string]string{}
}

func fieldValues(ctx context.Context) (tenantID string, side string) {
	if tenant, ok := tenantctx.FromContext(ctx); ok {
		return tenant.ID.String(), tenantSide
	}
	if tenantctx.IsHost(ctx) {
		return "", hostSide
	}
	return "", ""
}
