package tenantctx

import (
	"context"
	"testing"

	"github.com/DarkInno/saas/core/types"
)

func TestWithTenantAndFromContext(t *testing.T) {
	tenant := types.Tenant{
		ID:     "tenant-a",
		Name:   "Tenant A",
		Status: types.TenantStatusActive,
		PlanID: "starter",
		Config: map[string]string{"region": "us"},
	}

	ctx := WithTenant(context.Background(), tenant)

	tenant.Config["region"] = "eu"

	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext() ok = false, want true")
	}
	if got.ID != tenant.ID || got.Name != tenant.Name || got.Status != tenant.Status || got.PlanID != tenant.PlanID {
		t.Fatalf("FromContext() = %+v, want tenant metadata", got)
	}
	if got.Config["region"] != "us" {
		t.Fatalf("stored tenant config was mutated, got region %q", got.Config["region"])
	}

	got.Config["region"] = "ap"
	again, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext() after mutation ok = false, want true")
	}
	if again.Config["region"] != "us" {
		t.Fatalf("returned tenant config mutated stored state, got region %q", again.Config["region"])
	}
}

func TestHostContext(t *testing.T) {
	tenant := types.Tenant{ID: "tenant-a"}

	hostCtx := WithHost(context.Background())
	if !IsHost(hostCtx) {
		t.Fatal("IsHost() = false, want true")
	}
	if _, ok := FromContext(hostCtx); ok {
		t.Fatal("FromContext() ok = true for host context, want false")
	}

	tenantCtx := WithTenant(hostCtx, tenant)
	if IsHost(tenantCtx) {
		t.Fatal("IsHost() = true after WithTenant(), want false")
	}
	if got, ok := FromContext(tenantCtx); !ok || got.ID != tenant.ID {
		t.Fatalf("FromContext() = %+v, %v; want tenant", got, ok)
	}

	hostAgain := WithHost(tenantCtx)
	if !IsHost(hostAgain) {
		t.Fatal("IsHost() = false after WithHost(), want true")
	}
	if _, ok := FromContext(hostAgain); ok {
		t.Fatal("FromContext() ok = true after WithHost(), want false")
	}
}

func TestDetachRemovesTenantAndCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	ctx := WithTenant(parent, types.Tenant{ID: "tenant-a"})
	cancel()

	detached := Detach(ctx)
	if _, ok := FromContext(detached); ok {
		t.Fatal("FromContext() ok = true after Detach(), want false")
	}
	if IsHost(detached) {
		t.Fatal("IsHost() = true after Detach(), want false")
	}
	if err := detached.Err(); err != nil {
		t.Fatalf("detached.Err() = %v, want nil", err)
	}
}

func TestSwitchReturnsNewTenantContext(t *testing.T) {
	original := WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})
	switched := Switch(original, types.Tenant{ID: "tenant-b"})

	oldTenant, ok := FromContext(original)
	if !ok {
		t.Fatal("FromContext(original) ok = false, want true")
	}
	if oldTenant.ID != "tenant-a" {
		t.Fatalf("original tenant = %q, want tenant-a", oldTenant.ID)
	}

	newTenant, ok := FromContext(switched)
	if !ok {
		t.Fatal("FromContext(switched) ok = false, want true")
	}
	if newTenant.ID != "tenant-b" {
		t.Fatalf("switched tenant = %q, want tenant-b", newTenant.ID)
	}
}

func TestFromContextWithoutTenant(t *testing.T) {
	if tenant, ok := FromContext(context.Background()); ok {
		t.Fatalf("FromContext() = %+v, true; want zero tenant, false", tenant)
	}
	if IsHost(context.Background()) {
		t.Fatal("IsHost() = true for background context, want false")
	}
}
