package subscription

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DarkInno/gotenancy/core/types"
)

func TestMemoryServiceSubscribe(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	events := []BillingEvent{}
	service := NewMemoryService(
		WithClock(func() time.Time { return now }),
		WithBillingHook(func(_ context.Context, event BillingEvent) error {
			events = append(events, event)
			return nil
		}),
	)

	got, err := service.Subscribe(ctx, "tenant-a", "starter")
	if err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
	if got.TenantID != "tenant-a" || got.PlanID != "starter" || got.Status != StatusActive || !got.StartDate.Equal(now) {
		t.Fatalf("Subscribe() = %+v, want active subscription", got)
	}
	if len(events) != 1 || events[0].Action != "subscribe" || events[0].ToPlan != "starter" {
		t.Fatalf("events = %+v, want subscribe event", events)
	}
	if _, err := service.Subscribe(ctx, "tenant-a", "starter"); !errors.Is(err, ErrSubscriptionAlreadyExists) {
		t.Fatalf("Subscribe(duplicate) error = %v, want ErrSubscriptionAlreadyExists", err)
	}
}

func TestMemoryServiceUpgradeDowngradeUnsubscribe(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	service := NewMemoryService(WithClock(func() time.Time { return now }))
	if _, err := service.Subscribe(ctx, "tenant-a", "starter"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}

	upgraded, err := service.Upgrade(ctx, "tenant-a", "pro")
	if err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}
	if upgraded.PlanID != "pro" || upgraded.Status != StatusActive {
		t.Fatalf("Upgrade() = %+v, want pro active", upgraded)
	}

	downgraded, err := service.Downgrade(ctx, "tenant-a", "starter")
	if err != nil {
		t.Fatalf("Downgrade() error = %v", err)
	}
	if downgraded.PlanID != "starter" || downgraded.Status != StatusActive {
		t.Fatalf("Downgrade() = %+v, want starter active", downgraded)
	}

	now = now.Add(time.Hour)
	cancelled, err := service.Unsubscribe(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("Unsubscribe() error = %v", err)
	}
	if cancelled.Status != StatusCancelled || cancelled.EndDate == nil || !cancelled.EndDate.Equal(now) {
		t.Fatalf("Unsubscribe() = %+v, want cancelled with end date", cancelled)
	}

	if _, err := service.Upgrade(ctx, "tenant-a", "pro"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Upgrade(cancelled) error = %v, want ErrInvalidTransition", err)
	}
	if _, err := service.Unsubscribe(ctx, "tenant-a"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("Unsubscribe(cancelled) error = %v, want ErrInvalidTransition", err)
	}
}

func TestMemoryServiceGetCopiesSubscription(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()
	periodEnd := time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC)
	if _, err := service.SubscribeWithPeriod(ctx, "tenant-a", "starter", periodEnd); err != nil {
		t.Fatalf("SubscribeWithPeriod() error = %v", err)
	}
	cancelled, err := service.Unsubscribe(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("Unsubscribe() error = %v", err)
	}

	cancelled.CurrentPeriodEnd = nil
	cancelled.EndDate = nil
	got, err := service.Get(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.CurrentPeriodEnd == nil || got.EndDate == nil {
		t.Fatal("Get() period/end date = nil, want stored dates")
	}

	got.CurrentPeriodEnd = nil
	got.EndDate = nil
	again, err := service.Get(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("Get() again error = %v", err)
	}
	if again.CurrentPeriodEnd == nil || again.EndDate == nil {
		t.Fatal("mutating returned subscription changed stored dates")
	}
}

func TestMemoryServiceStoreCRUDAndList(t *testing.T) {
	ctx := context.Background()
	start := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	subscription := Subscription{TenantID: "tenant-b", PlanID: "pro", Status: StatusActive, StartDate: start}

	if err := store.Create(ctx, subscription); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if err := store.Create(ctx, subscription); !errors.Is(err, ErrSubscriptionAlreadyExists) {
		t.Fatalf("Create(duplicate) error = %v, want ErrSubscriptionAlreadyExists", err)
	}
	if err := store.Create(ctx, Subscription{TenantID: "tenant-a", PlanID: "starter", Status: StatusExpired, StartDate: start}); err != nil {
		t.Fatalf("Create(tenant-a) error = %v", err)
	}

	subscription.PlanID = "enterprise"
	if err := store.Update(ctx, subscription); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	got, err := store.Get(ctx, "tenant-b")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if got.PlanID != "enterprise" {
		t.Fatalf("Get().PlanID = %q, want enterprise", got.PlanID)
	}

	list, err := store.List(ctx, ListFilter{Statuses: []Status{StatusActive}})
	if err != nil {
		t.Fatalf("List(active) error = %v", err)
	}
	if len(list) != 1 || list[0].TenantID != "tenant-b" {
		t.Fatalf("List(active) = %+v, want tenant-b only", list)
	}

	list, err = store.List(ctx, ListFilter{Limit: 1})
	if err != nil {
		t.Fatalf("List(limit) error = %v", err)
	}
	if len(list) != 1 || list[0].TenantID != "tenant-a" {
		t.Fatalf("List(limit) = %+v, want tenant-a first", list)
	}

	if err := store.Delete(ctx, "tenant-b"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := store.Get(ctx, "tenant-b"); !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("Get(deleted) error = %v, want ErrSubscriptionNotFound", err)
	}
	if _, err := store.List(ctx, ListFilter{Statuses: []Status{"paused"}}); !errors.Is(err, ErrInvalidListFilter) {
		t.Fatalf("List(invalid status) error = %v, want ErrInvalidListFilter", err)
	}
}

