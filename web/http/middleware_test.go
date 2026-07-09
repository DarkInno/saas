package httpgotenancy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	tenantctx "github.com/DarkInno/gotenancy/core/context"
	"github.com/DarkInno/gotenancy/core/resolver"
	"github.com/DarkInno/gotenancy/core/store"
	"github.com/DarkInno/gotenancy/core/types"
)

func TestTenantMiddlewareRejectsInactiveTenantByDefault(t *testing.T) {
	backing := store.NewMemoryStore()
	if err := backing.Create(context.Background(), types.Tenant{ID: "tenant-a", Status: types.TenantStatusActive}); err != nil {
		t.Fatalf("Create(active) error = %v", err)
	}
	if err := backing.Create(context.Background(), types.Tenant{ID: "tenant-b", Status: types.TenantStatusSuspended}); err != nil {
		t.Fatalf("Create(suspended) error = %v", err)
	}

	handler := TenantMiddleware(resolver.NewComposite(resolver.NewHeaderContrib("", types.TenantIDStrategyString)), backing)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant, ok := tenantctx.FromContext(r.Context())
			if !ok {
				t.Fatal("tenant missing from context")
			}
			_, _ = w.Write([]byte(tenant.ID.String()))
		}),
	)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(resolver.DefaultHeaderName, "tenant-a")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "tenant-a" {
		t.Fatalf("active response = %d %q, want 200 tenant-a", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(resolver.DefaultHeaderName, "tenant-b")
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("suspended status = %d, want 403", recorder.Code)
	}
}

func TestTenantMiddlewareRejectsMissingTenant(t *testing.T) {
	handler := TenantMiddleware(resolver.NewComposite(resolver.NewHeaderContrib("", types.TenantIDStrategyString)), store.NewMemoryStore())(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("missing tenant status = %d, want 401", recorder.Code)
	}
}

func TestTenantStatusGuard(t *testing.T) {
	handler := TenantStatusGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusUnauthorized || recorder.Body.String() != `{"error":"tenant_required"}` {
		t.Fatalf("missing tenant response = %d %q, want 401 tenant_required", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(tenantctx.WithTenant(request.Context(), types.Tenant{
		ID:     "tenant-a",
		Status: types.TenantStatusSuspended,
	}))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || recorder.Body.String() != `{"error":"tenant_inactive"}` {
		t.Fatalf("inactive tenant response = %d %q, want 403 tenant_inactive", recorder.Code, recorder.Body.String())
	}

	recorder = httptest.NewRecorder()
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(tenantctx.WithTenant(request.Context(), types.Tenant{
		ID:     "tenant-a",
		Status: types.TenantStatusActive,
	}))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK || recorder.Body.String() != "ok" {
		t.Fatalf("active tenant response = %d %q, want 200 ok", recorder.Code, recorder.Body.String())
	}
}

func TestHostGuard(t *testing.T) {
	handler := HostGuard(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("without host status = %d, want 403", recorder.Code)
	}

	recorder = httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request = request.WithContext(tenantctx.WithHost(request.Context()))
	handler.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("with host status = %d, want 200", recorder.Code)
	}
}
