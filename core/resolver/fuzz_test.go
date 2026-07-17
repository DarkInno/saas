package resolver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/DarkInno/gotenancy/core/types"
)

func FuzzCompositeResolverPriority(f *testing.F) {
	for _, seed := range []struct {
		query  string
		header string
	}{
		{query: "query-tenant", header: "header-tenant"},
		{query: "", header: "header-tenant"},
		{query: "", header: ""},
	} {
		f.Add(seed.query, seed.header)
	}

	resolver := NewComposite(
		NewQueryContrib("", types.TenantIDStrategyString),
		NewHeaderContrib("", types.TenantIDStrategyString),
	)
	f.Fuzz(func(t *testing.T, queryValue, headerValue string) {
		queryNormalized := strings.TrimSpace(queryValue)
		headerNormalized := strings.TrimSpace(headerValue)
		requestURL := "http://example.com"
		if queryNormalized != "" {
			requestURL += "?tenant_id=" + url.QueryEscape(queryValue)
		}
		req := httptest.NewRequest(http.MethodGet, requestURL, nil)
		if headerNormalized != "" {
			req.Header.Set(DefaultHeaderName, headerValue)
		}

		id, err := resolver.Resolve(req)
		switch {
		case queryNormalized != "":
			if err != nil || id != types.TenantID(queryNormalized) {
				t.Fatalf("Resolve(query=%q, header=%q) = %q, %v; want query tenant", queryValue, headerValue, id, err)
			}
		case headerNormalized != "":
			if err != nil || id != types.TenantID(headerNormalized) {
				t.Fatalf("Resolve(query=%q, header=%q) = %q, %v; want header tenant", queryValue, headerValue, id, err)
			}
		default:
			if !errors.Is(err, ErrNoTenant) {
				t.Fatalf("Resolve(query=%q, header=%q) error = %v, want ErrNoTenant", queryValue, headerValue, err)
			}
		}
	})
}
