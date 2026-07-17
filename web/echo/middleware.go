package echosaas

import (
	"context"
	"errors"
	"net/http"

	tenantctx "github.com/DarkInno/saas/core/context"
	"github.com/DarkInno/saas/core/resolver"
	"github.com/DarkInno/saas/core/store"
	"github.com/DarkInno/saas/core/types"

	"github.com/labstack/echo/v4"
)

// TenantMiddleware resolves the current tenant and stores it in request context.
func TenantMiddleware(resolver resolver.Resolver, store store.Store) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tenantID, err := resolver.Resolve(c.Request())
			if err != nil {
				return c.JSON(http.StatusUnauthorized, errorBody("tenant_required"))
			}

			tenant, err := store.Get(c.Request().Context(), tenantID)
			if err != nil {
				status := http.StatusForbidden
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					status = http.StatusRequestTimeout
				}
				return c.JSON(status, errorBody("tenant_forbidden"))
			}
			if tenant.Status != types.TenantStatusActive {
				return c.JSON(http.StatusForbidden, errorBody("tenant_inactive"))
			}

			request := c.Request()
			c.SetRequest(request.WithContext(tenantctx.WithTenant(request.Context(), tenant)))
			return next(c)
		}
	}
}

// TenantStatusGuard allows only active tenants.
func TenantStatusGuard() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tenant, ok := tenantctx.FromContext(c.Request().Context())
			if !ok {
				return c.JSON(http.StatusUnauthorized, errorBody("tenant_required"))
			}
			if tenant.Status != types.TenantStatusActive {
				return c.JSON(http.StatusForbidden, errorBody("tenant_inactive"))
			}
			return next(c)
		}
	}
}

// HostGuardMiddleware allows only explicit host context.
func HostGuardMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !tenantctx.IsHost(c.Request().Context()) {
				return c.JSON(http.StatusForbidden, errorBody("host_required"))
			}
			return next(c)
		}
	}
}

func errorBody(code string) map[string]string {
	return map[string]string{"error": code}
}
