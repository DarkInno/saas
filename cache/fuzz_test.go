package cache

import (
	"context"
	"errors"
	"strings"
	"testing"

	tenantctx "github.com/DarkInno/saas/core/context"
	"github.com/DarkInno/saas/core/types"
)

func FuzzRedisURLAndKeyBuilder(f *testing.F) {
	for _, seed := range []struct {
		rawURL string
		key    string
	}{
		{rawURL: "redis://localhost:6379/0", key: "profile"},
		{rawURL: "redis://cache-user:super-secret@localhost/%zz", key: "t2:unsafe"},
	} {
		f.Add(seed.rawURL, seed.key)
	}

	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-fuzz"})
	builder := KeyBuilder{}
	f.Fuzz(func(t *testing.T, rawURL, key string) {
		redisCache, err := NewRedisFromURL(rawURL)
		if err != nil {
			if !errors.Is(err, ErrInvalidRedisConfig) {
				t.Fatalf("NewRedisFromURL(%q) error = %v, want ErrInvalidRedisConfig", rawURL, err)
			}
			if strings.Contains(rawURL, "super-secret") && strings.Contains(err.Error(), "super-secret") {
				t.Fatalf("NewRedisFromURL(%q) error leaked password: %q", rawURL, err)
			}
		} else if closeErr := redisCache.Close(); closeErr != nil {
			t.Fatalf("NewRedisFromURL(%q) Close() error = %v", rawURL, closeErr)
		}

		built, err := builder.Build(ctx, key)
		unsafe := key == "" || strings.HasPrefix(key, tenantPrefix) || strings.HasPrefix(key, legacyTenantPrefix) || strings.HasPrefix(key, globalPrefix)
		if unsafe {
			if !errors.Is(err, ErrUnsafeKey) {
				t.Fatalf("Build(%q) error = %v, want ErrUnsafeKey", key, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("Build(%q) error = %v", key, err)
		}
		if !strings.HasPrefix(built, tenantPrefix) {
			t.Fatalf("Build(%q) = %q, want tenant prefix %q", key, built, tenantPrefix)
		}
	})
}
