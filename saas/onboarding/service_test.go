package onboarding

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DarkInno/gotenancy/biz/audit"
	"github.com/DarkInno/gotenancy/biz/notification"
	"github.com/DarkInno/gotenancy/core/store"
	"github.com/DarkInno/gotenancy/core/types"
	saasfeature "github.com/DarkInno/gotenancy/saas/feature"
	saasplan "github.com/DarkInno/gotenancy/saas/plan"
	saasquota "github.com/DarkInno/gotenancy/saas/quota"
	saassubscription "github.com/DarkInno/gotenancy/saas/subscription"
	saastenant "github.com/DarkInno/gotenancy/saas/tenant"
)

func TestServiceOnboardInitializesSaaSModules(t *testing.T) {
	ctx := context.Background()
	plans := saasplan.NewMemoryService()
	plan := saasplan.Plan{
		ID:   "starter",
		Name: "Starter",
		Features: []saasplan.Feature{{
			Key:     "exports",
			Enabled: false,
			Config:  map[string]string{"limit": "10"},
		}},
		Quotas: []saasplan.Quota{{
			Resource: "api_calls",
			Limit:    100,
			Period:   saasplan.QuotaPeriodMonth,
		}},
	}
	if err := plans.Create(ctx, plan); err != nil {
		t.Fatalf("plans.Create() error = %v", err)
	}

	features := saasfeature.NewMemoryStore()
	quotas := saasquota.NewMemoryStore()
	audits := audit.NewMemoryStore()
	notifier := notification.NewMemoryNotifier()
	subscriptions := saassubscription.NewMemoryService()
	service := New(
		saastenant.New(store.NewMemoryStore()),
		plans,
		subscriptions,
		WithFeatureStore(features),
		WithQuotaStore(quotas),
		WithAuditStore(audits),
		WithNotifier(notifier),
	)
	periodEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)

	result, err := service.Onboard(ctx, Input{
		Tenant: saastenant.CreateInput{
			ID:     "tenant-a",
			Name:   "Tenant A",
			PlanID: "starter",
			Config: map[string]string{"region": "us"},
		},
		SubscriptionPeriodEnd: &periodEnd,
		FeatureOverrides: []saasfeature.Flag{{
			Key:     "exports",
			Enabled: true,
			Config:  map[string]string{"limit": "20"},
		}},
		Welcome: &notification.Message{
			TenantID: "tenant-b",
			Channel:  "email",
			To:       "owner@example.com",
			Subject:  "Welcome",
		},
		AuditMetadata: map[string]string{"source": "signup"},
	})
	if err != nil {
		t.Fatalf("Onboard() error = %v", err)
	}

	if result.Tenant.ID != "tenant-a" || result.Tenant.Status != "active" || result.Tenant.PlanID != "starter" {
		t.Fatalf("Tenant = %+v, want active starter tenant", result.Tenant)
	}
	if result.Subscription.Status != saassubscription.StatusActive || result.Subscription.CurrentPeriodEnd == nil || !result.Subscription.CurrentPeriodEnd.Equal(periodEnd) {
		t.Fatalf("Subscription = %+v, want active subscription with period end", result.Subscription)
	}
	if len(result.Features) != 1 || !result.Features[0].Enabled || result.Features[0].Config["limit"] != "20" {
		t.Fatalf("Features = %+v, want tenant override", result.Features)
	}
	if len(result.QuotaLimits) != 1 || result.QuotaLimits[0].Resource != "api_calls" || result.QuotaLimits[0].Period != saasquota.PeriodMonth {
		t.Fatalf("QuotaLimits = %+v, want converted monthly API quota", result.QuotaLimits)
	}

	used, err := quotas.Get(ctx, "tenant-a", "api_calls", saasquota.PeriodMonth)
	if err != nil {
		t.Fatalf("quotas.Get() error = %v", err)
	}
	if used != 0 {
		t.Fatalf("quota usage = %d, want reset to zero", used)
	}

	events, err := audits.List(ctx, "tenant-a")
	if err != nil {
		t.Fatalf("audits.List() error = %v", err)
	}
	if len(events) != 1 || events[0].Action != "tenant.onboard" || events[0].Metadata["plan_id"] != "starter" || events[0].Metadata["source"] != "signup" {
		t.Fatalf("audit events = %+v, want onboarding audit metadata", events)
	}

	messages := notifier.Messages()
	if len(messages) != 1 || messages[0].TenantID != "tenant-a" || messages[0].To != "owner@example.com" {
		t.Fatalf("messages = %+v, want tenant welcome message", messages)
	}
}

func TestServiceOnboardValidation(t *testing.T) {
	ctx := context.Background()
	plans := saasplan.NewMemoryService()
	subscriptions := saassubscription.NewMemoryService()
	service := New(saastenant.New(store.NewMemoryStore()), plans, subscriptions)

	if _, err := service.Onboard(ctx, Input{}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Onboard(empty) error = %v, want ErrInvalidInput", err)
	}
	if _, err := service.Onboard(ctx, Input{Tenant: saastenant.CreateInput{Name: "Tenant", PlanID: "missing"}}); !errors.Is(err, saasplan.ErrPlanNotFound) {
		t.Fatalf("Onboard(missing plan) error = %v, want ErrPlanNotFound", err)
	}

	backing := store.NewMemoryStore()
	periodEnd := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	service = New(saastenant.New(backing), plans, basicSubscriptionService{})
	_, err := service.Onboard(ctx, Input{
		Tenant: saastenant.CreateInput{
			ID:     "tenant-a",
			Name:   "Tenant A",
			PlanID: "starter",
		},
		SubscriptionPeriodEnd: &periodEnd,
	})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("Onboard(period unsupported) error = %v, want ErrInvalidInput", err)
	}
	if _, err := backing.Get(ctx, "tenant-a"); !errors.Is(err, store.ErrTenantNotFound) {
		t.Fatalf("tenant created before validation, Get() error = %v, want ErrTenantNotFound", err)
	}
}

type basicSubscriptionService struct{}

func (basicSubscriptionService) Subscribe(context.Context, types.TenantID, string) (saassubscription.Subscription, error) {
	return saassubscription.Subscription{}, nil
}

func (basicSubscriptionService) Unsubscribe(context.Context, types.TenantID) (saassubscription.Subscription, error) {
	return saassubscription.Subscription{}, nil
}

func (basicSubscriptionService) Upgrade(context.Context, types.TenantID, string) (saassubscription.Subscription, error) {
	return saassubscription.Subscription{}, nil
}

func (basicSubscriptionService) Downgrade(context.Context, types.TenantID, string) (saassubscription.Subscription, error) {
	return saassubscription.Subscription{}, nil
}

func (basicSubscriptionService) Get(context.Context, types.TenantID) (saassubscription.Subscription, error) {
	return saassubscription.Subscription{}, saassubscription.ErrSubscriptionNotFound
}
