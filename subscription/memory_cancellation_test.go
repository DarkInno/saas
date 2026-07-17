package subscription

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryServiceCanceledRequestsLeaveSubscriptionStateUntouched(t *testing.T) {
	service := NewMemoryService()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	valid := Subscription{
		TenantID:  "tenant-a",
		PlanID:    "starter",
		Status:    StatusActive,
		StartDate: time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC),
	}
	periodEnd := valid.StartDate.AddDate(0, 1, 0)

	tests := []struct {
		name string
		run  func() error
	}{
		{name: "create", run: func() error { return service.Create(ctx, valid) }},
		{name: "subscribe", run: func() error { _, err := service.Subscribe(ctx, valid.TenantID, valid.PlanID); return err }},
		{name: "subscribe with period", run: func() error {
			_, err := service.SubscribeWithPeriod(ctx, valid.TenantID, valid.PlanID, periodEnd)
			return err
		}},
		{name: "unsubscribe", run: func() error { _, err := service.Unsubscribe(ctx, valid.TenantID); return err }},
		{name: "renew", run: func() error { _, err := service.Renew(ctx, valid.TenantID, periodEnd); return err }},
		{name: "expire", run: func() error { _, err := service.Expire(ctx, valid.TenantID); return err }},
		{name: "expire due", run: func() error { _, err := service.ExpireDue(ctx); return err }},
		{name: "get", run: func() error { _, err := service.Get(ctx, valid.TenantID); return err }},
		{name: "list", run: func() error { _, err := service.List(ctx, ListFilter{}); return err }},
		{name: "list page", run: func() error { _, err := service.ListPage(ctx, PageFilter{}); return err }},
		{name: "update", run: func() error { return service.Update(ctx, valid) }},
		{name: "delete", run: func() error { return service.Delete(ctx, valid.TenantID) }},
		{name: "upgrade", run: func() error { _, err := service.Upgrade(ctx, valid.TenantID, "pro"); return err }},
		{name: "downgrade", run: func() error { _, err := service.Downgrade(ctx, valid.TenantID, "free"); return err }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); !errors.Is(err, context.Canceled) {
				t.Fatalf("%s canceled error = %v, want context.Canceled", tt.name, err)
			}
		})
	}

	if _, err := service.Get(context.Background(), valid.TenantID); !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("Get(after canceled requests) error = %v, want no persisted subscription", err)
	}
}

func TestMemoryServiceRejectsInvalidLifecycleInputBeforeMutation(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()

	if err := service.Create(ctx, Subscription{}); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Create(invalid) error = %v, want ErrInvalidSubscription", err)
	}
	if _, err := service.Subscribe(ctx, "", "starter"); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Subscribe(blank tenant) error = %v, want ErrInvalidSubscription", err)
	}
	if _, err := service.SubscribeWithPeriod(ctx, "tenant-a", "starter", time.Time{}); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("SubscribeWithPeriod(zero period) error = %v, want ErrInvalidSubscription", err)
	}
	if _, err := service.Renew(ctx, "", time.Now()); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Renew(blank tenant) error = %v, want ErrInvalidSubscription", err)
	}
	if _, err := service.Expire(ctx, ""); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Expire(blank tenant) error = %v, want ErrInvalidSubscription", err)
	}
	if _, err := service.Get(ctx, ""); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Get(blank tenant) error = %v, want ErrInvalidSubscription", err)
	}
	if _, err := service.List(ctx, ListFilter{Offset: 1}); !errors.Is(err, ErrInvalidListFilter) {
		t.Fatalf("List(offset without limit) error = %v, want ErrInvalidListFilter", err)
	}
	if _, err := service.ListPage(ctx, PageFilter{Cursor: "tenant-a", Offset: 1, Limit: 1}); !errors.Is(err, ErrInvalidListFilter) {
		t.Fatalf("ListPage(cursor with offset) error = %v, want ErrInvalidListFilter", err)
	}
	if err := service.Update(ctx, Subscription{}); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Update(invalid) error = %v, want ErrInvalidSubscription", err)
	}
	if err := service.Delete(ctx, ""); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Delete(blank tenant) error = %v, want ErrInvalidSubscription", err)
	}
	if _, err := service.Upgrade(ctx, "tenant-a", ""); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Upgrade(blank plan) error = %v, want ErrInvalidSubscription", err)
	}

	if subscriptions, err := service.List(ctx, ListFilter{}); err != nil || len(subscriptions) != 0 {
		t.Fatalf("List(after invalid requests) = %#v, %v; want empty state", subscriptions, err)
	}
}
