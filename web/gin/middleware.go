package ginsaas

import (
	"context"
	"errors"
	"net/http"

	tenantctx "github.com/DarkInno/saas/core/context"
	"github.com/DarkInno/saas/core/resolver"
	"github.com/DarkInno/saas/core/store"
	"github.com/DarkInno/saas/core/types"

	"github.com/gin-gonic/gin"
)

// TenantMiddleware resolves the current tenant and stores it in request context.
func TenantMiddleware(resolver resolver.Resolver, store store.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, err := resolver.Resolve(c.Request)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorBody("tenant_required"))
			return
		}

		tenant, err := store.Get(c.Request.Context(), tenantID)
		if err != nil {
			status := http.StatusForbidden
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				status = http.StatusRequestTimeout
			}
			c.AbortWithStatusJSON(status, errorBody("tenant_forbidden"))
			return
		}
		if tenant.Status != types.TenantStatusActive {
			c.AbortWithStatusJSON(http.StatusForbidden, errorBody("tenant_inactive"))
			return
		}

		c.Request = c.Request.WithContext(tenantctx.WithTenant(c.Request.Context(), tenant))
		c.Next()
	}
}

// TenantStatusGuard allows only active tenants.
func TenantStatusGuard() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenant, ok := tenantctx.FromContext(c.Request.Context())
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, errorBody("tenant_required"))
			return
		}
		if tenant.Status != types.TenantStatusActive {
			c.AbortWithStatusJSON(http.StatusForbidden, errorBody("tenant_inactive"))
			return
		}
		c.Next()
	}
}

// HostGuardMiddleware allows only explicit host context.
func HostGuardMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !tenantctx.IsHost(c.Request.Context()) {
			c.AbortWithStatusJSON(http.StatusForbidden, errorBody("host_required"))
			return
		}
		c.Next()
	}
}

func errorBody(code string) gin.H {
	return gin.H{"error": code}
}
