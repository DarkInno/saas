package subscription

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/DarkInno/gotenancy/core/types"
)

var _ LifecycleService = (*MemoryService)(nil)
var _ Store = (*MemoryService)(nil)
var _ PagedStore = (*MemoryService)(nil)

// MemoryStore is kept as a Store-oriented name for MemoryService.
type MemoryStore = MemoryService

// Option configures MemoryService.
type Option func(*MemoryService)

// WithClock sets the clock used for subscription dates.
func WithClock(clock func() time.Time) Option {
	return func(service *MemoryService) {
		if clock != nil {
			service.now = clock
		}
	}
}

// WithBillingHook sets the billing hook.
func WithBillingHook(hook BillingHook) Option {
	return func(service *MemoryService) {
		service.billing = hook
	}
}

// WithGracePeriod sets the post-period grace window before ExpireDue marks a subscription expired.
func WithGracePeriod(period time.Duration) Option {
	return func(service *MemoryService) {
		if period > 0 {
			service.gracePeriod = period
		}
	}
}

// MemoryService is a thread-safe in-memory subscription service.
type MemoryService struct {
	mu            sync.RWMutex
	now           func() time.Time
	gracePeriod   time.Duration
	billing       BillingHook
	subscriptions map[types.TenantID]Subscription
}

// NewMemoryService creates an empty subscription service.
func NewMemoryService(opts ...Option) *MemoryService {
	service := &MemoryService{
		now:           time.Now,
		subscriptions: map[types.TenantID]Subscription{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(service)
		}
	}
	return service
}

// NewMemoryStore creates an empty subscription store.
func NewMemoryStore(opts ...Option) *MemoryStore {
	return NewMemoryService(opts...)
}

// Create inserts a subscription.
func (service *MemoryService) Create(ctx context.Context, subscription Subscription) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateSubscription(subscription); err != nil {
		return err
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	if _, ok := service.subscriptions[subscription.TenantID]; ok {
		return ErrSubscriptionAlreadyExists
	}
	service.subscriptions[subscription.TenantID] = cloneSubscription(subscription)
	return nil
}

// Subscribe creates an active subscription for a tenant.
func (service *MemoryService) Subscribe(ctx context.Context, tenantID types.TenantID, planID string) (Subscription, error) {
	return service.subscribe(ctx, tenantID, planID, nil)
}

// SubscribeWithPeriod creates an active subscription with a current billing period end.
func (service *MemoryService) SubscribeWithPeriod(ctx context.Context, tenantID types.TenantID, planID string, currentPeriodEnd time.Time) (Subscription, error) {
	if currentPeriodEnd.IsZero() {
		return Subscription{}, ErrInvalidSubscription
	}
	return service.subscribe(ctx, tenantID, planID, &currentPeriodEnd)
}

func (service *MemoryService) subscribe(ctx context.Context, tenantID types.TenantID, planID string, currentPeriodEnd *time.Time) (Subscription, error) {
	if err := ctx.Err(); err != nil {
		return Subscription{}, err
	}
	if tenantID == "" || planID == "" {
		return Subscription{}, ErrInvalidSubscription
	}

	service.mu.Lock()
	if _, ok := service.subscriptions[tenantID]; ok {
		service.mu.Unlock()
		return Subscription{}, ErrSubscriptionAlreadyExists
	}

	subscription := Subscription{
		TenantID:  tenantID,
		PlanID:    planID,
		Status:    StatusActive,
		StartDate: service.now(),
	}
	if currentPeriodEnd != nil {
		service.setPeriod(&subscription, *currentPeriodEnd)
	}
	service.subscriptions[tenantID] = cloneSubscription(subscription)
	service.mu.Unlock()

	if err := service.emit(ctx, BillingEvent{TenantID: tenantID, Action: "subscribe", ToPlan: planID, Status: subscription.Status, CurrentPeriodEnd: cloneTimePtr(subscription.CurrentPeriodEnd)}); err != nil {
		return Subscription{}, err
	}
	return subscription, nil
}

// Unsubscribe cancels an active subscription.
func (service *MemoryService) Unsubscribe(ctx context.Context, tenantID types.TenantID) (Subscription, error) {
	if err := ctx.Err(); err != nil {
		return Subscription{}, err
	}

	service.mu.Lock()
	current, ok := service.subscriptions[tenantID]
	if !ok {
		service.mu.Unlock()
		return Subscription{}, ErrSubscriptionNotFound
	}
	if current.Status != StatusActive {
		service.mu.Unlock()
		return Subscription{}, ErrInvalidTransition
	}
	now := service.now()
	current.Status = StatusCancelled
	current.EndDate = &now
	service.subscriptions[tenantID] = cloneSubscription(current)
	service.mu.Unlock()

	if err := service.emit(ctx, BillingEvent{TenantID: tenantID, Action: "unsubscribe", FromPlan: current.PlanID, Status: current.Status}); err != nil {
		return Subscription{}, err
	}
	return cloneSubscription(current), nil
}

