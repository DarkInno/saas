package plan

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

const planColumns = "id, name, features, quotas"

func TestSQLStorePlanGetAndListProtectCatalogReads(t *testing.T) {
	ctx := context.Background()

	t.Run("missing and malformed plans do not become usable catalog entries", func(t *testing.T) {
		store, mock := newMockPlanSQLStore(t, SQLDialectPostgres)
		if _, err := store.Get(ctx, ""); !errors.Is(err, ErrInvalidPlan) {
			t.Fatalf("Get(blank) error = %v, want ErrInvalidPlan", err)
		}
		mock.ExpectQuery(regexp.QuoteMeta("SELECT " + planColumns + " FROM saas_plans WHERE id = $1")).
			WithArgs("missing").
			WillReturnError(sql.ErrNoRows)
		if _, err := store.Get(ctx, "missing"); !errors.Is(err, ErrPlanNotFound) {
			t.Fatalf("Get(missing) error = %v, want ErrPlanNotFound", err)
		}
		mock.ExpectQuery(regexp.QuoteMeta("SELECT " + planColumns + " FROM saas_plans WHERE id = $1")).
			WithArgs("broken").
			WillReturnRows(sqlmock.NewRows(planColumnNames()).AddRow("broken", "Broken", "[", "[]"))
		if _, err := store.Get(ctx, "broken"); err == nil {
			t.Fatal("Get(malformed features) error = nil, want JSON error")
		}
		assertPlanSQLMockExpectations(t, mock)
	})

	t.Run("tenant catalog page filters before cursor and bounds results", func(t *testing.T) {
		store, mock := newMockPlanSQLStore(t, SQLDialectPostgres)
		starter := testPlan("starter")
		mock.ExpectQuery(regexp.QuoteMeta("SELECT "+planColumns+" FROM saas_plans WHERE id IN ($1, $2) AND id > $3 ORDER BY id LIMIT $4")).
			WithArgs("enterprise", "starter", "enterprise", 2).
			WillReturnRows(mockPlanRows(t, starter))

		got, err := store.ListPage(ctx, PageFilter{IDs: []string{"enterprise", "starter"}, Cursor: "enterprise", Limit: 2})
		if err != nil {
			t.Fatalf("ListPage() error = %v", err)
		}
		if len(got) != 1 || got[0].ID != "starter" || got[0].Features[0].Config["seats"] != "5" {
			t.Fatalf("ListPage() = %#v, want decoded starter catalog entry", got)
		}
		assertPlanSQLMockExpectations(t, mock)
	})

	t.Run("database list failure is not interpreted as an empty catalog", func(t *testing.T) {
		store, mock := newMockPlanSQLStore(t, SQLDialectMySQL)
		wantErr := errors.New("catalog database unavailable")
		mock.ExpectQuery(regexp.QuoteMeta("SELECT " + planColumns + " FROM saas_plans ORDER BY id")).WillReturnError(wantErr)
		if _, err := store.List(ctx, ListFilter{}); !errors.Is(err, wantErr) {
			t.Fatalf("List() error = %v, want %v", err, wantErr)
		}
		assertPlanSQLMockExpectations(t, mock)
	})
}

func TestSQLStorePlanWritesDistinguishDuplicateNoOpConflictAndMissing(t *testing.T) {
	ctx := context.Background()
	plan := testPlan("starter")
	features, quotas := planJSON(t, plan)

	t.Run("duplicate plan is exposed as a semantic duplicate", func(t *testing.T) {
		store, mock := newMockPlanSQLStore(t, SQLDialectMySQL)
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO saas_plans (id, name, features, quotas) VALUES (?, ?, ?, ?)")).
			WithArgs(plan.ID, plan.Name, features, quotas).
			WillReturnError(errors.New("duplicate key value violates unique constraint"))
		if err := store.Create(ctx, plan); !errors.Is(err, ErrPlanAlreadyExists) {
			t.Fatalf("Create(duplicate) error = %v, want ErrPlanAlreadyExists", err)
		}
		assertPlanSQLMockExpectations(t, mock)
	})

	t.Run("unchanged update is accepted after readback", func(t *testing.T) {
		store, mock := newMockPlanSQLStore(t, SQLDialectMySQL)
		mock.ExpectExec(regexp.QuoteMeta("UPDATE saas_plans SET name = ?, features = ?, quotas = ? WHERE id = ?")).
			WithArgs(plan.Name, features, quotas, plan.ID).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT " + planColumns + " FROM saas_plans WHERE id = ?")).
			WithArgs(plan.ID).
			WillReturnRows(mockPlanRows(t, plan))
		if err := store.Update(ctx, plan); err != nil {
			t.Fatalf("Update(no-op) error = %v", err)
		}
		assertPlanSQLMockExpectations(t, mock)
	})

	t.Run("concurrent replacement is not silently accepted", func(t *testing.T) {
		store, mock := newMockPlanSQLStore(t, SQLDialectPostgres)
		current := clonePlan(plan)
		current.Name = "Enterprise replacement"
		mock.ExpectExec(regexp.QuoteMeta("UPDATE saas_plans SET name = $1, features = $2, quotas = $3 WHERE id = $4")).
			WithArgs(plan.Name, features, quotas, plan.ID).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectQuery(regexp.QuoteMeta("SELECT " + planColumns + " FROM saas_plans WHERE id = $1")).
			WithArgs(plan.ID).
			WillReturnRows(mockPlanRows(t, current))
		if err := store.Update(ctx, plan); !errors.Is(err, ErrPlanConflict) {
			t.Fatalf("Update(concurrent replacement) error = %v, want ErrPlanConflict", err)
		}
		assertPlanSQLMockExpectations(t, mock)
	})

	t.Run("delete of a missing catalog entry is visible", func(t *testing.T) {
		store, mock := newMockPlanSQLStore(t, SQLDialectPostgres)
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM saas_plans WHERE id = $1")).
			WithArgs("missing").
			WillReturnResult(sqlmock.NewResult(0, 0))
		if err := store.Delete(ctx, "missing"); !errors.Is(err, ErrPlanNotFound) {
			t.Fatalf("Delete(missing) error = %v, want ErrPlanNotFound", err)
		}
		assertPlanSQLMockExpectations(t, mock)
	})
}

