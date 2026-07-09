package subscription

import (
	"context"
	"time"

	"github.com/DarkInno/gotenancy/core/types"
)

// Service manages tenant subscriptions.
type Service interface {
	Subscribe(ctx context.Context, tenantID types.TenantID, planID string) (Subscription, error)
	Unsubscribe(ctx context.Context, tenantID types.TenantID) (Subscription, error)
	Upgrade(ctx context.Context, tenantID types.TenantID, planID string) (Subscription, error)
	Downgrade(ctx context.Context, tenantID types.TenantID, planID string) (Subscription, error)
	Get(ctx context.Context, tenantID types.TenantID) (Subscription, error)
}

// LifecycleService extends Service with billing-period renewal and expiration operations.
type LifecycleService interface {
	Service
	SubscribeWithPeriod(ctx context.Context, tenantID types.TenantID, planID string, currentPeriodEnd time.Time) (Subscription, error)
	Renew(ctx context.Context, tenantID types.TenantID, currentPeriodEnd time.Time) (Subscription, error)
	Expire(ctx context.Context, tenantID types.TenantID) (Subscription, error)
	ExpireDue(ctx context.Context) ([]Subscription, error)
}