// Renew reactivates an active or expired subscription with a new current period end.
func (service *MemoryService) Renew(ctx context.Context, tenantID types.TenantID, currentPeriodEnd time.Time) (Subscription, error) {
	if err := ctx.Err(); err != nil {
		return Subscription{}, err
	}
	if tenantID == "" || currentPeriodEnd.IsZero() {
		return Subscription{}, ErrInvalidSubscription
	}

	service.mu.Lock()
	current, ok := service.subscriptions[tenantID]
	if !ok {
		service.mu.Unlock()
		return Subscription{}, ErrSubscriptionNotFound
	}
	if current.Status == StatusCancelled {
		service.mu.Unlock()
		return Subscription{}, ErrInvalidTransition
	}
	current.Status = StatusActive
	current.EndDate = nil
	service.setPeriod(&current, currentPeriodEnd)
	service.subscriptions[tenantID] = cloneSubscription(current)
	service.mu.Unlock()

	if err := service.emit(ctx, BillingEvent{TenantID: tenantID, Action: "renew", ToPlan: current.PlanID, Status: current.Status, CurrentPeriodEnd: cloneTimePtr(current.CurrentPeriodEnd)}); err != nil {
		return Subscription{}, err
	}
	return cloneSubscription(current), nil
}

// Expire marks an active subscription expired immediately.
func (service *MemoryService) Expire(ctx context.Context, tenantID types.TenantID) (Subscription, error) {
	if err := ctx.Err(); err != nil {
		return Subscription{}, err
	}
	if tenantID == "" {
		return Subscription{}, ErrInvalidSubscription
	}

	service.mu.Lock()
	current, ok := service.subscriptions[tenantID]
	if !ok {
		service.mu.Unlock()
		return Subscription{}, ErrSubscriptionNotFound
	}
	if current.Status != StatusActive {
		service.mu.Unlock()
		return Subscription{}, ErrInvalidTransition
	}
	now := service.now()
	current.Status = StatusExpired
	current.EndDate = &now
	service.subscriptions[tenantID] = cloneSubscription(current)
	service.mu.Unlock()

	if err := service.emit(ctx, BillingEvent{TenantID: tenantID, Action: "expire", FromPlan: current.PlanID, Status: current.Status, CurrentPeriodEnd: cloneTimePtr(current.CurrentPeriodEnd)}); err != nil {
		return Subscription{}, err
	}
	return cloneSubscription(current), nil
}

// ExpireDue marks active subscriptions expired after their current period and grace window end.
func (service *MemoryService) ExpireDue(ctx context.Context) ([]Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	service.mu.Lock()

	now := service.now()
	expired := []Subscription{}
	events := []BillingEvent{}
	for tenantID, current := range service.subscriptions {
		if !subscriptionDue(current, now) {
			continue
		}

		next := current
		next.Status = StatusExpired
		endDate := expirationDate(next)
		next.EndDate = &endDate
		service.subscriptions[tenantID] = cloneSubscription(next)
		expired = append(expired, cloneSubscription(next))
		events = append(events, BillingEvent{TenantID: tenantID, Action: "expire", FromPlan: next.PlanID, Status: next.Status, CurrentPeriodEnd: cloneTimePtr(next.CurrentPeriodEnd)})
	}
	service.mu.Unlock()

	for _, event := range events {
		if err := service.emit(ctx, event); err != nil {
			return expired, err
		}
	}
	return expired, nil
}

// Upgrade changes an active subscription to a higher plan.
func (service *MemoryService) Upgrade(ctx context.Context, tenantID types.TenantID, planID string) (Subscription, error) {
	return service.changePlan(ctx, tenantID, planID, "upgrade")
}

// Downgrade changes an active subscription to a lower plan.
func (service *MemoryService) Downgrade(ctx context.Context, tenantID types.TenantID, planID string) (Subscription, error) {
	return service.changePlan(ctx, tenantID, planID, "downgrade")
}

// Get returns a subscription by tenant ID.
func (service *MemoryService) Get(ctx context.Context, tenantID types.TenantID) (Subscription, error) {
	if err := ctx.Err(); err != nil {
		return Subscription{}, err
	}
	if tenantID == "" {
		return Subscription{}, ErrInvalidSubscription
	}

	service.mu.RLock()
	defer service.mu.RUnlock()

	subscription, ok := service.subscriptions[tenantID]
	if !ok {
		return Subscription{}, ErrSubscriptionNotFound
	}
	return cloneSubscription(subscription), nil
}

