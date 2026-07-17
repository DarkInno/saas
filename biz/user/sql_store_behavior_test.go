package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSQLStoreGetUserAndMemberMapMissingAndInvalidRows(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid user id does not query database", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectMySQL)
		if _, err := store.GetUser(ctx, ""); !errors.Is(err, ErrInvalidUser) {
			t.Fatalf("GetUser(empty id) error = %v, want ErrInvalidUser", err)
		}
		assertUserSQLMockExpectations(t, mock)
	})

	t.Run("mysql missing user maps not found", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectMySQL)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, name FROM biz_users WHERE id = ?")).
			WithArgs("missing-user").
			WillReturnError(sql.ErrNoRows)

		if _, err := store.GetUser(ctx, "missing-user"); !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("GetUser(missing) error = %v, want ErrUserNotFound", err)
		}
		assertUserSQLMockExpectations(t, mock)
	})

	t.Run("postgres invalid persisted user is rejected", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectPostgres)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, name FROM biz_users WHERE id = $1")).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows(userColumnNames()).AddRow("user-1", "", "Missing email"))

		if _, err := store.GetUser(ctx, "user-1"); !errors.Is(err, ErrInvalidUser) {
			t.Fatalf("GetUser(invalid persisted row) error = %v, want ErrInvalidUser", err)
		}
		assertUserSQLMockExpectations(t, mock)
	})

	t.Run("postgres missing member maps not found", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectPostgres)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT tenant_id, user_id, roles FROM biz_members WHERE tenant_id = $1 AND user_id = $2")).
			WithArgs("tenant-a", "user-1").
			WillReturnError(sql.ErrNoRows)

		if _, err := store.GetMember(ctx, "tenant-a", "user-1"); !errors.Is(err, ErrMemberNotFound) {
			t.Fatalf("GetMember(missing) error = %v, want ErrMemberNotFound", err)
		}
		assertUserSQLMockExpectations(t, mock)
	})

	t.Run("invalid member roles JSON is surfaced", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectMySQL)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT tenant_id, user_id, roles FROM biz_members WHERE tenant_id = ? AND user_id = ?")).
			WithArgs("tenant-a", "user-1").
			WillReturnRows(sqlmock.NewRows(memberColumnNames()).AddRow("tenant-a", "user-1", `["admin"`))

		if _, err := store.GetMember(ctx, "tenant-a", "user-1"); err == nil {
			t.Fatal("GetMember(invalid roles JSON) error = nil, want JSON decoding error")
		} else {
			var syntaxErr *json.SyntaxError
			if !errors.As(err, &syntaxErr) {
				t.Fatalf("GetMember(invalid roles JSON) error = %T %v, want *json.SyntaxError", err, err)
			}
		}
		assertUserSQLMockExpectations(t, mock)
	})
}

func TestSQLStoreCreateAndAddMemberMapDuplicateAndMissingUser(t *testing.T) {
	ctx := context.Background()

	t.Run("postgres duplicate user maps user exists", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectPostgres)
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO biz_users (id, email, name) VALUES ($1, $2, $3)")).
			WithArgs("user-1", "user@example.com", "User One").
			WillReturnError(errors.New("pq: duplicate key value violates unique constraint biz_users_pkey"))

		if err := store.CreateUser(ctx, User{ID: "user-1", Email: "user@example.com", Name: "User One"}); !errors.Is(err, ErrUserExists) {
			t.Fatalf("CreateUser(duplicate) error = %v, want ErrUserExists", err)
		}
		assertUserSQLMockExpectations(t, mock)
	})

	t.Run("mysql missing user prevents membership insert", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectMySQL)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, name FROM biz_users WHERE id = ?")).
			WithArgs("missing-user").
			WillReturnError(sql.ErrNoRows)

		if err := store.AddMember(ctx, Member{TenantID: "tenant-a", UserID: "missing-user", Roles: []string{"viewer"}}); !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("AddMember(missing user) error = %v, want ErrUserNotFound", err)
		}
		assertUserSQLMockExpectations(t, mock)
	})

	t.Run("postgres duplicate membership maps member exists", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectPostgres)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT id, email, name FROM biz_users WHERE id = $1")).
			WithArgs("user-1").
			WillReturnRows(sqlmock.NewRows(userColumnNames()).AddRow("user-1", "user@example.com", "User One"))
		mock.ExpectExec(regexp.QuoteMeta("INSERT INTO biz_members (tenant_id, user_id, roles) VALUES ($1, $2, $3)")).
			WithArgs("tenant-a", "user-1", `["admin","viewer"]`).
			WillReturnError(errors.New("pq: duplicate key value violates unique constraint biz_members_pkey"))

		if err := store.AddMember(ctx, Member{TenantID: "tenant-a", UserID: "user-1", Roles: []string{"admin", "viewer"}}); !errors.Is(err, ErrMemberExists) {
			t.Fatalf("AddMember(duplicate) error = %v, want ErrMemberExists", err)
		}
		assertUserSQLMockExpectations(t, mock)
	})
}

