package identity

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"reflect"
	"testing"
)

func TestMemoryStoreTenantIsolationAndConflict(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	link := Link{
		TenantID:      "tenant-a",
		UserID:        "u1",
		Provider:      ProviderGoogle,
		Subject:       "sub-1",
		Email:         "u1@example.com",
		EmailVerified: true,
		Metadata:      map[string]string{"org": "a"},
	}
	if err := store.Link(ctx, link); err != nil {
		t.Fatalf("Link() error = %v", err)
	}
	link.Metadata["org"] = "mutated"

	got, err := store.GetByExternal(ctx, "tenant-a", ProviderGoogle, "sub-1")
	if err != nil {
		t.Fatalf("GetByExternal() error = %v", err)
	}
	if got.UserID != "u1" || got.Metadata["org"] != "a" {
		t.Fatalf("GetByExternal() = %+v, want cloned tenant-a link", got)
	}

	if _, err := store.GetByExternal(ctx, "tenant-b", ProviderGoogle, "sub-1"); !errors.Is(err, ErrIdentityNotFound) {
		t.Fatalf("GetByExternal(other tenant) error = %v, want ErrIdentityNotFound", err)
	}

	conflict := got
	conflict.UserID = "u2"
	if err := store.Link(ctx, conflict); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("Link(conflict) error = %v, want ErrIdentityConflict", err)
	}
}

