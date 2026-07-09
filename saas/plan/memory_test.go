package plan

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestMemoryServiceCRUD(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()
	plan := testPlan("starter")

	if err := service.Create(ctx, plan); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := service.Create(ctx, plan); !errors.Is(err, ErrPlanAlreadyExists) {
		t.Fatalf("Create(duplicate) error = %v, want ErrPlanAlreadyExists", err)
	}

	got, err := service.Get(ctx, plan.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.ID != plan.ID || got.Name != plan.Name {
		t.Fatalf("Get() = %+v, want plan", got)
	}

	plan.Name = "Starter Updated"
	plan.Quotas[0].Limit = 200
	if err := service.Update(ctx, plan); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	got, err = service.Get(ctx, plan.ID)
	if err != nil {
		t.Fatalf("Get() after update error = %v", err)
	}
	if got.Name != "Starter Updated" || got.Quotas[0].Limit != 200 {
		t.Fatalf("Get() after update = %+v, want updated", got)
	}

	if err := service.Delete(ctx, plan.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.Get(ctx, plan.ID); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("Get(deleted) error = %v, want ErrPlanNotFound", err)
	}
}

func TestMemoryServiceCopiesPlan(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()
	plan := testPlan("starter")

	if err := service.Create(ctx, plan); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	plan.Features[0].Config["seats"] = "999"

	got, err := service.Get(ctx, "starter")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.Features[0].Config["seats"] != "5" {
		t.Fatalf("stored feature config = %q, want 5", got.Features[0].Config["seats"])
	}

	got.Features[0].Config["seats"] = "1"
	again, err := service.Get(ctx, "starter")
	if err != nil {
		t.Fatalf("Get() again error = %v", err)
	}
	if again.Features[0].Config["seats"] != "5" {
		t.Fatalf("returned feature config mutated store, got %q", again.Features[0].Config["seats"])
	}
}

func TestMemoryServiceList(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	for _, id := range []string{"pro", "starter", "enterprise"} {
		if err := store.Create(ctx, testPlan(id)); err != nil {
			t.Fatalf("Create(%q) error = %v", id, err)
		}
	}

	got, err := store.List(ctx, ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 || got[0].ID != "enterprise" || got[1].ID != "pro" {
		t.Fatalf("List() = %+v, want first two sorted plans", got)
	}

	got, err = store.List(ctx, ListFilter{IDs: []string{"starter"}})
	if err != nil {
		t.Fatalf("List(by id) error = %v", err)
	}
	if len(got) != 1 || got[0].ID != "starter" {
		t.Fatalf("List(by id) = %+v, want starter", got)
	}

	if _, err := store.List(ctx, ListFilter{Offset: 1}); !errors.Is(err, ErrInvalidListFilter) {
		t.Fatalf("List(invalid) error = %v, want ErrInvalidListFilter", err)
	}
}

func TestMemoryServiceValidation(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()

	tests := []Plan{
		{},
		{ID: "starter"},
		{ID: "starter", Name: "Starter", Features: []Feature{{}}},
		{ID: "starter", Name: "Starter", Quotas: []Quota{{Resource: "api", Limit: -1, Period: QuotaPeriodDay}}},
		{ID: "starter", Name: "Starter", Quotas: []Quota{{Resource: "api", Limit: 1}}},
	}

	for i, plan := range tests {
		if err := service.Create(ctx, plan); !errors.Is(err, ErrInvalidPlan) {
			t.Fatalf("Create(invalid %d) error = %v, want ErrInvalidPlan", i, err)
		}
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

	store, err = NewSQLStore(db, WithTableName("public.saas_plans"), WithSQLDialect(SQLDialectPostgres))
	if err != nil {
		t.Fatalf("NewSQLStore(custom) error = %v", err)
	}
	if store.table != "public.saas_plans" || store.dialect != SQLDialectPostgres {
		t.Fatalf("SQLStore = %+v, want custom table and postgres dialect", store)
	}
	if got := store.placeholders(3, 2); got != "$2, $3, $4" {
		t.Fatalf("postgres placeholders = %q, want $2, $3, $4", got)
	}

	if _, err := NewSQLStore(db, WithTableName("saas_plans;drop")); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("NewSQLStore(unsafe table) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := NewSQLStore(db, WithSQLDialect("oracle")); !errors.Is(err, ErrUnsupportedSQLDialect) {
		t.Fatalf("NewSQLStore(unsupported dialect) error = %v, want ErrUnsupportedSQLDialect", err)
	}

	features, quotas, err := marshalPlanParts(testPlan("starter"))
	if err != nil {
		t.Fatalf("marshalPlanParts() error = %v", err)
	}
	decodedFeatures, err := unmarshalFeatures(features)
	if err != nil {
		t.Fatalf("unmarshalFeatures() error = %v", err)
	}
	if !reflect.DeepEqual(decodedFeatures, testPlan("starter").Features) {
		t.Fatalf("unmarshalFeatures() = %#v, want plan features", decodedFeatures)
	}
	decodedQuotas, err := unmarshalQuotas(quotas)
	if err != nil {
		t.Fatalf("unmarshalQuotas() error = %v", err)
	}
	if !reflect.DeepEqual(decodedQuotas, testPlan("starter").Quotas) {
		t.Fatalf("unmarshalQuotas() = %#v, want plan quotas", decodedQuotas)
	}
}

func testPlan(id string) Plan {
	return Plan{
		ID:   id,
		Name: "Plan " + id,
		Features: []Feature{{
			Key:     "members",
			Enabled: true,
			Config:  map[string]string{"seats": "5"},
		}},
		Quotas: []Quota{{
			Resource: "api_calls",
			Limit:    100,
			Period:   QuotaPeriodDay,
		}},
	}
}
