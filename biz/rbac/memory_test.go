package rbac

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
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

func TestNewSQLStoreValidationAndCodec(t *testing.T) {
	if _, err := NewSQLStore(nil); !errors.Is(err, ErrNilDB) {
		t.Fatalf("NewSQLStore(nil) error = %v, want ErrNilDB", err)
	}

	db := &sql.DB{}
	store, err := NewSQLStore(db)
	if err != nil {
		t.Fatalf("NewSQLStore() error = %v", err)
	}
	if store.table != DefaultSQLTableName {
		t.Fatalf("default table = %q, want %q", store.table, DefaultSQLTableName)
	}

	store, err = NewSQLStore(db, WithTableName("public.rbac_roles"), WithSQLDialect(SQLDialectPostgres))
	if err != nil {
		t.Fatalf("NewSQLStore(custom) error = %v", err)
	}
	if store.table != "public.rbac_roles" || store.dialect != SQLDialectPostgres {
		t.Fatalf("SQLStore = %+v, want custom table and postgres dialect", store)
	}
	if got := store.placeholders(3, 2); got != "$2, $3, $4" {
		t.Fatalf("postgres placeholders = %q, want $2, $3, $4", got)
	}

	if _, err := NewSQLStore(db, WithTableName("rbac_roles;drop")); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("NewSQLStore(unsafe table) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := NewSQLStore(db, WithSQLDialect("oracle")); !errors.Is(err, ErrUnsupportedSQLDialect) {
		t.Fatalf("NewSQLStore(unsupported dialect) error = %v, want ErrUnsupportedSQLDialect", err)
	}

	raw, err := marshalPermissions([]Permission{"orders.read", "orders.write"})
	if err != nil {
		t.Fatalf("marshalPermissions() error = %v", err)
	}
	decoded, err := unmarshalPermissions(raw)
	if err != nil {
		t.Fatalf("unmarshalPermissions() error = %v", err)
	}
	want := []Permission{"orders.read", "orders.write"}
	if !reflect.DeepEqual(decoded, want) {
		t.Fatalf("unmarshalPermissions() = %#v, want %#v", decoded, want)
	}

	role, err := scanRole(roleScannerFunc(func(dest ...any) error {
		*(dest[0].(*string)) = "tenant-a"
		*(dest[1].(*string)) = "admin"
		*(dest[2].(*string)) = raw
		return nil
	}))
	if err != nil {
		t.Fatalf("scanRole() error = %v", err)
	}
	if role.TenantID != "tenant-a" || role.Key != "admin" || !reflect.DeepEqual(role.Permissions, want) {
		t.Fatalf("scanRole() = %+v, want decoded role", role)
	}
}

type roleScannerFunc func(dest ...any) error

func (fn roleScannerFunc) Scan(dest ...any) error {
	return fn(dest...)
}