func TestMemoryServiceExpireDueWithGraceAndRenew(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	periodEnd := now.Add(24 * time.Hour)
	events := []BillingEvent{}
	service := NewMemoryService(
		WithClock(func() time.Time { return now }),
		WithGracePeriod(2*time.Hour),
		WithBillingHook(func(_ context.Context, event BillingEvent) error {
			events = append(events, event)
			return nil
		}),
	)

	got, err := service.SubscribeWithPeriod(ctx, "tenant-a", "starter", periodEnd)
	if err != nil {
		t.Fatalf("SubscribeWithPeriod() error = %v", err)
	}
	if got.CurrentPeriodEnd == nil || !got.CurrentPeriodEnd.Equal(periodEnd) {
		t.Fatalf("CurrentPeriodEnd = %v, want %v", got.CurrentPeriodEnd, periodEnd)
	}
	if got.GracePeriodEnd == nil || !got.GracePeriodEnd.Equal(periodEnd.Add(2*time.Hour)) {
		t.Fatalf("GracePeriodEnd = %v, want %v", got.GracePeriodEnd, periodEnd.Add(2*time.Hour))
	}

	now = periodEnd.Add(time.Hour)
	expired, err := service.ExpireDue(ctx)
	if err != nil {
		t.Fatalf("ExpireDue(before grace end) error = %v", err)
	}
	if len(expired) != 0 {
		t.Fatalf("ExpireDue(before grace end) expired = %+v, want none", expired)
	}

	now = periodEnd.Add(2 * time.Hour)
	expired, err = service.ExpireDue(ctx)
	if err != nil {
		t.Fatalf("ExpireDue() error = %v", err)
	}
	if len(expired) != 1 || expired[0].Status != StatusExpired {
		t.Fatalf("ExpireDue() = %+v, want one expired subscription", expired)
	}
	if expired[0].EndDate == nil || !expired[0].EndDate.Equal(periodEnd.Add(2*time.Hour)) {
		t.Fatalf("expired EndDate = %v, want grace end", expired[0].EndDate)
	}

	renewEnd := periodEnd.Add(30 * 24 * time.Hour)
	renewed, err := service.Renew(ctx, "tenant-a", renewEnd)
	if err != nil {
		t.Fatalf("Renew() error = %v", err)
	}
	if renewed.Status != StatusActive || renewed.EndDate != nil || renewed.CurrentPeriodEnd == nil || !renewed.CurrentPeriodEnd.Equal(renewEnd) {
		t.Fatalf("Renew() = %+v, want active with new period end", renewed)
	}
	if len(events) != 3 || events[0].Action != "subscribe" || events[1].Action != "expire" || events[2].Action != "renew" {
		t.Fatalf("events = %+v, want subscribe/expire/renew", events)
	}
}

func TestMemoryServiceBillingHookCanReenterService(t *testing.T) {
	ctx := context.Background()
	var service *MemoryService
	service = NewMemoryService(WithBillingHook(func(ctx context.Context, event BillingEvent) error {
		got, err := service.Get(ctx, event.TenantID)
		if err != nil {
			return err
		}
		if got.PlanID != "starter" {
			t.Fatalf("hook Get() PlanID = %q, want starter", got.PlanID)
		}
		return nil
	}))

	if _, err := service.Subscribe(ctx, "tenant-a", "starter"); err != nil {
		t.Fatalf("Subscribe() error = %v", err)
	}
}

func TestMemoryServiceValidationAndMissing(t *testing.T) {
	ctx := context.Background()
	service := NewMemoryService()

	if _, err := service.Subscribe(ctx, "", "starter"); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Subscribe(empty tenant) error = %v, want ErrInvalidSubscription", err)
	}
	if _, err := service.Subscribe(ctx, "tenant-a", ""); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Subscribe(empty plan) error = %v, want ErrInvalidSubscription", err)
	}
	if _, err := service.Get(ctx, types.TenantID("missing")); !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("Get(missing) error = %v, want ErrSubscriptionNotFound", err)
	}
	if _, err := service.Upgrade(ctx, "missing", "pro"); !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("Upgrade(missing) error = %v, want ErrSubscriptionNotFound", err)
	}
	if _, err := service.Downgrade(ctx, "missing", "starter"); !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("Downgrade(missing) error = %v, want ErrSubscriptionNotFound", err)
	}
	if _, err := service.Unsubscribe(ctx, "missing"); !errors.Is(err, ErrSubscriptionNotFound) {
		t.Fatalf("Unsubscribe(missing) error = %v, want ErrSubscriptionNotFound", err)
	}
	if err := service.Create(ctx, Subscription{TenantID: "tenant-a", PlanID: "starter", Status: "paused", StartDate: time.Now()}); !errors.Is(err, ErrInvalidSubscription) {
		t.Fatalf("Create(invalid status) error = %v, want ErrInvalidSubscription", err)
	}
}

func TestNewSQLStoreValidation(t *testing.T) {
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

	store, err = NewSQLStore(db, WithTableName("public.saas_subscriptions"), WithSQLDialect(SQLDialectPostgres))
	if err != nil {
		t.Fatalf("NewSQLStore(custom) error = %v", err)
	}
	if store.table != "public.saas_subscriptions" || store.dialect != SQLDialectPostgres {
		t.Fatalf("SQLStore = %+v, want custom table and postgres dialect", store)
	}
	if got := store.placeholders(2, 3); got != "$3, $4" {
		t.Fatalf("postgres placeholders = %q, want $3, $4", got)
	}

	if _, err := NewSQLStore(db, WithTableName("saas_subscriptions;drop")); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("NewSQLStore(unsafe table) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := NewSQLStore(db, WithSQLDialect("oracle")); !errors.Is(err, ErrUnsupportedSQLDialect) {
		t.Fatalf("NewSQLStore(unsupported dialect) error = %v, want ErrUnsupportedSQLDialect", err)
	}
}
