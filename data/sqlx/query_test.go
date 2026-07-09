package sqlxtenant

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	tenantctx "github.com/DarkInno/gotenancy/core/context"
	"github.com/DarkInno/gotenancy/core/types"
	"github.com/DarkInno/gotenancy/data"
)

func TestQueryAddsTenantFilter(t *testing.T) {
	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})
	query, args, err := Query(ctx, "SELECT * FROM orders")
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if query != "SELECT * FROM orders WHERE tenant_id = ?" {
		t.Fatalf("Query() sql = %q", query)
	}
	if len(args) != 1 || args[0] != "tenant-a" {
		t.Fatalf("Query() args = %#v, want tenant-a", args)
	}

	query, args, err = Query(ctx, "SELECT * FROM orders WHERE status = ?", data.WithTenantField("orders.tenant_id"))
	if err != nil {
		t.Fatalf("Query(where) error = %v", err)
	}
	if query != "SELECT * FROM orders WHERE status = ? AND orders.tenant_id = ?" {
		t.Fatalf("Query(where) sql = %q", query)
	}
	if len(args) != 1 || args[0] != "tenant-a" {
		t.Fatalf("Query(where) args = %#v, want tenant-a", args)
	}
}

func TestTenantConditionForComplexSQL(t *testing.T) {
	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})

	condition, err := TenantCondition(ctx, data.WithTenantField("orders.tenant_id"))
	if err != nil {
		t.Fatalf("TenantCondition() error = %v", err)
	}
	if condition.Expression != "orders.tenant_id = ?" {
		t.Fatalf("TenantCondition().Expression = %q, want orders.tenant_id = ?", condition.Expression)
	}
	if len(condition.Args) != 1 || condition.Args[0] != "tenant-a" {
		t.Fatalf("TenantCondition().Args = %#v, want tenant-a", condition.Args)
	}

	hostCondition, err := TenantCondition(tenantctx.WithHost(context.Background()))
	if err != nil {
		t.Fatalf("TenantCondition(host) error = %v", err)
	}
	if !hostCondition.Empty() {
		t.Fatalf("TenantCondition(host) = %+v, want empty", hostCondition)
	}
}

func TestQueryHostContext(t *testing.T) {
	query, args, err := QueryWithArgs(tenantctx.WithHost(context.Background()), "SELECT * FROM orders WHERE status = ?", []any{"open"})
	if err != nil {
		t.Fatalf("Query(host) error = %v", err)
	}
	if query != "SELECT * FROM orders WHERE status = ?" || len(args) != 1 || args[0] != "open" {
		t.Fatalf("Query(host) = %q, %#v; want original sql and args", query, args)
	}
}

func TestQueryValidation(t *testing.T) {
	if _, _, err := Query(context.Background(), "SELECT * FROM orders"); !errors.Is(err, data.ErrNoTenant) {
		t.Fatalf("Query(no tenant) error = %v, want ErrNoTenant", err)
	}
	if _, _, err := Query(tenantctx.WithHost(context.Background()), " "); !errors.Is(err, ErrUnsafeSQL) {
		t.Fatalf("Query(empty sql) error = %v, want ErrUnsafeSQL", err)
	}
}

func TestQueryRejectsUnsafeTenantRewriteSQL(t *testing.T) {
	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})
	tests := []string{
		"INSERT INTO orders (name) VALUES (?)",
		"SELECT * FROM orders ORDER BY created_at DESC",
		"SELECT * FROM orders LIMIT 10",
		"SELECT * FROM orders JOIN order_items ON order_items.order_id = orders.id",
		"UPDATE orders SET name = ? WHERE id = ? RETURNING id",
		"SELECT * FROM orders; DELETE FROM orders",
		"SELECT * FROM orders -- WHERE tenant_id = ?",
	}

	for _, sql := range tests {
		if _, _, err := Query(ctx, sql); !errors.Is(err, ErrUnsafeSQL) {
			t.Fatalf("Query(%q) error = %v, want ErrUnsafeSQL", sql, err)
		}
	}
}

func TestExecWrappers(t *testing.T) {
	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})
	db := &fakeDB{}

	if err := SelectContext(ctx, db, &[]string{}, "SELECT * FROM orders WHERE status = ?", []any{"open"}); err != nil {
		t.Fatalf("SelectContext() error = %v", err)
	}
	if db.query != "SELECT * FROM orders WHERE status = ? AND tenant_id = ?" || db.args[0] != "open" || db.args[1] != "tenant-a" {
		t.Fatalf("SelectContext captured %q %#v", db.query, db.args)
	}

	if _, err := ExecContext(ctx, db, "UPDATE orders SET name = ? WHERE id = ?", []any{"updated", 1}); err != nil {
		t.Fatalf("ExecContext() error = %v", err)
	}
	if db.query != "UPDATE orders SET name = ? WHERE id = ? AND tenant_id = ?" || db.args[0] != "updated" || db.args[1] != 1 || db.args[2] != "tenant-a" {
		t.Fatalf("ExecContext captured %q %#v", db.query, db.args)
	}
}

type fakeDB struct {
	query string
	args  []interface{}
}

func (db *fakeDB) SelectContext(_ context.Context, _ interface{}, query string, args ...interface{}) error {
	db.query = query
	db.args = args
	return nil
}

func (db *fakeDB) GetContext(_ context.Context, _ interface{}, query string, args ...interface{}) error {
	db.query = query
	db.args = args
	return nil
}

func (db *fakeDB) ExecContext(_ context.Context, query string, args ...interface{}) (sql.Result, error) {
	db.query = query
	db.args = args
	return fakeResult(1), nil
}

type fakeResult int64

func (result fakeResult) LastInsertId() (int64, error) {
	return 0, nil
}

func (result fakeResult) RowsAffected() (int64, error) {
	return int64(result), nil
}
