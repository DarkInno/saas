package identity

import (
	"context"
	"errors"
	"testing"
)

func TestMemoryStoreUnlinkRemovesOnlyRequestedTenantIdentity(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	for _, link := range []Link{
		{TenantID: "tenant-a", UserID: "user-1", Provider: ProviderGoogle, Subject: "subject-1", Email: "user@example.com", Metadata: map[string]string{"tenant": "a"}},
		{TenantID: "tenant-b", UserID: "user-1", Provider: ProviderGoogle, Subject: "subject-1", Email: "user@example.com", Metadata: map[string]string{"tenant": "b"}},
	} {
		if err := store.Link(ctx, link); err != nil {
			t.Fatalf("Link(%s) error = %v", link.TenantID, err)
		}
	}

	links, err := store.GetByUser(ctx, "tenant-a", "user-1")
	if err != nil || len(links) != 1 {
		t.Fatalf("GetByUser(tenant-a) = %#v, %v; want one link", links, err)
	}
	links[0].Metadata["tenant"] = "mutated"

	if err := store.Unlink(ctx, "tenant-a", ProviderGoogle, "subject-1"); err != nil {
		t.Fatalf("Unlink(tenant-a) error = %v", err)
	}
	if _, err := store.GetByExternal(ctx, "tenant-a", ProviderGoogle, "subject-1"); !errors.Is(err, ErrIdentityNotFound) {
		t.Fatalf("GetByExternal(unlinked tenant) error = %v, want ErrIdentityNotFound", err)
	}
	otherTenant, err := store.GetByExternal(ctx, "tenant-b", ProviderGoogle, "subject-1")
	if err != nil || otherTenant.Metadata["tenant"] != "b" {
		t.Fatalf("GetByExternal(tenant-b) = %#v, %v; want unaffected cloned tenant-b link", otherTenant, err)
	}
	if err := store.Unlink(ctx, "tenant-a", ProviderGoogle, "subject-1"); !errors.Is(err, ErrIdentityNotFound) {
		t.Fatalf("Unlink(missing) error = %v, want ErrIdentityNotFound", err)
	}
	if err := store.Unlink(ctx, "tenant-b", "", "subject-1"); !errors.Is(err, ErrInvalidIdentity) {
		t.Fatalf("Unlink(invalid) error = %v, want ErrInvalidIdentity", err)
	}

	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := store.Unlink(canceled, "tenant-b", ProviderGoogle, "subject-1"); !errors.Is(err, context.Canceled) {
		t.Fatalf("Unlink(canceled) error = %v, want context.Canceled", err)
	}
	if _, err := store.GetByExternal(ctx, "tenant-b", ProviderGoogle, "subject-1"); err != nil {
		t.Fatalf("GetByExternal(after canceled unlink) error = %v, want retained link", err)
	}
}
