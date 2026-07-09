package echogotenancy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	tenantctx "github.com/DarkInno/gotenancy/core/context"
	"github.com/DarkInno/gotenancy/core/resolver"
	"github.com/DarkInno/gotenancy/core/store"
	"github.com/DarkInno/gotenancy/core/types"

	"github.com/labstack/echo/v4"
)

func TestTenantMiddlewareRejectsInactiveTenantByDefault(t *testing.T) {
	backing := store.NewMemoryStore()
	if err := backing.Create(context.Background(), types.Tenant{ID: "tenant-a", Status: types.TenantStatusActive}); err != nil {
		t.Fatalf("Create(active) error = %v", err)
	}
	if err := backing.Create(context.Background(), types.Tenant{ID: "tenant-b", Status: types.TenantStatusSuspended}); err != nil {
		t.Fatalf("Create(suspended) error = %v", err)
	}

	router := echo.New()
	router.Use(TenantMiddleware(resolver.NewComposite(resolver.NewHeaderContrib("", types.TenantIDStrategyString)), backing))
	router.GET("/ok", func(c echo.Context) error {
		tenant, ok := tenantctx.FromContext(c.Request().Context())
		if !ok {
			t.Fatal("tenant missing from request context")
		}
		return c.String(http.StatusOK, tenant.ID.String())
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/ok", nil)
	request.Header.Set(resolver.DefaultHeaderName, "tenant-a")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "tenant-a" {
		t.Fatalf("active response = %d %q, want 200 tenant-a", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/ok", nil)
	request.Header.Set(resolver.DefaultHeaderName, "tenant-b")
	router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("suspended status = %d, want 403", recorder.Code)
	}
}

func TestTenantMiddlewareRejectsMissingTenant(t *testing.T) {
	router := echo.New()
	router.Use(TenantMiddleware(resolver.NewComposite(resolver.NewHeaderContrib("", types.TenantIDStrategyString)), store.NewMemoryStore()))
	router.GET("/ok", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/ok", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing tenant status = %d, want 401", recorder.Code)
	}
}

func TestTenantStatusGuard(t *testing.T) {
	router := echo.New()
	router.GET("/missing", func(c echo.Context) error {
		return c.String(http.StatusOK, "unexpected")
	}, TenantStatusGuard())
	router.GET("/inactive", func(c echo.Context) error {
		return c.String(http.StatusOK, "unexpected")
	}, injectEchoTenant(types.TenantStatusSuspended), TenantStatusGuard())
	router.GET("/active", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, injectEchoTenant(types.TenantStatusActive), TenantStatusGuard())

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing tenant status = %d, want 401", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/inactive", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("inactive tenant status = %d, want 403", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/active", nil))
	if recorder.Code != http.StatusOK || recorder.Body.String() != "ok" {
		t.Fatalf("active tenant response = %d %q, want 200 ok", recorder.Code, recorder.Body.String())
	}
}

func TestHostGuardMiddleware(t *testing.T) {
	router := echo.New()
	router.GET("/host", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, HostGuardMiddleware())
	router.GET("/host-ok", func(c echo.Context) error {
		return c.NoContent(http.StatusOK)
	}, func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := c.Request()
			c.SetRequest(request.WithContext(tenantctx.WithHost(request.Context())))
			return next(c)
		}
	}, HostGuardMiddleware())

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/host", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("host guard without host status = %d, want 403", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/host-ok", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("host guard with host status = %d, want 200", recorder.Code)
	}
}

func injectEchoTenant(status types.TenantStatus) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := c.Request()
			c.SetRequest(request.WithContext(tenantctx.WithTenant(request.Context(), types.Tenant{
				ID:     "tenant-a",
				Status: status,
			})))
			return next(c)
		}
	}
}