// List returns subscriptions matching filter.
func (service *MemoryService) List(ctx context.Context, filter ListFilter) ([]Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := filter.validate(); err != nil {
		return nil, err
	}
	return service.list(filter, "")
}

// ListPage returns subscriptions after the cursor while preserving List filtering semantics.
func (service *MemoryService) ListPage(ctx context.Context, filter PageFilter) ([]Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := filter.validate(); err != nil {
		return nil, err
	}
	return service.list(filter.listFilter(), filter.Cursor)
}

func (service *MemoryService) list(filter ListFilter, cursor types.TenantID) ([]Subscription, error) {
	service.mu.RLock()
	defer service.mu.RUnlock()

	subscriptions := make([]Subscription, 0, len(service.subscriptions))
	for _, subscription := range service.subscriptions {
		if filter.matches(subscription) {
			subscriptions = append(subscriptions, cloneSubscription(subscription))
		}
	}
	sort.Slice(subscriptions, func(i, j int) bool {
		return subscriptions[i].TenantID < subscriptions[j].TenantID
	})
	subscriptions = seekSubscriptions(subscriptions, cursor)
	return pageSubscriptions(subscriptions, filter), nil
}

// Update replaces a subscription.
func (service *MemoryService) Update(ctx context.Context, subscription Subscription) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateSubscription(subscription); err != nil {
		return err
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	if _, ok := service.subscriptions[subscription.TenantID]; !ok {
		return ErrSubscriptionNotFound
	}
	service.subscriptions[subscription.TenantID] = cloneSubscription(subscription)
	return nil
}

// Delete removes a subscription by tenant ID.
func (service *MemoryService) Delete(ctx context.Context, tenantID types.TenantID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if tenantID == "" {
		return ErrInvalidSubscription
	}

	service.mu.Lock()
	defer service.mu.Unlock()

	if _, ok := service.subscriptions[tenantID]; !ok {
		return ErrSubscriptionNotFound
	}
	delete(service.subscriptions, tenantID)
	return nil
}

func (service *MemoryService) changePlan(ctx context.Context, tenantID types.TenantID, planID string, action string) (Subscription, error) {
	if err := ctx.Err(); err != nil {
		return Subscription{}, err
	}
	if tenantID == "" || planID == "" {
		return Subscription{}, ErrInvalidSubscription
	}

	service.mu.Lock()
	current, ok := service.subscriptions[tenantID]
	if !ok {
		service.mu.Unlock()
		return Subscription{}, ErrSubscriptionNotFound
	}
	if current.Status != StatusActive {
		service.mu.Unlock()
		return Subscription{}, ErrInvalidTransition
	}
	fromPlan := current.PlanID
	current.PlanID = planID
	service.subscriptions[tenantID] = cloneSubscription(current)
	service.mu.Unlock()

	if err := service.emit(ctx, BillingEvent{TenantID: tenantID, Action: action, FromPlan: fromPlan, ToPlan: planID, Status: current.Status}); err != nil {
		return Subscription{}, err
	}
	return cloneSubscription(current), nil
}

func (service *MemoryService) emit(ctx context.Context, event BillingEvent) error {
	if service.billing == nil {
		return nil
	}
	return service.billing(ctx, event)
}

func (service *MemoryService) setPeriod(subscription *Subscription, currentPeriodEnd time.Time) {
	end := currentPeriodEnd
	subscription.CurrentPeriodEnd = &end
	if service.gracePeriod > 0 {
		graceEnd := end.Add(service.gracePeriod)
		subscription.GracePeriodEnd = &graceEnd
		return
	}
	subscription.GracePeriodEnd = nil
}

func subscriptionDue(subscription Subscription, now time.Time) bool {
	if subscription.Status != StatusActive || subscription.CurrentPeriodEnd == nil {
		return false
	}
	dueAt := expirationDate(subscription)
	return !now.Before(dueAt)
}

func expirationDate(subscription Subscription) time.Time {
	if subscription.GracePeriodEnd != nil {
		return *subscription.GracePeriodEnd
	}
	return *subscription.CurrentPeriodEnd
}

func cloneSubscription(subscription Subscription) Subscription {
	subscription.CurrentPeriodEnd = cloneTimePtr(subscription.CurrentPeriodEnd)
	subscription.GracePeriodEnd = cloneTimePtr(subscription.GracePeriodEnd)
	subscription.EndDate = cloneTimePtr(subscription.EndDate)
	return subscription
}

func cloneTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func validateSubscription(subscription Subscription) error {
	if subscription.TenantID == "" || subscription.PlanID == "" || !validStatus(subscription.Status) || subscription.StartDate.IsZero() {
		return ErrInvalidSubscription
	}
	return nil
}
