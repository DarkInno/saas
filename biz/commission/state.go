package commission

// TemplateStatus describes the lifecycle state of a commission template.
type TemplateStatus string

const (
	TemplateStatusDraft   TemplateStatus = "draft"
	TemplateStatusActive  TemplateStatus = "active"
	TemplateStatusRetired TemplateStatus = "retired"
)

func validTemplateStatus(status TemplateStatus) bool {
	switch status {
	case TemplateStatusDraft, TemplateStatusActive, TemplateStatusRetired:
		return true
	default:
		return false
	}
}

// ProgramStatus describes the lifecycle state of a commission program.
type ProgramStatus string

const (
	ProgramStatusDraft           ProgramStatus = "draft"
	ProgramStatusPendingApproval ProgramStatus = "pending_approval"
	ProgramStatusActive          ProgramStatus = "active"
	ProgramStatusSuspended       ProgramStatus = "suspended"
	ProgramStatusRetired         ProgramStatus = "retired"
)

func validProgramStatus(status ProgramStatus) bool {
	switch status {
	case ProgramStatusDraft, ProgramStatusPendingApproval, ProgramStatusActive, ProgramStatusSuspended, ProgramStatusRetired:
		return true
	default:
		return false
	}
}

// EarningStatus describes a calculated commission earning's settlement state.
type EarningStatus string

const (
	EarningStatusPending     EarningStatus = "pending"
	EarningStatusAvailable   EarningStatus = "available"
	EarningStatusHeld        EarningStatus = "held"
	EarningStatusSettling    EarningStatus = "settling"
	EarningStatusSettled     EarningStatus = "settled"
	EarningStatusReversed    EarningStatus = "reversed"
	EarningStatusRecoveryDue EarningStatus = "recovery_due"
)

func validEarningStatus(status EarningStatus) bool {
	switch status {
	case EarningStatusPending,
		EarningStatusAvailable,
		EarningStatusHeld,
		EarningStatusSettling,
		EarningStatusSettled,
		EarningStatusReversed,
		EarningStatusRecoveryDue:
		return true
	default:
		return false
	}
}

// EarningAction requests a state change for an earning.
type EarningAction string

const (
	EarningActionMakeAvailable    EarningAction = "make_available"
	EarningActionHold             EarningAction = "hold"
	EarningActionRelease          EarningAction = "release"
	EarningActionStartSettlement  EarningAction = "start_settlement"
	EarningActionSettle           EarningAction = "settle"
	EarningActionRejectSettlement EarningAction = "reject_settlement"
	EarningActionReverse          EarningAction = "reverse"
)

func validEarningAction(action EarningAction) bool {
	switch action {
	case EarningActionMakeAvailable,
		EarningActionHold,
		EarningActionRelease,
		EarningActionStartSettlement,
		EarningActionSettle,
		EarningActionRejectSettlement,
		EarningActionReverse:
		return true
	default:
		return false
	}
}

// TransitionEarning returns the state reached by applying action to status.
func TransitionEarning(status EarningStatus, action EarningAction) (EarningStatus, error) {
	switch status {
	case EarningStatusPending:
		switch action {
		case EarningActionMakeAvailable:
			return EarningStatusAvailable, nil
		case EarningActionHold:
			return EarningStatusHeld, nil
		}
	case EarningStatusAvailable:
		switch action {
		case EarningActionHold:
			return EarningStatusHeld, nil
		case EarningActionStartSettlement:
			return EarningStatusSettling, nil
		case EarningActionReverse:
			return EarningStatusReversed, nil
		}
	case EarningStatusHeld:
		switch action {
		case EarningActionRelease:
			return EarningStatusAvailable, nil
		case EarningActionReverse:
			return EarningStatusReversed, nil
		}
	case EarningStatusSettling:
		switch action {
		case EarningActionSettle:
			return EarningStatusSettled, nil
		case EarningActionRejectSettlement:
			return EarningStatusAvailable, nil
		}
	case EarningStatusSettled:
		if action == EarningActionReverse {
			return EarningStatusRecoveryDue, nil
		}
	}
	return "", ErrInvalidEarningTransition
}
