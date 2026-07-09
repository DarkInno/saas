package onboarding

import (
	"context"
	"time"

	"github.com/DarkInno/gotenancy/biz/audit"
	"github.com/DarkInno/gotenancy/biz/notification"
	"github.com/DarkInno/gotenancy/core/types"
	saasfeature "github.com/DarkInno/gotenancy/saas/feature"
	saasplan "github.com/DarkInno/gotenancy/saas/plan"
	saasquota "github.com/DarkInno/gotenancy/saas/quota"
	saassubscription "github.com/DarkInno/gotenancy/saas/subscription"
	saastenant "github.com/DarkInno/gotenancy/saas/tenant"
)

// Service coordinates the default tenant onboarding flow.
type Service struct {
	tenants       saastenant.Service
	plans         saasplan.Service
	subscriptions saassubscription.Service
	features      saasfeature.Store
	quotas        saasquota.Store
	audit         audit.Store
	notifier      notification.Notifier
}

// Option configures onboarding integrations.
type Option func(*Service)

// WithFeatureStore initializes plan defaults and tenant feature overrides.
func WithFeatureStore(store saasfeature.Store) Option {
	return func(service *Service) {
		service.features = store
	}
}

// WithQuotaStore initializes tenant quota usage buckets from the selected plan.
func WithQuotaStore(store saasquota.Store) Option {
	return func(service *Service) {
		service.quotas = store
	}
}

// WithAuditStore records a tenant.onboard event after the flow succeeds.
func WithAuditStore(store audit.Store) Option {
	return func(service *Service) {
		service.audit = store
	}
}

// WithNotifier sends an optional welcome message after the flow succeeds.
func WithNotifier(notifier notification.Notifier) Option {
	return func(service *Service) {
		service.notifier = notifier
	}
}

// New creates an onboarding service.
func New(tenants saastenant.Service, plans saasplan.Service, subscriptions saassubscription.Service, opts ...Option) *Service {
	service := &Service{
		tenants:       tenants,
		plans:         plans,
		subscriptions: subscriptions,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

// Input describes a tenant onboarding request.
type Input struct {
	Tenant                saastenant.CreateInput
	SubscriptionPeriodEnd *time.Time
	FeatureOverrides      []saasfeature.Flag
	Welcome               *notification.Message
	SkipActivation        bool
	AuditMetadata         map[string]string
}

// Result describes the completed onboarding state.
type Result struct {
	Tenant       types.Tenant
	Plan         saasplan.Plan
	Subscription saassubscription.Subscription
	Features     []saasfeature.Flag
	QuotaLimits  []saasquota.Limit
}

// Onboard creates the tenant, attaches the selected plan subscription, initializes
// feature and quota state, records audit metadata, optionally sends a welcome
// message, and activates the tenant unless SkipActivation is set.
func (service *Service) Onboard(ctx context.Context, input Input) (Result, error) {
	if err := service.validate(input); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}

	selectedPlan, err := service.plans.Get(ctx, input.Tenant.PlanID)
	if err != nil {
		return Result{}, err
	}

	created, err := service.tenants.Create(ctx, input.Tenant)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Tenant:      created,
		Plan:        selectedPlan,
		QuotaLimits: quotaLimits(created.ID, selectedPlan.Quotas),
	}

	if service.features != nil {
		if err := service.features.SetPlanDefaults(ctx, selectedPlan.ID, planFeatureFlags(selectedPlan.Features)); err != nil {
			return Result{}, err
		}
		if len(input.FeatureOverrides) > 0 {
			if err := service.features.SetTenantOverrides(ctx, created.ID, input.FeatureOverrides); err != nil {
				return Result{}, err
			}
		}
		flags, err := service.features.List(ctx, created.ID, selectedPlan.ID)
		if err != nil {
			return Result{}, err
		}
		result.Features = flags
	}

	if service.quotas != nil {
		for _, limit := range result.QuotaLimits {
			if err := service.quotas.Reset(ctx, limit.TenantID, limit.Resource, limit.Period); err != nil {
				return Result{}, err
			}
		}
	}

	subscription, err := service.subscribe(ctx, created.ID, selectedPlan.ID, input.SubscriptionPeriodEnd)
	if err != nil {
		return Result{}, err
	}
	result.Subscription = subscription

	if !input.SkipActivation {
		activated, err := service.tenants.Activate(ctx, created.ID)
		if err != nil {
			return Result{}, err
		}
		result.Tenant = activated
	}

	if service.audit != nil {
		if err := service.audit.Record(ctx, audit.Event{
			TenantID: created.ID,
			Action:   "tenant.onboard",
			Resource: "tenant:" + created.ID.String(),
			Metadata: auditMetadata(selectedPlan.ID, result.Tenant.Status, input.AuditMetadata),
		}); err != nil {
			return Result{}, err
		}
	}

	if input.Welcome != nil {
		if service.notifier == nil {
			return Result{}, ErrInvalidInput
		}
		message := cloneWelcome(created.ID, *input.Welcome)
		if err := service.notifier.Send(ctx, message); err != nil {
			return Result{}, err
		}
	}

	return result, nil
}

