package fibersaas

import (
	"context"
	"errors"
	"net/http"

	tenantctx "github.com/DarkInno/saas/core/context"
	"github.com/DarkInno/saas/core/resolver"
	"github.com/DarkInno/saas/core/store"
	"github.com/DarkInno/saas/core/types"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
)

// TenantMiddleware resolves the current tenant and stores it in Fiber user context.
func TenantMiddleware(resolver resolver.Resolver, store store.Store) fiber.Handler {
	return func(c *fiber.Ctx) error {
		request, err := adaptor.ConvertRequest(c, false)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(errorBody("tenant_required"))
		}

		ctx := c.UserContext()
		request = request.WithContext(ctx)
		tenantID, err := resolver.Resolve(request)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(errorBody("tenant_required"))
		}

		tenant, err := store.Get(ctx, tenantID)
		if err != nil {
			status := http.StatusForbidden
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				status = http.StatusRequestTimeout
			}
			return c.Status(status).JSON(errorBody("tenant_forbidden"))
		}
		if tenant.Status != types.TenantStatusActive {
			return c.Status(http.StatusForbidden).JSON(errorBody("tenant_inactive"))
		}

		c.SetUserContext(tenantctx.WithTenant(ctx, tenant))
		return c.Next()
	}
}

// TenantStatusGuard allows only active tenants.
func TenantStatusGuard() fiber.Handler {
	return func(c *fiber.Ctx) error {
		tenant, ok := tenantctx.FromContext(c.UserContext())
		if !ok {
			return c.Status(http.StatusUnauthorized).JSON(errorBody("tenant_required"))
		}
		if tenant.Status != types.TenantStatusActive {
			return c.Status(http.StatusForbidden).JSON(errorBody("tenant_inactive"))
		}
		return c.Next()
	}
}

// HostGuardMiddleware allows only explicit host context.
func HostGuardMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		if !tenantctx.IsHost(c.UserContext()) {
			return c.Status(http.StatusForbidden).JSON(errorBody("host_required"))
		}
		return c.Next()
	}
}

func errorBody(code string) fiber.Map {
	return fiber.Map{"error": code}
}
