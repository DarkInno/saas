package rbac

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryEnforcerRoleDeletionIsTenantScopedAndCancellationSafe(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryEnforcer()
	for _, role := range []Role{
		{TenantID: "tenant-a", Key: "reader", Permissions: []Permission{"orders.read"}},
		{TenantID: "tenant-b", Key: "reader", Permissions: []Permission{"orders.write"}},
	} {
		if err := service.CreateRole(ctx, role); err != nil {
			t.Fatalf("CreateRole(%s) error = %v", role.TenantID, err)
		}
	}
	if err := service.DeleteRole(ctx, "tenant-a", "reader"); err != nil {
		t.Fatalf("DeleteRole(tenant-a) error = %v", err)
	}
	if err := service.Enforce(ctx, "tenant-a", []string{"reader"}, "orders.read"); !errors.Is(err, ErrPermissionDeny) {
		t.Fatalf("Enforce(deleted role) error = %v, want ErrPermissionDeny", err)
	}
	if err := service.Enforce(ctx, "tenant-b", []string{"reader"}, "orders.write"); err != nil {
		t.Fatalf("Enforce(tenant-b) error = %v, want unaffected authorization", err)
	}
	if err := service.DeleteRole(ctx, "tenant-a", "reader"); !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("DeleteRole(missing) error = %v, want ErrRoleNotFound", err)
	}
	if err := service.Enforce(ctx, "", []string{"reader"}, "orders.write"); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("Enforce(blank tenant) error = %v, want ErrInvalidRole", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := service.DeleteRole(canceled, "tenant-b", "reader"); !errors.Is(err, context.Canceled) {
		t.Fatalf("DeleteRole(canceled) error = %v, want context.Canceled", err)
	}
	if _, err := service.GetRole(ctx, "tenant-b", "reader"); err != nil {
		t.Fatalf("GetRole(after canceled delete) error = %v, want retained role", err)
	}
}
