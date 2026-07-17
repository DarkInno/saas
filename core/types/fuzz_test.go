package types

import (
	"strings"
	"testing"
)

func FuzzParseTenantID(f *testing.F) {
	for _, seed := range []struct {
		raw      string
		strategy string
	}{
		{raw: " tenant-fuzz ", strategy: string(TenantIDStrategyString)},
		{raw: "42", strategy: string(TenantIDStrategyInt)},
		{raw: "A0EEBC99-9C0B-4EF8-BB6D-6BB9BD380A11", strategy: string(TenantIDStrategyUUID)},
	} {
		f.Add(seed.raw, seed.strategy)
	}

	f.Fuzz(func(t *testing.T, raw, strategy string) {
		id, err := ParseTenantID(raw, TenantIDStrategy(strategy))
		if err != nil {
			return
		}

		if id == "" || id.String() != strings.TrimSpace(id.String()) {
			t.Fatalf("ParseTenantID(%q, %q) = %q, want a nonempty normalized identifier", raw, strategy, id)
		}
	})
}
