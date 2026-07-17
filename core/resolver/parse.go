package resolver

import "github.com/DarkInno/saas/core/types"

func parseTenantID(raw string, strategy types.TenantIDStrategy) (types.TenantID, error) {
	return types.ParseTenantID(raw, strategy)
}
