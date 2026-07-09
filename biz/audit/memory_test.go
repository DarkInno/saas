package audit

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestMemoryStoreRecordAndList(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore(WithClock(func() time.Time { return now }))
	event := Event{TenantID: "tenant-a", ActorID: "u1", Action: "orders.create", Resource: "order:1", Metadata: map[string]string{"ip": "127.0.0.1"}}

	if err := store.Record(ctx, event); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	event.Metadata["ip"] = "changed"

	events, err := store.List(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(events) != 1 || events[0].CreatedAt != now || events[0].Metadata["ip"] != "127.0.0.1" {
		t.Fatalf("List() = %+v, want copied event", events)
	}
	if other, err := store.List(ctx, "tenant-b"); err != nil || len(other) != 0 {
		t.Fatalf("List(other) = %+v, %v; want empty nil", other, err)
	}
}

func TestMemoryStoreListPageCursor(t *testing.T) {
	ctx := context.Background()
	base := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	events := []Event{
		{ID: "event-1", TenantID: "tenant-a", Action: "orders.create", Resource: "order:1", CreatedAt: base},
		{ID: "event-2", TenantID: "tenant-a", Action: "orders.update", Resource: "order:1", CreatedAt: base},
		{ID: "event-3", TenantID: "tenant-a", Action: "orders.delete", Resource: "order:1", CreatedAt: base.Add(time.Second)},
	}
	for _, event := range events {
		if err := store.Record(ctx, event); err != nil {
			t.Fatalf("Record(%s) error = %v", event.ID, err)
		}
	}

	first, err := store.ListPage(ctx, "tenant-a", ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("ListPage(first) error = %v", err)
	}
	if len(first) != 2 || first[0].ID != "event-1" || first[1].ID != "event-2" {
		t.Fatalf("ListPage(first) = %+v, want event-1/event-2", first)
	}

	next, err := store.ListPage(ctx, "tenant-a", ListFilter{Cursor: CursorFor(first[len(first)-1]), Limit: 2})
	if err != nil {
		t.Fatalf("ListPage(next) error = %v", err)
	}
	if len(next) != 1 || next[0].ID != "event-3" {
		t.Fatalf("ListPage(next) = %+v, want event-3", next)
	}

	if _, err := store.ListPage(ctx, "tenant-a", ListFilter{Limit: -1}); !errors.Is(err, ErrInvalidListFilter) {
		t.Fatalf("ListPage(invalid) error = %v, want ErrInvalidListFilter", err)
	}
	if _, err := store.ListPage(ctx, "tenant-a", ListFilter{Cursor: Cursor{CreatedAt: base}}); !errors.Is(err, ErrInvalidListFilter) {
		t.Fatalf("ListPage(invalid cursor) error = %v, want ErrInvalidListFilter", err)
	}
}

func TestMemoryStoreValidation(t *testing.T) {
	if err := NewMemoryStore().Record(context.Background(), Event{}); !errors.Is(err, ErrInvalidEvent) {
		t.Fatalf("Record(invalid) error = %v, want ErrInvalidEvent", err)
	}
}

func TestNewSQLStoreValidationAndScan(t *testing.T) {
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

	store, err = NewSQLStore(db, WithTableName("public.audit_events"), WithSQLDialect(SQLDialectPostgres))
	if err != nil {
		t.Fatalf("NewSQLStore(custom) error = %v", err)
	}
	if store.table != "public.audit_events" || store.dialect != SQLDialectPostgres {
		t.Fatalf("SQLStore = %+v, want custom table and postgres dialect", store)
	}
	if got := store.placeholders(3, 2); got != "$2, $3, $4" {
		t.Fatalf("postgres placeholders = %q, want $2, $3, $4", got)
	}

	if _, err := NewSQLStore(db, WithTableName("audit_events;drop")); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("NewSQLStore(unsafe table) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := NewSQLStore(db, WithSQLDialect("oracle")); !errors.Is(err, ErrUnsupportedSQLDialect) {
		t.Fatalf("NewSQLStore(unsupported dialect) error = %v, want ErrUnsupportedSQLDialect", err)
	}

	createdAt := time.Date(2026, 7, 9, 10, 0, 0, 0, time.UTC)
	event, err := scanEvent(eventScannerFunc(func(dest ...any) error {
		*(dest[0].(*sql.NullString)) = sql.NullString{String: "event-1", Valid: true}
		*(dest[1].(*string)) = "tenant-a"
		*(dest[2].(*sql.NullString)) = sql.NullString{String: "user-1", Valid: true}
		*(dest[3].(*string)) = "orders.create"
		*(dest[4].(*string)) = "order:1"
		*(dest[5].(*time.Time)) = createdAt
		*(dest[6].(*string)) = `{"ip":"127.0.0.1"}`
		return nil
	}))
	if err != nil {
		t.Fatalf("scanEvent() error = %v", err)
	}
	wantMetadata := map[string]string{"ip": "127.0.0.1"}
	if event.ID != "event-1" || event.TenantID != "tenant-a" || event.ActorID != "user-1" || !reflect.DeepEqual(event.Metadata, wantMetadata) {
		t.Fatalf("scanEvent() = %+v, want decoded event", event)
	}
}

type eventScannerFunc func(dest ...any) error

func (fn eventScannerFunc) Scan(dest ...any) error {
	return fn(dest...)
}
