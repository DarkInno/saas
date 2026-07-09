package subscription

import (
	"time"

	"github.com/DarkInno/gotenancy/core/types"
)

// Subscription binds a tenant to a plan.
type Subscription struct {
	TenantID         types.TenantID
	PlanID           string
	Status           Status
	StartDate        time.Time
	EndDate          *time.Time
	CurrentPeriodEnd *time.Time
	GracePeriodEnd   *time.Time
}
