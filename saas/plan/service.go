package plan

import "context"

// Service manages SaaS plans.
type Service interface {
	Create(ctx context.Context, plan Plan) error
	Get(ctx context.Context, id string) (Plan, error)
	Update(ctx context.Context, plan Plan) error
	Delete(ctx context.Context, id string) error
}

// Store persists SaaS plans.
type Store interface {
	Service
	List(ctx context.Context, filter ListFilter) ([]Plan, error)
}

// ListFilter restricts plan list queries.
type ListFilter struct {
	IDs    []string
	Limit  int
	Offset int
}

func (filter ListFilter) matches(plan Plan) bool {
	if len(filter.IDs) == 0 {
		return true
	}
	for _, id := range filter.IDs {
		if plan.ID == id {
			return true
		}
	}
	return false
}

func (filter ListFilter) validate() error {
	if filter.Limit < 0 || filter.Offset < 0 {
		return ErrInvalidListFilter
	}
	if filter.Offset > 0 && filter.Limit == 0 {
		return ErrInvalidListFilter
	}
	for _, id := range filter.IDs {
		if id == "" {
			return ErrInvalidListFilter
		}
	}
	return nil
}

func pagePlans(plans []Plan, filter ListFilter) []Plan {
	if filter.Offset >= len(plans) {
		return []Plan{}
	}

	start := filter.Offset
	end := len(plans)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}
	return plans[start:end]
}