func (service *Service) validate(input Input) error {
	if service == nil || service.tenants == nil || service.plans == nil || service.subscriptions == nil {
		return ErrInvalidInput
	}
	if input.Tenant.PlanID == "" {
		return ErrInvalidInput
	}
	if input.SubscriptionPeriodEnd != nil && input.SubscriptionPeriodEnd.IsZero() {
		return ErrInvalidInput
	}
	if input.SubscriptionPeriodEnd != nil {
		if _, ok := service.subscriptions.(saassubscription.LifecycleService); !ok {
			return ErrInvalidInput
		}
	}
	if input.Welcome != nil && service.notifier == nil {
		return ErrInvalidInput
	}
	return nil
}

func (service *Service) subscribe(ctx context.Context, tenantID types.TenantID, planID string, periodEnd *time.Time) (saassubscription.Subscription, error) {
	if periodEnd == nil {
		return service.subscriptions.Subscribe(ctx, tenantID, planID)
	}
	lifecycle, ok := service.subscriptions.(saassubscription.LifecycleService)
	if !ok {
		return saassubscription.Subscription{}, ErrInvalidInput
	}
	return lifecycle.SubscribeWithPeriod(ctx, tenantID, planID, *periodEnd)
}

func planFeatureFlags(features []saasplan.Feature) []saasfeature.Flag {
	flags := make([]saasfeature.Flag, len(features))
	for i, feature := range features {
		flags[i] = saasfeature.Flag{
			Key:     feature.Key,
			Enabled: feature.Enabled,
			Config:  cloneStringMap(feature.Config),
		}
	}
	return flags
}

func quotaLimits(tenantID types.TenantID, quotas []saasplan.Quota) []saasquota.Limit {
	limits := make([]saasquota.Limit, len(quotas))
	for i, quota := range quotas {
		limits[i] = saasquota.Limit{
			TenantID: tenantID,
			Resource: quota.Resource,
			Limit:    quota.Limit,
			Period:   quotaPeriod(quota.Period),
		}
	}
	return limits
}

func quotaPeriod(period saasplan.QuotaPeriod) saasquota.Period {
	switch period {
	case saasplan.QuotaPeriodNone:
		return saasquota.PeriodNone
	case saasplan.QuotaPeriodDay:
		return saasquota.PeriodDay
	case saasplan.QuotaPeriodMonth:
		return saasquota.PeriodMonth
	default:
		return saasquota.Period(period)
	}
}

func auditMetadata(planID string, status types.TenantStatus, extra map[string]string) map[string]string {
	metadata := cloneStringMap(extra)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["plan_id"] = planID
	metadata["tenant_status"] = string(status)
	return metadata
}

func cloneWelcome(tenantID types.TenantID, message notification.Message) notification.Message {
	message.TenantID = tenantID
	message.Metadata = cloneStringMap(message.Metadata)
	return message
}

func cloneStringMap(values map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
