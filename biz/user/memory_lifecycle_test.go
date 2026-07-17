package user

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryServiceMembershipRemovalRemainsTenantScoped(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()
	if err := service.CreateUser(ctx, User{ID: "user-1", Email: "user@example.com"}); err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	for _, member := range []Member{
		{TenantID: "tenant-a", UserID: "user-1", Roles: []string{"viewer"}},
		{TenantID: "tenant-b", UserID: "user-1", Roles: []string{"owner"}},
	} {
		if err := service.AddMember(ctx, member); err != nil {
			t.Fatalf("AddMember(%s) error = %v", member.TenantID, err)
		}
	}

	member, err := service.GetMember(ctx, "tenant-a", "user-1")
	if err != nil {
		t.Fatalf("GetMember(tenant-a) error = %v", err)
	}
	member.Roles[0] = "mutated"
	if err := service.RemoveMember(ctx, "tenant-a", "user-1"); err != nil {
		t.Fatalf("RemoveMember(tenant-a) error = %v", err)
	}
	if _, err := service.GetMember(ctx, "tenant-a", "user-1"); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("GetMember(removed tenant) error = %v, want ErrMemberNotFound", err)
	}
	remaining, err := service.GetMember(ctx, "tenant-b", "user-1")
	if err != nil || len(remaining.Roles) != 1 || remaining.Roles[0] != "owner" {
		t.Fatalf("GetMember(tenant-b) = %#v, %v; want unaffected owner membership", remaining, err)
	}
	if err := service.RemoveMember(ctx, "tenant-a", "user-1"); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("RemoveMember(missing) error = %v, want ErrMemberNotFound", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := service.ListMembers(canceled, "tenant-b"); !errors.Is(err, context.Canceled) {
		t.Fatalf("ListMembers(canceled) error = %v, want context.Canceled", err)
	}
	if _, err := service.ListMembersPage(ctx, "", MemberListFilter{}); !errors.Is(err, ErrInvalidUser) {
		t.Fatalf("ListMembersPage(blank tenant) error = %v, want ErrInvalidUser", err)
	}
}
