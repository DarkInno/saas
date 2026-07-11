package subscription

import "errors"

var (
	// ErrSubscriptionNotFound reports that a subscription does not exist.
	ErrSubscriptionNotFound = errors.New("gotenancy/subscription: subscription not found")

	// ErrSubscriptionAlreadyExists reports that a tenant already has a subscription.
	ErrSubscriptionAlreadyExists = errors.New("gotenancy/subscription: subscription already exists")

	// ErrSubscriptionConflict reports a concurrent replacement during update.
	ErrSubscriptionConflict = errors.New("gotenancy/subscription: subscription update conflict")

	// ErrInvalidSubscription reports invalid subscription metadata.
	ErrInvalidSubscription = errors.New("gotenancy/subscription: invalid subscription")

	// ErrInvalidTransition reports an invalid subscription lifecycle transition.
	ErrInvalidTransition = errors.New("gotenancy/subscription: invalid transition")

	// ErrBillingHookReentrantMutation reports a billing hook attempting to
	// mutate a subscription that already has an in-flight billing event.
	ErrBillingHookReentrantMutation = errors.New("gotenancy/subscription: billing hook cannot mutate pending subscription")

	// ErrInvalidListFilter reports an invalid subscription list filter.
	ErrInvalidListFilter = errors.New("gotenancy/subscription: invalid list filter")

	// ErrNilDB reports that a SQL store was created with a nil database handle.
	ErrNilDB = errors.New("gotenancy/subscription: nil db")

	// ErrInvalidTableName reports an unsafe SQL table name.
	ErrInvalidTableName = errors.New("gotenancy/subscription: invalid table name")

	// ErrUnsupportedSQLDialect reports an unsupported SQLStore dialect.
	ErrUnsupportedSQLDialect = errors.New("gotenancy/subscription: unsupported sql dialect")
)