func TestSQLStorePlanCancellationAndMalformedCatalogDataRemainVisible(t *testing.T) {
	plan := testPlan("starter")

	t.Run("canceled catalog operations do not reach storage", func(t *testing.T) {
		store, mock := newMockPlanSQLStore(t, SQLDialectMySQL)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if err := store.Create(ctx, plan); !errors.Is(err, context.Canceled) {
			t.Fatalf("Create(canceled) error = %v, want context.Canceled", err)
		}
		if _, err := store.Get(ctx, plan.ID); !errors.Is(err, context.Canceled) {
			t.Fatalf("Get(canceled) error = %v, want context.Canceled", err)
		}
		if _, err := store.List(ctx, ListFilter{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("List(canceled) error = %v, want context.Canceled", err)
		}
		if _, err := store.ListPage(ctx, PageFilter{}); !errors.Is(err, context.Canceled) {
			t.Fatalf("ListPage(canceled) error = %v, want context.Canceled", err)
		}
		if err := store.Update(ctx, plan); !errors.Is(err, context.Canceled) {
			t.Fatalf("Update(canceled) error = %v, want context.Canceled", err)
		}
		if err := store.Delete(ctx, plan.ID); !errors.Is(err, context.Canceled) {
			t.Fatalf("Delete(canceled) error = %v, want context.Canceled", err)
		}
		assertPlanSQLMockExpectations(t, mock)
	})

	t.Run("empty JSON collections are valid but malformed ones are rejected", func(t *testing.T) {
		store, mock := newMockPlanSQLStore(t, SQLDialectMySQL)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT " + planColumns + " FROM saas_plans WHERE id = ?")).
			WithArgs("empty").
			WillReturnRows(sqlmock.NewRows(planColumnNames()).AddRow("empty", "Empty", "", ""))
		empty, err := store.Get(context.Background(), "empty")
		if err != nil || empty.Features != nil || empty.Quotas != nil {
			t.Fatalf("Get(empty collections) = %#v, %v; want valid plan with nil collections", empty, err)
		}

		mock.ExpectQuery(regexp.QuoteMeta("SELECT " + planColumns + " FROM saas_plans ORDER BY id LIMIT ?")).
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows(planColumnNames()).AddRow("broken", "Broken", "[]", "not-json"))
		if _, err := store.List(context.Background(), ListFilter{Limit: 1}); err == nil {
			t.Fatal("List(malformed quotas) error = nil, want JSON error")
		}
		assertPlanSQLMockExpectations(t, mock)
	})

	t.Run("invalid page and write failures do not look successful", func(t *testing.T) {
		store, mock := newMockPlanSQLStore(t, SQLDialectPostgres)
		if _, err := store.List(context.Background(), ListFilter{Offset: 1}); !errors.Is(err, ErrInvalidListFilter) {
			t.Fatalf("List(offset without limit) error = %v, want ErrInvalidListFilter", err)
		}
		if _, err := store.ListPage(context.Background(), PageFilter{Cursor: "starter", Offset: 1, Limit: 1}); !errors.Is(err, ErrInvalidListFilter) {
			t.Fatalf("ListPage(cursor with offset) error = %v, want ErrInvalidListFilter", err)
		}

		features, quotas := planJSON(t, plan)
		writeErr := errors.New("catalog database unavailable")
		mock.ExpectExec(regexp.QuoteMeta("UPDATE saas_plans SET name = $1, features = $2, quotas = $3 WHERE id = $4")).
			WithArgs(plan.Name, features, quotas, plan.ID).
			WillReturnError(writeErr)
		if err := store.Update(context.Background(), plan); !errors.Is(err, writeErr) {
			t.Fatalf("Update(driver failure) error = %v, want %v", err, writeErr)
		}
		mock.ExpectExec(regexp.QuoteMeta("DELETE FROM saas_plans WHERE id = $1")).
			WithArgs(plan.ID).
			WillReturnResult(sqlmock.NewErrorResult(writeErr))
		if err := store.Delete(context.Background(), plan.ID); !errors.Is(err, writeErr) {
			t.Fatalf("Delete(rows affected failure) error = %v, want %v", err, writeErr)
		}
		assertPlanSQLMockExpectations(t, mock)
	})
}

func newMockPlanSQLStore(t *testing.T, dialect SQLDialect) (*SQLStore, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	store, err := NewSQLStore(db, WithSQLDialect(dialect))
	if err != nil {
		_ = db.Close()
		t.Fatalf("NewSQLStore() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return store, mock
}

func planJSON(t *testing.T, plan Plan) (string, string) {
	t.Helper()
	features, quotas, err := marshalPlanParts(plan)
	if err != nil {
		t.Fatalf("marshalPlanParts() error = %v", err)
	}
	return features, quotas
}

func mockPlanRows(t *testing.T, plan Plan) *sqlmock.Rows {
	t.Helper()
	features, quotas := planJSON(t, plan)
	return sqlmock.NewRows(planColumnNames()).AddRow(plan.ID, plan.Name, features, quotas)
}

func planColumnNames() []string {
	return []string{"id", "name", "features", "quotas"}
}

func assertPlanSQLMockExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
