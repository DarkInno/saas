package feature

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestMemoryStoreResolveAndList(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	if err := store.SetPlanDefaults(ctx, "starter", []Flag{
		{Key: "members", Enabled: true, Config: map[string]string{"limit": "5"}},
		{Key: "exports", Enabled: false},
	}); err != nil {
		t.Fatalf("SetPlanDefaults() error = %v", err)
	}
	if err := store.SetTenantOverrides(ctx, "tenant-a", []Flag{
		{Key: "exports", Enabled: true, Config: map[string]string{"format": "csv"}},
	}); err != nil {
		t.Fatalf("SetTenantOverrides() error = %v", err)
	}

	flag, err := store.Resolve(ctx, "tenant-a", "starter", "exports")
	if err != nil {
		t.Fatalf("Resolve(exports) error = %v", err)
	}
	if !flag.Enabled || flag.Config["format"] != "csv" {
		t.Fatalf("Resolve(exports) = %+v, want tenant override enabled", flag)
	}

	flag, err = store.Resolve(ctx, "tenant-a", "starter", "members")
	if err != nil {
		t.Fatalf("Resolve(members) error = %v", err)
	}
	if !flag.Enabled || flag.Config["limit"] != "5" {
		t.Fatalf("Resolve(members) = %+v, want plan default", flag)
	}

	flags, err := store.List(ctx, "tenant-a", "starter")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(flags) != 2 || flags[0].Key != "exports" || flags[1].Key != "members" {
		t.Fatalf("List() = %+v, want exports then members", flags)
	}
}

func TestMemoryStoreCopiesFlags(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	flags := []Flag{{Key: "members", Enabled: true, Config: map[string]string{"limit": "5"}}}
	if err := store.SetPlanDefaults(ctx, "starter", flags); err != nil {
		t.Fatalf("SetPlanDefaults() error = %v", err)
	}
	flags[0].Config["limit"] = "999"

	flag, err := store.Resolve(ctx, "tenant-a", "starter", "members")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if flag.Config["limit"] != "5" {
		t.Fatalf("stored config = %q, want 5", flag.Config["limit"])
	}

	flag.Config["limit"] = "1"
	again, err := store.Resolve(ctx, "tenant-a", "starter", "members")
	if err != nil {
		t.Fatalf("Resolve() again error = %v", err)
	}
	if again.Config["limit"] != "5" {
		t.Fatalf("returned config mutated store, got %q", again.Config["limit"])
	}
}

func TestMemoryStoreValidationAndMissing(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	if err := store.SetPlanDefaults(ctx, "", nil); !errors.Is(err, ErrInvalidFeature) {
		t.Fatalf("SetPlanDefaults(empty plan) error = %v, want ErrInvalidFeature", err)
	}
	if err := store.SetPlanDefaults(ctx, "starter", []Flag{{}}); !errors.Is(err, ErrInvalidFeature) {
		t.Fatalf("SetPlanDefaults(empty key) error = %v, want ErrInvalidFeature", err)
	}
	if err := store.SetTenantOverrides(ctx, "", nil); !errors.Is(err, ErrInvalidFeature) {
		t.Fatalf("SetTenantOverrides(empty tenant) error = %v, want ErrInvalidFeature", err)
	}
	if _, err := store.Resolve(ctx, "tenant-a", "starter", "missing"); !errors.Is(err, ErrFeatureNotFound) {
		t.Fatalf("Resolve(missing) error = %v, want ErrFeatureNotFound", err)
	}
}

func TestNewSQLStoreValidation(t *testing.T) {
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
	if got := store.keyColumn(); got != "`key`" {
		t.Fatalf("mysql key column = %q, want quoted key", got)
	}

	store, err = NewSQLStore(db, WithTableName("public.saas_feature_flags"), WithSQLDialect(SQLDialectPostgres))
	if err != nil {
		t.Fatalf("NewSQLStore(custom) error = %v", err)
	}
	if store.table != "public.saas_feature_flags" || store.dialect != SQLDialectPostgres {
		t.Fatalf("SQLStore = %+v, want custom table and postgres dialect", store)
	}
	if got := store.placeholders(2, 4); got != "$4, $5" {
		t.Fatalf("postgres placeholders = %q, want $4, $5", got)
	}
	if got := store.keyColumn(); got != "key" {
		t.Fatalf("postgres key column = %q, want key", got)
	}

	if _, err := NewSQLStore(db, WithTableName("saas_feature_flags;drop")); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("NewSQLStore(unsafe table) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := NewSQLStore(db, WithSQLDialect("oracle")); !errors.Is(err, ErrUnsupportedSQLDialect) {
		t.Fatalf("NewSQLStore(unsupported dialect) error = %v, want ErrUnsupportedSQLDialect", err)
	}
}
