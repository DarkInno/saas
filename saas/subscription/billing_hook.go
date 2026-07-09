package subscription

import (
	"context"
	"time"

	"github.com/DarkInno/gotenancy/core/types"
)

// BillingEvent describes a subscription change for external billing systems.
type BillingEvent struct {
	TenantID         types.TenantID
	Action           string
	FromPlan         string
	ToPlan           string
	Status           Status
	CurrentPeriodEnd *time.Time
}

// BillingHook receives subscription lifecycle changes.
type BillingHook func(context.Context, BillingEvent) error
