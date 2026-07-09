package rbac

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryServiceRoleAndAuthorize(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()
	role := Role{TenantID: "tenant-a", Key: "admin", Permissions: []Permission{"orders.read", "orders.write"}}

	if err := service.CreateRole(ctx, role); err != nil {
		t.Fatalf("CreateRole() error = %v", err)
	}
	role.Permissions[0] = "mutated"

	got, err := service.GetRole(ctx, "tenant-a", "admin")
	if err != nil {
		t.Fatalf("GetRole() error = %v", err)
	}
	if got.Permissions[0] != "orders.read" {
		t.Fatalf("GetRole permissions = %#v, want copy", got.Permissions)
	}
	if !got.HasPermission("orders.read") || got.HasPermission("billing.write") || got.HasPermission("") {
		t.Fatalf("HasPermission() result for %#v is not expected", got.Permissions)
	}
	if err := service.Authorize(ctx, "tenant-a", []string{"admin"}, "orders.write"); err != nil {
		t.Fatalf("Authorize(allowed) error = %v", err)
	}
	if err := service.Enforce(ctx, "tenant-a", []string{"admin"}, "orders.read"); err != nil {
		t.Fatalf("Enforce(allowed) error = %v", err)
	}
	if err := service.Authorize(ctx, "tenant-a", []string{"admin"}, "billing.write"); !errors.Is(err, ErrPermissionDeny) {
		t.Fatalf("Authorize(denied) error = %v, want ErrPermissionDeny", err)
	}
	if err := service.Authorize(ctx, "tenant-b", []string{"admin"}, "orders.write"); !errors.Is(err, ErrPermissionDeny) {
		t.Fatalf("Authorize(other tenant) error = %v, want ErrPermissionDeny", err)
	}
}

func TestMemoryServiceValidation(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()
	if err := service.CreateRole(ctx, Role{}); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("CreateRole(invalid) error = %v, want ErrInvalidRole", err)
	}
	if err := service.CreateRole(ctx, Role{TenantID: "tenant-a", Key: "admin", Permissions: []Permission{""}}); !errors.Is(err, ErrInvalidRole) {
		t.Fatalf("CreateRole(empty permission) error = %v, want ErrInvalidRole", err)
	}
	if _, err := service.GetRole(ctx, "tenant-a", "missing"); !errors.Is(err, ErrRoleNotFound) {
		t.Fatalf("GetRole(missing) error = %v, want ErrRoleNotFound", err)
	}
}
