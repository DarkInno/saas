package commission

import "errors"

var (
	// ErrInvalidAmount reports an amount with an invalid currency or minor-unit value.
	ErrInvalidAmount = errors.New("saas/commission: invalid amount")

	// ErrAmountOverflow reports a commission calculation that cannot fit in int64 minor units.
	ErrAmountOverflow = errors.New("saas/commission: amount overflow")

	// ErrInvalidCommissionEvent reports invalid commissionable source-event data.
	ErrInvalidCommissionEvent = errors.New("saas/commission: invalid commission event")

	// ErrLimitExceeded reports a request that exceeds the configured Service
	// resource bounds. It is deliberately distinct from malformed input so a
	// host can map it to a bounded-request response without retrying it.
	ErrLimitExceeded = errors.New("saas/commission: limit exceeded")

	// ErrInvalidBeneficiary reports an invalid commission beneficiary reference.
	ErrInvalidBeneficiary = errors.New("saas/commission: invalid beneficiary")

	// ErrInvalidTier reports an invalid commission tier.
	ErrInvalidTier = errors.New("saas/commission: invalid tier")

	// ErrInvalidRule reports an invalid commission rule.
	ErrInvalidRule = errors.New("saas/commission: invalid rule")

	// ErrInvalidTemplate reports an invalid commission template.
	ErrInvalidTemplate = errors.New("saas/commission: invalid template")

	// ErrEventTypeDisabled reports an event type not enabled by a template.
	ErrEventTypeDisabled = errors.New("saas/commission: event type disabled")

	// ErrCurrencyMismatch reports incompatible commissionable and cap currencies.
	ErrCurrencyMismatch = errors.New("saas/commission: currency mismatch")

	// ErrCommissionCapExceeded reports a calculation above the configured template cap.
	ErrCommissionCapExceeded = errors.New("saas/commission: commission cap exceeded")

	// ErrInvalidEarningTransition reports an unsupported earning status transition.
	ErrInvalidEarningTransition = errors.New("saas/commission: invalid earning transition")

	// ErrNilStore reports a Service without a persistence store.
	ErrNilStore = errors.New("saas/commission: nil store")

	// ErrNilDB reports a SQLStore constructed without a database handle.
	ErrNilDB = errors.New("saas/commission: nil db")

	// ErrInvalidTableName reports an unsafe or incomplete SQL table-name option.
	ErrInvalidTableName = errors.New("saas/commission: invalid table name")

	// ErrUnsupportedSQLDialect reports a dialect without supported placeholders.
	ErrUnsupportedSQLDialect = errors.New("saas/commission: unsupported sql dialect")

	// ErrUnauthorized reports an actor operating outside its tenant boundary.
	ErrUnauthorized = errors.New("saas/commission: unauthorized")

	// ErrInvalidTransition reports an unsupported template or program lifecycle change.
	ErrInvalidTransition = errors.New("saas/commission: invalid lifecycle transition")

	ErrTemplateAlreadyExists = errors.New("saas/commission: template already exists")
	ErrTemplateNotFound      = errors.New("saas/commission: template not found")
	ErrTemplateNotActive     = errors.New("saas/commission: template not active")

	ErrInvalidProgram         = errors.New("saas/commission: invalid program")
	ErrProgramAlreadyExists   = errors.New("saas/commission: program already exists")
	ErrProgramNotFound        = errors.New("saas/commission: program not found")
	ErrProgramNotActive       = errors.New("saas/commission: program not active")
	ErrProgramExceedsTemplate = errors.New("saas/commission: program exceeds template")

	ErrInvalidAttribution = errors.New("saas/commission: invalid attribution")
	ErrVersionConflict    = errors.New("saas/commission: version conflict")

	// ErrEventConflict reports a source-event key reused with different facts.
	ErrEventConflict = errors.New("saas/commission: event conflict")
	ErrInvalidEvent  = errors.New("saas/commission: invalid event commit")

	ErrInvalidEarning       = errors.New("saas/commission: invalid earning")
	ErrEarningNotFound      = errors.New("saas/commission: earning not found")
	ErrEarningUnavailable   = errors.New("saas/commission: earning unavailable")
	ErrInvalidEarningFilter = errors.New("saas/commission: invalid earning filter")

	ErrInvalidSettlement              = errors.New("saas/commission: invalid settlement")
	ErrSettlementAlreadyExists        = errors.New("saas/commission: settlement already exists")
	ErrSettlementNotFound             = errors.New("saas/commission: settlement not found")
	ErrSettlementOutcomeConflict      = errors.New("saas/commission: settlement outcome conflict")
	ErrSettlementAdapterNotConfigured = errors.New("saas/commission: settlement adapter not configured")
	ErrInvalidSettlementReceipt       = errors.New("saas/commission: invalid settlement receipt")

	ErrInvalidOutboxFilter = errors.New("saas/commission: invalid outbox filter")
	ErrInvalidOutbox       = errors.New("saas/commission: invalid outbox event")
	ErrOutboxNotFound      = errors.New("saas/commission: outbox event not found")
)
