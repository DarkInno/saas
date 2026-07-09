package user

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
)

func TestMemoryServiceUserAndMembers(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()

	if err := service.CreateUser(ctx, User{ID: "u1", Email: "u1@example.com", Name: "User 1"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	if err := service.CreateUser(ctx, User{ID: "u1", Email: "u1@example.com"}); !errors.Is(err, ErrUserExists) {
		t.Fatalf("CreateUser(duplicate) error = %v, want ErrUserExists", err)
	}

	member := Member{TenantID: "tenant-a", UserID: "u1", Roles: []string{"admin"}}
	if err := service.AddMember(ctx, member); err != nil {
		t.Fatalf("AddMember() error = %v", err)
	}
	member.Roles[0] = "mutated"

	got, err := service.GetMember(ctx, "tenant-a", "u1")
	if err != nil {
		t.Fatalf("GetMember() error = %v", err)
	}
	if got.Roles[0] != "admin" {
		t.Fatalf("GetMember roles = %#v, want admin copy", got.Roles)
	}

	members, err := service.ListMembers(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("ListMembers() error = %v", err)
	}
	if len(members) != 1 || members[0].UserID != "u1" {
		t.Fatalf("ListMembers() = %+v, want u1", members)
	}

	if _, err := service.GetMember(ctx, "tenant-b", "u1"); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("GetMember(other tenant) error = %v, want ErrMemberNotFound", err)
	}
	if err := service.RemoveMember(ctx, "tenant-a", "u1"); err != nil {
		t.Fatalf("RemoveMember() error = %v", err)
	}
}

func TestMemoryServiceValidation(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()

	if err := service.CreateUser(ctx, User{}); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("CreateUser(invalid) error = %v, want ErrInvalidUser", err)
	}
	if err := service.AddMember(ctx, Member{TenantID: "tenant-a", UserID: "missing"}); !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("AddMember(missing user) error = %v, want ErrUserNotFound", err)
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
	if store.usersTable != DefaultSQLUsersTableName || store.membersTable != DefaultSQLMembersTableName {
		t.Fatalf("default tables = %q/%q, want %q/%q", store.usersTable, store.membersTable, DefaultSQLUsersTableName, DefaultSQLMembersTableName)
	}

	store, err = NewSQLStore(db, WithUsersTableName("public.biz_users"), WithMembersTableName("public.biz_members"), WithSQLDialect(SQLDialectPostgres))
	if err != nil {
		t.Fatalf("NewSQLStore(custom) error = %v", err)
	}
	if store.usersTable != "public.biz_users" || store.membersTable != "public.biz_members" || store.dialect != SQLDialectPostgres {
		t.Fatalf("SQLStore = %+v, want custom tables and postgres dialect", store)
	}
	if got := store.placeholders(3, 2); got != "$2, $3, $4" {
		t.Fatalf("postgres placeholders = %q, want $2, $3, $4", got)
	}

	if _, err := NewSQLStore(db, WithUsersTableName("biz_users;drop")); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("NewSQLStore(unsafe users table) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := NewSQLStore(db, WithMembersTableName("biz_members;drop")); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("NewSQLStore(unsafe members table) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := NewSQLStore(db, WithSQLDialect("oracle")); !errors.Is(err, ErrUnsupportedSQLDialect) {
		t.Fatalf("NewSQLStore(unsupported dialect) error = %v, want ErrUnsupportedSQLDialect", err)
	}

	raw, err := marshalRoles([]string{"admin", "viewer"})
	if err != nil {
		t.Fatalf("marshalRoles() error = %v", err)
	}
	roles, err := unmarshalRoles(raw)
	if err != nil {
		t.Fatalf("unmarshalRoles() error = %v", err)
	}
	if !reflect.DeepEqual(roles, []string{"admin", "viewer"}) {
		t.Fatalf("unmarshalRoles() = %#v, want admin/viewer", roles)
	}

	user, err := scanUser(scannerFunc(func(dest ...any) error {
		*(dest[0].(*string)) = "u1"
		*(dest[1].(*string)) = "u1@example.com"
		*(dest[2].(*string)) = "User 1"
		return nil
	}))
	if err != nil {
		t.Fatalf("scanUser() error = %v", err)
	}
	if user.ID != "u1" || user.Email != "u1@example.com" || user.Name != "User 1" {
		t.Fatalf("scanUser() = %+v, want decoded user", user)
	}

	member, err := scanMember(scannerFunc(func(dest ...any) error {
		*(dest[0].(*string)) = "tenant-a"
		*(dest[1].(*string)) = "u1"
		*(dest[2].(*string)) = raw
		return nil
	}))
	if err != nil {
		t.Fatalf("scanMember() error = %v", err)
	}
	if member.TenantID != "tenant-a" || member.UserID != "u1" || !reflect.DeepEqual(member.Roles, []string{"admin", "viewer"}) {
		t.Fatalf("scanMember() = %+v, want decoded member", member)
	}
}

type scannerFunc func(dest ...any) error

func (fn scannerFunc) Scan(dest ...any) error {
	return fn(dest...)
}
