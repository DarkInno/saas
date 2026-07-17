package httpsaas

import (
	"context"
	"errors"
	"net/http"

	tenantctx "github.com/DarkInno/saas/core/context"
	"github.com/DarkInno/saas/core/resolver"
	"github.com/DarkInno/saas/core/store"
	"github.com/DarkInno/saas/core/types"
)

// TenantMiddleware resolves the current tenant and stores it in request context.
func TenantMiddleware(resolver resolver.Resolver, store store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID, err := resolver.Resolve(r)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "tenant_required")
				return
			}

			tenant, err := store.Get(r.Context(), tenantID)
			if err != nil {
				status := http.StatusForbidden
				if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
					status = http.StatusRequestTimeout
				}
				writeError(w, status, "tenant_forbidden")
				return
			}
			if tenant.Status != types.TenantStatusActive {
				writeError(w, http.StatusForbidden, "tenant_inactive")
				return
			}

			next.ServeHTTP(w, r.WithContext(tenantctx.WithTenant(r.Context(), tenant)))
		})
	}
}

// TenantStatusGuard allows only active tenants.
func TenantStatusGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant, ok := tenantctx.FromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "tenant_required")
			return
		}
		if tenant.Status != types.TenantStatusActive {
			writeError(w, http.StatusForbidden, "tenant_inactive")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// HostGuard allows only host context.
func HostGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !tenantctx.IsHost(r.Context()) {
			writeError(w, http.StatusForbidden, "host_required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(`{"error":"` + code + `"}`))
}