func TestMemoryStoreGetByUserSortsLinks(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	links := []Link{
		{TenantID: "tenant-a", UserID: "u1", Provider: ProviderMicrosoft, Subject: "sub-2", Email: "u1@example.com"},
		{TenantID: "tenant-a", UserID: "u1", Provider: ProviderGoogle, Subject: "sub-1", Email: "u1@example.com"},
		{TenantID: "tenant-b", UserID: "u1", Provider: ProviderGitHub, Subject: "sub-3", Email: "u1@example.com"},
	}
	for _, link := range links {
		if err := store.Link(ctx, link); err != nil {
			t.Fatalf("Link() error = %v", err)
		}
	}

	got, err := store.GetByUser(ctx, "tenant-a", "u1")
	if err != nil {
		t.Fatalf("GetByUser() error = %v", err)
	}
	if len(got) != 2 || got[0].Provider != ProviderGoogle || got[1].Provider != ProviderMicrosoft {
		t.Fatalf("GetByUser() = %+v, want tenant-a links sorted by provider", got)
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

	store, err = NewSQLStore(db, WithTableName("public.identity_links"), WithSQLDialect(SQLDialectPostgres))
	if err != nil {
		t.Fatalf("NewSQLStore(custom) error = %v", err)
	}
	if store.table != "public.identity_links" || store.dialect != SQLDialectPostgres {
		t.Fatalf("SQLStore = %+v, want custom table and postgres dialect", store)
	}
	if got := store.placeholders(3, 2); got != "$2, $3, $4" {
		t.Fatalf("postgres placeholders = %q, want $2, $3, $4", got)
	}

	if _, err := NewSQLStore(db, WithTableName("identity_links;drop")); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("NewSQLStore(unsafe table) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := NewSQLStore(db, WithSQLDialect("oracle")); !errors.Is(err, ErrUnsupportedSQLDialect) {
		t.Fatalf("NewSQLStore(unsupported dialect) error = %v, want ErrUnsupportedSQLDialect", err)
	}

	link, err := scanLink(linkScannerFunc(func(dest ...any) error {
		*(dest[0].(*string)) = "tenant-a"
		*(dest[1].(*string)) = string(ProviderGoogle)
		*(dest[2].(*string)) = "sub-1"
		*(dest[3].(*string)) = "u1"
		*(dest[4].(*string)) = "u1@example.com"
		*(dest[5].(*string)) = "User 1"
		*(dest[6].(*bool)) = true
		*(dest[7].(*string)) = `{"org":"a"}`
		return nil
	}))
	if err != nil {
		t.Fatalf("scanLink() error = %v", err)
	}
	wantMetadata := map[string]string{"org": "a"}
	if link.TenantID != "tenant-a" || link.Provider != ProviderGoogle || link.Subject != "sub-1" || link.UserID != "u1" || !reflect.DeepEqual(link.Metadata, wantMetadata) {
		t.Fatalf("scanLink() = %+v, want decoded link", link)
	}
}

func TestSQLStoreLinkTreatsMySQLNoopUpdateAsSuccess(t *testing.T) {
	db := sql.OpenDB(identityTestConnector{found: true})
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewSQLStore(db)
	if err != nil {
		t.Fatalf("NewSQLStore() error = %v", err)
	}
	link := Link{
		TenantID: "tenant-a", Provider: ProviderGoogle, Subject: "sub-1",
		UserID: "u1", Email: "u1@example.com", EmailVerified: true,
		Metadata: map[string]string{"org": "a"},
	}

	if err := store.Link(context.Background(), link); err != nil {
		t.Fatalf("Link(existing unchanged identity) error = %v, want success", err)
	}
}

func TestSQLStoreNoopUpdateStillReportsMissingIdentity(t *testing.T) {
	db := sql.OpenDB(identityTestConnector{found: false})
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewSQLStore(db)
	if err != nil {
		t.Fatalf("NewSQLStore() error = %v", err)
	}
	link := Link{TenantID: "tenant-a", Provider: ProviderGoogle, Subject: "sub-1", UserID: "u1", Email: "u1@example.com"}

	err = store.requireUpdatedLink(context.Background(), link, driver.RowsAffected(0))
	if !errors.Is(err, ErrIdentityNotFound) {
		t.Fatalf("requireUpdatedLink() error = %v, want ErrIdentityNotFound", err)
	}
}

func TestConfirmUpdatedLinkRejectsConcurrentReplacement(t *testing.T) {
	desired := Link{TenantID: "tenant-a", Provider: ProviderGoogle, Subject: "sub-1", UserID: "u1", Email: "u1@example.com"}
	current := desired
	current.Email = "replacement@example.com"
	if err := confirmUpdatedLink(current, desired); !errors.Is(err, ErrIdentityConflict) {
		t.Fatalf("confirmUpdatedLink() error = %v, want ErrIdentityConflict", err)
	}
}

type linkScannerFunc func(dest ...any) error

func (fn linkScannerFunc) Scan(dest ...any) error {
	return fn(dest...)
}

type identityTestConnector struct {
	found bool
}

func (connector identityTestConnector) Connect(context.Context) (driver.Conn, error) {
	return identityTestConn(connector), nil
}

func (connector identityTestConnector) Driver() driver.Driver {
	return identityTestDriver(connector)
}

type identityTestDriver struct {
	found bool
}

func (driver identityTestDriver) Open(string) (driver.Conn, error) {
	return identityTestConn(driver), nil
}

type identityTestConn struct {
	found bool
}

func (conn identityTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare is not supported")
}

func (identityTestConn) Close() error { return nil }

func (identityTestConn) Begin() (driver.Tx, error) {
	return nil, errors.New("Begin is not supported")
}

func (identityTestConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	return driver.RowsAffected(0), nil
}

func (conn identityTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &identityTestRows{found: conn.found}, nil
}

type identityTestRows struct {
	found bool
	done  bool
}

func (*identityTestRows) Columns() []string {
	return []string{"tenant_id", "provider", "subject", "user_id", "email", "name", "email_verified", "metadata"}
}

func (*identityTestRows) Close() error { return nil }

func (rows *identityTestRows) Next(dest []driver.Value) error {
	if rows.done || !rows.found {
		return io.EOF
	}
	rows.done = true
	copy(dest, []driver.Value{"tenant-a", "google", "sub-1", "u1", "u1@example.com", "", true, `{"org":"a"}`})
	return nil
}