func TestSQLStoreListMembersPageStaysTenantScopedAndSurfacesRowFailures(t *testing.T) {
	ctx := context.Background()

	t.Run("postgres cursor page binds tenant and cursor", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectPostgres)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT tenant_id, user_id, roles FROM biz_members WHERE tenant_id = $1 AND user_id > $2 ORDER BY user_id LIMIT $3")).
			WithArgs("tenant-a", "user-1", 2).
			WillReturnRows(sqlmock.NewRows(memberColumnNames()).
				AddRow("tenant-a", "user-2", `["viewer"]`).
				AddRow("tenant-a", "user-3", `["admin"]`))

		members, err := store.ListMembersPage(ctx, "tenant-a", MemberListFilter{Cursor: "user-1", Limit: 2})
		if err != nil {
			t.Fatalf("ListMembersPage() error = %v", err)
		}
		if len(members) != 2 || members[0].TenantID != "tenant-a" || members[0].UserID != "user-2" || len(members[0].Roles) != 1 || members[0].Roles[0] != "viewer" || members[1].UserID != "user-3" {
			t.Fatalf("ListMembersPage() = %#v, want tenant-a members user-2/user-3", members)
		}
		assertUserSQLMockExpectations(t, mock)
	})

	t.Run("mysql tenant list query error is returned", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectMySQL)
		wantErr := errors.New("database unavailable")
		mock.ExpectQuery(regexp.QuoteMeta("SELECT tenant_id, user_id, roles FROM biz_members WHERE tenant_id = ? ORDER BY user_id")).
			WithArgs("tenant-a").
			WillReturnError(wantErr)

		if _, err := store.ListMembers(ctx, "tenant-a"); !errors.Is(err, wantErr) {
			t.Fatalf("ListMembers(query error) = %v, want %v", err, wantErr)
		}
		assertUserSQLMockExpectations(t, mock)
	})

	t.Run("mysql invalid JSON row is not returned as membership", func(t *testing.T) {
		store, mock := newUserSQLMockStore(t, SQLDialectMySQL)
		mock.ExpectQuery(regexp.QuoteMeta("SELECT tenant_id, user_id, roles FROM biz_members WHERE tenant_id = ? ORDER BY user_id LIMIT ?")).
			WithArgs("tenant-a", 1).
			WillReturnRows(sqlmock.NewRows(memberColumnNames()).AddRow("tenant-a", "user-1", `{"role":`))

		if _, err := store.ListMembersPage(ctx, "tenant-a", MemberListFilter{Limit: 1}); err == nil {
			t.Fatal("ListMembersPage(invalid roles JSON) error = nil, want JSON decoding error")
		}
		assertUserSQLMockExpectations(t, mock)
	})
}

func TestSQLStoreRemoveMemberZeroRowsMapsNotFound(t *testing.T) {
	store, mock := newUserSQLMockStore(t, SQLDialectMySQL)
	mock.ExpectExec(regexp.QuoteMeta("DELETE FROM biz_members WHERE tenant_id = ? AND user_id = ?")).
		WithArgs("tenant-a", "user-1").
		WillReturnResult(sqlmock.NewResult(0, 0))

	if err := store.RemoveMember(context.Background(), "tenant-a", "user-1"); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("RemoveMember(missing) error = %v, want ErrMemberNotFound", err)
	}
	assertUserSQLMockExpectations(t, mock)
}

func TestSQLStoreCancelledContextStopsEveryPublicPersistenceMethod(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	store, mock := newUserSQLMockStore(t, SQLDialectMySQL)

	if err := store.CreateUser(ctx, User{ID: "user-1", Email: "user@example.com"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("CreateUser(cancelled) error = %v, want context.Canceled", err)
	}
	if _, err := store.GetUser(ctx, "user-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetUser(cancelled) error = %v, want context.Canceled", err)
	}
	if err := store.AddMember(ctx, Member{TenantID: "tenant-a", UserID: "user-1"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("AddMember(cancelled) error = %v, want context.Canceled", err)
	}
	if _, err := store.GetMember(ctx, "tenant-a", "user-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("GetMember(cancelled) error = %v, want context.Canceled", err)
	}
	if _, err := store.ListMembersPage(ctx, "tenant-a", MemberListFilter{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListMembersPage(cancelled) error = %v, want context.Canceled", err)
	}
	if err := store.RemoveMember(ctx, "tenant-a", "user-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("RemoveMember(cancelled) error = %v, want context.Canceled", err)
	}
	assertUserSQLMockExpectations(t, mock)
}

func newUserSQLMockStore(t *testing.T, dialect SQLDialect) (*SQLStore, sqlmock.Sqlmock) {
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

func assertUserSQLMockExpectations(t *testing.T, mock sqlmock.Sqlmock) {
	t.Helper()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func userColumnNames() []string {
	return []string{"id", "email", "name"}
}

func memberColumnNames() []string {
	return []string{"tenant_id", "user_id", "roles"}
}
