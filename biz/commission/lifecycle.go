package commission

func nextTemplateStatus(status TemplateStatus, action TemplateAction) (TemplateStatus, error) {
	switch action {
	case TemplateActionActivate:
		if status == TemplateStatusDraft {
			return TemplateStatusActive, nil
		}
	case TemplateActionRetire:
		if status == TemplateStatusActive {
			return TemplateStatusRetired, nil
		}
	}
	return "", ErrInvalidTransition
}

func nextProgramStatus(status ProgramStatus, action ProgramAction) (ProgramStatus, error) {
	switch action {
	case ProgramActionSubmit:
		if status == ProgramStatusDraft {
			return ProgramStatusPendingApproval, nil
		}
	case ProgramActionApprove:
		if status == ProgramStatusPendingApproval {
			return ProgramStatusActive, nil
		}
	case ProgramActionSuspend:
		if status == ProgramStatusActive {
			return ProgramStatusSuspended, nil
		}
	case ProgramActionResume:
		if status == ProgramStatusSuspended {
			return ProgramStatusActive, nil
		}
	case ProgramActionRetire:
		switch status {
		case ProgramStatusDraft, ProgramStatusPendingApproval, ProgramStatusActive, ProgramStatusSuspended:
			return ProgramStatusRetired, nil
		}
	}
	return "", ErrInvalidTransition
}

// AvailableEarningActions reports state-valid actions. Authorization is kept
// separate so host applications can filter this list for the current actor.
// It includes system and aggregate-owned actions; callers that expose only the
// Service.TransitionEarning command should use AvailableManualEarningActions.
func AvailableEarningActions(status EarningStatus) []EarningAction {
	switch status {
	case EarningStatusPending:
		return []EarningAction{EarningActionMakeAvailable, EarningActionHold}
	case EarningStatusAvailable:
		return []EarningAction{EarningActionHold, EarningActionStartSettlement, EarningActionReverse}
	case EarningStatusHeld:
		return []EarningAction{EarningActionRelease, EarningActionReverse}
	case EarningStatusSettling:
		return []EarningAction{EarningActionSettle, EarningActionRejectSettlement}
	case EarningStatusSettled:
		return []EarningAction{EarningActionReverse}
	default:
		return []EarningAction{}
	}
}

// AvailableManualEarningActions reports the state-valid actions accepted by
// Service.TransitionEarning. Time-based release is performed by
// Service.MakeAvailableDue, and settlement actions belong to the settlement
// aggregate methods so they cannot be applied to an earning in isolation.
func AvailableManualEarningActions(status EarningStatus) []EarningAction {
	switch status {
	case EarningStatusPending:
		return []EarningAction{EarningActionHold}
	case EarningStatusAvailable:
		return []EarningAction{EarningActionHold, EarningActionReverse}
	case EarningStatusHeld:
		return []EarningAction{EarningActionRelease, EarningActionReverse}
	case EarningStatusSettled:
		return []EarningAction{EarningActionReverse}
	default:
		return []EarningAction{}
	}
}
