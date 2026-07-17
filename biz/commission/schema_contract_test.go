package commission

import (
	"errors"
	"reflect"
	"testing"
)

func TestRequiredSQLIndexesDefaultContract(t *testing.T) {
	t.Parallel()

	specs, err := RequiredSQLIndexes(DefaultSQLTableNames(), SQLDialectMySQL)
	if err != nil {
		t.Fatalf("RequiredSQLIndexes() error = %v", err)
	}

	byName := sqlIndexSpecsByName(specs)
	assertSQLIndexSpec(t, byName, SQLIndexSpec{
		Name:    "commission_events_source_uq",
		Table:   DefaultSQLEventTableName,
		Columns: []string{"tenant_id", "source_type", "source_id"},
		Unique:  true,
	})
	assertSQLIndexSpec(t, byName, SQLIndexSpec{
		Name:    "commission_earnings_tenant_id_uq",
		Table:   DefaultSQLEarningTableName,
		Columns: []string{"tenant_id", "id"},
		Unique:  true,
	})
	assertSQLIndexSpec(t, byName, SQLIndexSpec{
		Name:    "commission_earnings_due_idx",
		Table:   DefaultSQLEarningTableName,
		Columns: []string{"status", "available_at", "id"},
	})
	assertSQLIndexSpec(t, byName, SQLIndexSpec{
		Name:    "commission_journals_tenant_earning_history_idx",
		Table:   DefaultSQLJournalTableName,
		Columns: []string{"tenant_id", "earning_id", "created_at", "id"},
	})
	assertSQLIndexSpec(t, byName, SQLIndexSpec{
		Name:    "commission_outbox_unpublished_idx",
		Table:   DefaultSQLOutboxTableName,
		Columns: []string{"tenant_id", "published_at", "created_at", "id"},
	})
	assertSQLIndexSpec(t, byName, SQLIndexSpec{
		Name:    "commission_settlements_tenant_status_idx",
		Table:   DefaultSQLSettlementTableName,
		Columns: []string{"tenant_id", "status", "updated_at", "id"},
	})
	assertSQLIndexSpec(t, byName, SQLIndexSpec{
		Name:    "commission_settlement_items_earning_idx",
		Table:   DefaultSQLSettlementItemTableName,
		Columns: []string{"tenant_id", "earning_id", "settlement_id"},
	})

	if len(byName) != len(specs) {
		t.Fatalf("RequiredSQLIndexes() returned duplicate names: %d specs, %d names", len(specs), len(byName))
	}
}

func TestRequiredSQLIndexesUsesPartialOutboxIndexWhereSupported(t *testing.T) {
	t.Parallel()

	for _, dialect := range []SQLDialect{SQLDialectPostgres, SQLDialectSQLite} {
		dialect := dialect
		t.Run(string(dialect), func(t *testing.T) {
			t.Parallel()
			specs, err := RequiredSQLIndexes(DefaultSQLTableNames(), dialect)
			if err != nil {
				t.Fatalf("RequiredSQLIndexes() error = %v", err)
			}
			assertSQLIndexSpec(t, sqlIndexSpecsByName(specs), SQLIndexSpec{
				Name:      "commission_outbox_unpublished_idx",
				Table:     DefaultSQLOutboxTableName,
				Columns:   []string{"tenant_id", "created_at", "id"},
				Predicate: "published_at IS NULL",
			})
		})
	}
}

func TestRequiredSQLIndexesUsesConfiguredTablesAndRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	names := DefaultSQLTableNames()
	names.Earnings = "finance.commission_earnings"
	specs, err := RequiredSQLIndexes(names, SQLDialectMySQL)
	if err != nil {
		t.Fatalf("RequiredSQLIndexes() error = %v", err)
	}
	assertSQLIndexSpec(t, sqlIndexSpecsByName(specs), SQLIndexSpec{
		Name:    "commission_earnings_due_idx",
		Table:   "finance.commission_earnings",
		Columns: []string{"status", "available_at", "id"},
	})

	unsafe := DefaultSQLTableNames()
	unsafe.Events = "commission_events; DROP TABLE tenants"
	if _, err := RequiredSQLIndexes(unsafe, SQLDialectMySQL); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("RequiredSQLIndexes(unsafe tables) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := RequiredSQLIndexes(DefaultSQLTableNames(), SQLDialect("oracle")); !errors.Is(err, ErrUnsupportedSQLDialect) {
		t.Fatalf("RequiredSQLIndexes(unsupported dialect) error = %v, want ErrUnsupportedSQLDialect", err)
	}
}

func sqlIndexSpecsByName(specs []SQLIndexSpec) map[string]SQLIndexSpec {
	byName := make(map[string]SQLIndexSpec, len(specs))
	for _, spec := range specs {
		byName[spec.Name] = spec
	}
	return byName
}

func assertSQLIndexSpec(t *testing.T, specs map[string]SQLIndexSpec, want SQLIndexSpec) {
	t.Helper()
	got, ok := specs[want.Name]
	if !ok {
		t.Fatalf("RequiredSQLIndexes() missing %q", want.Name)
	}
	if got.Table != want.Table {
		t.Errorf("%s table = %q, want %q", want.Name, got.Table, want.Table)
	}
	if !reflect.DeepEqual(got.Columns, want.Columns) {
		t.Errorf("%s columns = %#v, want %#v", want.Name, got.Columns, want.Columns)
	}
	if got.Unique != want.Unique {
		t.Errorf("%s unique = %t, want %t", want.Name, got.Unique, want.Unique)
	}
	if got.Predicate != want.Predicate {
		t.Errorf("%s predicate = %q, want %q", want.Name, got.Predicate, want.Predicate)
	}
}
