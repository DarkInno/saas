package audit

import (
	"time"

	"github.com/DarkInno/saas/core/types"
)

type Event struct {
	ID        string
	TenantID  types.TenantID
	ActorID   string
	Action    string
	Resource  string
	CreatedAt time.Time
	Metadata  map[string]string
}
