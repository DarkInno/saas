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

// PagedStore extends Store with cursor-based plan listing.
type PagedStore interface {
	Store
	ListPage(ctx context.Context, filter PageFilter) ([]Plan, error)
}

// ListFilter restricts plan list queries.
type ListFilter struct {
	IDs    []string
	Limit  int
	Offset int
}

// PageFilter restricts cursor-based plan list queries.
type PageFilter struct {
	IDs    []string
	Limit  int
	Offset int
	// Cursor returns rows ordered after the plan ID cursor.
	Cursor string
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

func (filter PageFilter) validate() error {
	if err := filter.listFilter().validate(); err != nil {
		return err
	}
	if filter.Cursor != "" && filter.Offset > 0 {
		return ErrInvalidListFilter
	}
	return nil
}

func (filter PageFilter) listFilter() ListFilter {
	return ListFilter{
		IDs:    filter.IDs,
		Limit:  filter.Limit,
		Offset: filter.Offset,
	}
}

func pagePlans(plans []Plan, filter ListFilter) []Plan {
	if filter.Offset >= len(plans) {
		return []Plan{}
	}

	start := filter.Offset
	end := len(plans)
	if filter.Limit > 0 && filter.Limit < end-start {
		end = start + filter.Limit
	}
	return plans[start:end]
}

func seekPlans(plans []Plan, cursor string) []Plan {
	if cursor == "" {
		return plans
	}
	start := len(plans)
	for i, plan := range plans {
		if plan.ID > cursor {
			start = i
			break
		}
	}
	return plans[start:]
}
