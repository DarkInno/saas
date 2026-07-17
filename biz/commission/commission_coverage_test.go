package commission

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestCommissionCoverageLifecycleCatalogs(t *testing.T) {
	t.Parallel()

	for _, status := range []TemplateStatus{TemplateStatusDraft, TemplateStatusActive, TemplateStatusRetired} {
		if !validTemplateStatus(status) {
			t.Fatalf("validTemplateStatus(%q) = false", status)
		}
	}
	if validTemplateStatus("unknown") {
		t.Fatal("validTemplateStatus(unknown) = true")
	}
	for _, status := range []ProgramStatus{ProgramStatusDraft, ProgramStatusPendingApproval, ProgramStatusActive, ProgramStatusSuspended, ProgramStatusRetired} {
		if !validProgramStatus(status) {
			t.Fatalf("validProgramStatus(%q) = false", status)
		}
	}
	if validProgramStatus("unknown") {
		t.Fatal("validProgramStatus(unknown) = true")
	}
	for _, status := range []EarningStatus{EarningStatusPending, EarningStatusAvailable, EarningStatusHeld, EarningStatusSettling, EarningStatusSettled, EarningStatusReversed, EarningStatusRecoveryDue} {
		if !validEarningStatus(status) {
			t.Fatalf("validEarningStatus(%q) = false", status)
		}
	}
	if validEarningStatus("unknown") {
		t.Fatal("validEarningStatus(unknown) = true")
	}

	templateTransitions := []struct {
		status TemplateStatus
		action TemplateAction
		want   TemplateStatus
	}{
		{TemplateStatusDraft, TemplateActionActivate, TemplateStatusActive},
		{TemplateStatusActive, TemplateActionRetire, TemplateStatusRetired},
	}
	for _, tt := range templateTransitions {
		got, err := nextTemplateStatus(tt.status, tt.action)
		if err != nil || got != tt.want {
			t.Fatalf("nextTemplateStatus(%q, %q) = %q, %v; want %q, nil", tt.status, tt.action, got, err, tt.want)
		}
	}
	if _, err := nextTemplateStatus(TemplateStatusDraft, TemplateActionRetire); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("nextTemplateStatus(invalid) error = %v, want ErrInvalidTransition", err)
	}

	programTransitions := []struct {
		status ProgramStatus
		action ProgramAction
		want   ProgramStatus
	}{
		{ProgramStatusDraft, ProgramActionSubmit, ProgramStatusPendingApproval},
		{ProgramStatusPendingApproval, ProgramActionApprove, ProgramStatusActive},
		{ProgramStatusActive, ProgramActionSuspend, ProgramStatusSuspended},
		{ProgramStatusSuspended, ProgramActionResume, ProgramStatusActive},
		{ProgramStatusDraft, ProgramActionRetire, ProgramStatusRetired},
		{ProgramStatusPendingApproval, ProgramActionRetire, ProgramStatusRetired},
		{ProgramStatusActive, ProgramActionRetire, ProgramStatusRetired},
		{ProgramStatusSuspended, ProgramActionRetire, ProgramStatusRetired},
	}
	for _, tt := range programTransitions {
		got, err := nextProgramStatus(tt.status, tt.action)
		if err != nil || got != tt.want {
			t.Fatalf("nextProgramStatus(%q, %q) = %q, %v; want %q, nil", tt.status, tt.action, got, err, tt.want)
		}
	}
	if _, err := nextProgramStatus(ProgramStatusRetired, ProgramActionResume); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("nextProgramStatus(invalid) error = %v, want ErrInvalidTransition", err)
	}

	legalEarningTransitions := []struct {
		status EarningStatus
		action EarningAction
		want   EarningStatus
	}{
		{EarningStatusPending, EarningActionMakeAvailable, EarningStatusAvailable},
		{EarningStatusPending, EarningActionHold, EarningStatusHeld},
		{EarningStatusAvailable, EarningActionHold, EarningStatusHeld},
		{EarningStatusAvailable, EarningActionStartSettlement, EarningStatusSettling},
		{EarningStatusAvailable, EarningActionReverse, EarningStatusReversed},
		{EarningStatusHeld, EarningActionRelease, EarningStatusAvailable},
		{EarningStatusHeld, EarningActionReverse, EarningStatusReversed},
		{EarningStatusSettling, EarningActionSettle, EarningStatusSettled},
		{EarningStatusSettling, EarningActionRejectSettlement, EarningStatusAvailable},
		{EarningStatusSettled, EarningActionReverse, EarningStatusRecoveryDue},
	}
	for _, tt := range legalEarningTransitions {
		got, err := TransitionEarning(tt.status, tt.action)
		if err != nil || got != tt.want {
			t.Fatalf("TransitionEarning(%q, %q) = %q, %v; want %q, nil", tt.status, tt.action, got, err, tt.want)
		}
	}
	if _, err := TransitionEarning(EarningStatusRecoveryDue, EarningActionRelease); !errors.Is(err, ErrInvalidEarningTransition) {
		t.Fatalf("TransitionEarning(invalid) error = %v, want ErrInvalidEarningTransition", err)
	}

	actionCases := []struct {
		status EarningStatus
		all    []EarningAction
		manual []EarningAction
	}{
		{EarningStatusPending, []EarningAction{EarningActionMakeAvailable, EarningActionHold}, []EarningAction{EarningActionHold}},
		{EarningStatusAvailable, []EarningAction{EarningActionHold, EarningActionStartSettlement, EarningActionReverse}, []EarningAction{EarningActionHold, EarningActionReverse}},
		{EarningStatusHeld, []EarningAction{EarningActionRelease, EarningActionReverse}, []EarningAction{EarningActionRelease, EarningActionReverse}},
		{EarningStatusSettling, []EarningAction{EarningActionSettle, EarningActionRejectSettlement}, []EarningAction{}},
		{EarningStatusSettled, []EarningAction{EarningActionReverse}, []EarningAction{EarningActionReverse}},
		{EarningStatusReversed, []EarningAction{}, []EarningAction{}},
	}
	for _, tt := range actionCases {
		if got := AvailableEarningActions(tt.status); !reflect.DeepEqual(got, tt.all) {
			t.Fatalf("AvailableEarningActions(%q) = %v, want %v", tt.status, got, tt.all)
		}
		if got := AvailableManualEarningActions(tt.status); !reflect.DeepEqual(got, tt.manual) {
			t.Fatalf("AvailableManualEarningActions(%q) = %v, want %v", tt.status, got, tt.manual)
		}
	}
	if got := AvailableEarningActions("unknown"); len(got) != 0 {
		t.Fatalf("AvailableEarningActions(unknown) = %v, want empty", got)
	}
	if got := AvailableManualEarningActions("unknown"); len(got) != 0 {
		t.Fatalf("AvailableManualEarningActions(unknown) = %v, want empty", got)
	}
}

func TestCommissionCoverageInputLimitsAndClones(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	event := CommissionEvent{
		TenantID:       "tenant-a",
		SourceType:     "payment.succeeded",
		SourceID:       "payment-1",
		OccurredAt:     now,
		Commissionable: Amount{Currency: "USD", Minor: 1},
		Attributes:     map[string]string{"order": "one"},
	}
	cloned := event.Clone()
	cloned.Attributes["order"] = "changed"
	if event.Attributes["order"] != "one" {
		t.Fatal("CommissionEvent.Clone() shares attribute storage")
	}
	if err := validateCommissionEvent(event); err != nil {
		t.Fatalf("validateCommissionEvent(valid) error = %v", err)
	}
	for _, invalid := range []CommissionEvent{
		{TenantID: event.TenantID, SourceType: " ", SourceID: event.SourceID, OccurredAt: now, Commissionable: event.Commissionable},
		{TenantID: event.TenantID, SourceType: event.SourceType, SourceID: " ", OccurredAt: now, Commissionable: event.Commissionable},
		{TenantID: event.TenantID, SourceType: event.SourceType, SourceID: event.SourceID, Commissionable: event.Commissionable},
		{TenantID: event.TenantID, SourceType: event.SourceType, SourceID: event.SourceID, OccurredAt: now, Commissionable: Amount{Currency: "USD", Minor: -1}},
	} {
		if err := validateCommissionEvent(invalid); !errors.Is(err, ErrInvalidCommissionEvent) {
			t.Fatalf("validateCommissionEvent(%+v) error = %v, want ErrInvalidCommissionEvent", invalid, err)
		}
	}

	for _, beneficiary := range []BeneficiaryRef{
		{Kind: BeneficiaryKindTenant, ID: "tenant-a"},
		{Kind: BeneficiaryKindUser, ID: "user-a"},
		{Kind: BeneficiaryKindExternal, ID: "partner-a"},
	} {
		if !validBeneficiary(beneficiary) {
			t.Fatalf("validBeneficiary(%+v) = false", beneficiary)
		}
	}
	if validBeneficiary(BeneficiaryRef{Kind: BeneficiaryKindUser, ID: " "}) || validBeneficiary(BeneficiaryRef{Kind: "unknown", ID: "id"}) {
		t.Fatal("validBeneficiary accepted invalid reference")
	}

	defaults := DefaultServiceLimits()
	limits, err := normalizeServiceLimits(ServiceLimits{})
	if err != nil || limits != defaults {
		t.Fatalf("normalizeServiceLimits(zero) = %+v, %v; want defaults, nil", limits, err)
	}
	if _, err := normalizeServiceLimits(ServiceLimits{MaxIdentifierBytes: -1}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("normalizeServiceLimits(negative) error = %v, want ErrLimitExceeded", err)
	}
	if _, err := normalizeServiceLimits(ServiceLimits{DefaultPageSize: 2, MaxPageSize: 1}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("normalizeServiceLimits(inverted page) error = %v, want ErrLimitExceeded", err)
	}

	small := defaults
	small.MaxIdentifierBytes = 4
	small.MaxEventAttributes = 1
	small.MaxEventAttributeBytes = 4
	small.MaxEventTypes = 1
	small.MaxRulesPerTemplate = 1
	small.MaxTiersPerRule = 1
	small.MaxFilterStatuses = 1
	small.DefaultPageSize = 1
	small.MaxPageSize = 2
	if err := small.validateEvent(event); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("validateEvent(long source) error = %v, want ErrLimitExceeded", err)
	}
	validSmallEvent := event
	validSmallEvent.SourceType = "pay"
	validSmallEvent.SourceID = "id"
	validSmallEvent.Attributes = map[string]string{"k": "v"}
	if err := small.validateEvent(validSmallEvent); err != nil {
		t.Fatalf("validateEvent(small valid) error = %v", err)
	}
	for _, attributes := range []map[string]string{
		{"": "v"},
		{"key": "vv"},
		{"k": "value"},
		{"a": "bb", "c": "d"},
	} {
		candidate := validSmallEvent
		candidate.Attributes = attributes
		if err := small.validateEvent(candidate); !errors.Is(err, ErrLimitExceeded) {
			t.Fatalf("validateEvent(attributes=%v) error = %v, want ErrLimitExceeded", attributes, err)
		}
	}

	rule := Rule{Slot: "slot", Beneficiary: BeneficiaryRef{Kind: BeneficiaryKindUser, ID: "user"}, Tiers: []Tier{{MinMinor: 0, BasisPoints: 100}}}
	template := Template{ID: "temp", AllowedEventTypes: []string{"pay"}, Rules: []Rule{rule}}
	if err := small.validateTemplate(template); err != nil {
		t.Fatalf("validateTemplate(small valid) error = %v", err)
	}
	template.AllowedEventTypes = append(template.AllowedEventTypes, "more")
	if err := small.validateTemplate(template); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("validateTemplate(too many event types) error = %v, want ErrLimitExceeded", err)
	}
	template.AllowedEventTypes = []string{"toolong"}
	if err := small.validateTemplate(template); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("validateTemplate(long event type) error = %v, want ErrLimitExceeded", err)
	}
	program := Program{ID: "prog", TemplateID: "temp", Rules: []Rule{rule}}
	if err := small.validateProgram(program); err != nil {
		t.Fatalf("validateProgram(small valid) error = %v", err)
	}
	program.Rules = append(program.Rules, rule)
	if err := small.validateProgram(program); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("validateProgram(too many rules) error = %v, want ErrLimitExceeded", err)
	}
	if err := small.validateRules([]Rule{{Slot: "slot", Beneficiary: BeneficiaryRef{Kind: BeneficiaryKindUser, ID: "user"}, Tiers: []Tier{{MinMinor: 0, BasisPoints: 100}, {MinMinor: 1, BasisPoints: 100}}}}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("validateRules(too many tiers) error = %v, want ErrLimitExceeded", err)
	}
	if err := small.validateAttribution(Attribution{ProgramID: "prog", Slot: "slot", Beneficiary: BeneficiaryRef{Kind: BeneficiaryKindUser, ID: "user"}}); err != nil {
		t.Fatalf("validateAttribution(small valid) error = %v", err)
	}
	if err := small.validateAttribution(Attribution{ProgramID: "prog", Slot: "slot", Beneficiary: BeneficiaryRef{Kind: BeneficiaryKindUser, ID: "toolong"}}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("validateAttribution(long beneficiary) error = %v, want ErrLimitExceeded", err)
	}

	filter, err := small.normalizeEarningFilter(EarningFilter{})
	if err != nil || filter.Limit != small.DefaultPageSize {
		t.Fatalf("normalizeEarningFilter(default) = %+v, %v; want default limit", filter, err)
	}
	if _, err := small.normalizeEarningFilter(EarningFilter{Limit: 3}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("normalizeEarningFilter(large page) error = %v, want ErrLimitExceeded", err)
	}
	if _, err := small.normalizeEarningFilter(EarningFilter{Limit: -1}); !errors.Is(err, ErrInvalidEarningFilter) {
		t.Fatalf("normalizeEarningFilter(invalid) error = %v, want ErrInvalidEarningFilter", err)
	}
	if _, err := small.normalizeEarningFilter(EarningFilter{Cursor: "toolong"}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("normalizeEarningFilter(long cursor) error = %v, want ErrLimitExceeded", err)
	}
	if _, err := small.normalizeEarningFilter(EarningFilter{Statuses: []EarningStatus{EarningStatusAvailable, EarningStatusHeld}}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("normalizeEarningFilter(too many statuses) error = %v, want ErrLimitExceeded", err)
	}

	outboxFilter, err := small.normalizeOutboxFilter(OutboxFilter{})
	if err != nil || outboxFilter.Limit != small.DefaultPageSize {
		t.Fatalf("normalizeOutboxFilter(default) = %+v, %v; want default limit", outboxFilter, err)
	}
	if _, err := small.normalizeOutboxFilter(OutboxFilter{Cursor: OutboxCursor{ID: "cursor"}}); !errors.Is(err, ErrInvalidOutboxFilter) {
		t.Fatalf("normalizeOutboxFilter(invalid cursor) error = %v, want ErrInvalidOutboxFilter", err)
	}
	if _, err := small.normalizeOutboxFilter(OutboxFilter{Limit: 3}); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("normalizeOutboxFilter(large page) error = %v, want ErrLimitExceeded", err)
	}
}

func TestCommissionCoverageServiceAttributionReadAndOutboxCommands(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 17, 12, 0, 0, 0, time.UTC)
	store := NewMemoryStore()
	service := newTestService(store, func() time.Time { return now })
	setupActiveProgram(t, ctx, service, "tenant-a", 0)
	tenantActor := Actor{ID: "tenant-admin", TenantID: "tenant-a"}

	first, err := service.SetAttribution(ctx, tenantActor, "tenant-a", Attribution{
		TenantID:    "tenant-a",
		ProgramID:   "program-a",
		Slot:        "referral",
		Beneficiary: BeneficiaryRef{Kind: BeneficiaryKindUser, ID: "user-override"},
		Active:      true,
	}, 0)
	if err != nil || first.Version != 1 {
		t.Fatalf("SetAttribution(create) = %+v, %v; want version 1", first, err)
	}
	updatedInput := first
	updatedInput.Beneficiary = BeneficiaryRef{Kind: BeneficiaryKindExternal, ID: "partner-override"}
	updated, err := service.SetAttribution(ctx, tenantActor, "tenant-a", updatedInput, first.Version)
	if err != nil || updated.Version != 2 || updated.CreatedAt != first.CreatedAt {
		t.Fatalf("SetAttribution(update) = %+v, %v; want version 2 and original creation", updated, err)
	}
	if _, err := service.SetAttribution(ctx, tenantActor, "tenant-a", Attribution{
		TenantID: "tenant-a", ProgramID: "program-a", Slot: "missing", Beneficiary: BeneficiaryRef{Kind: BeneficiaryKindUser, ID: "user"}, Active: true,
	}, 0); !errors.Is(err, ErrInvalidAttribution) {
		t.Fatalf("SetAttribution(missing slot) error = %v, want ErrInvalidAttribution", err)
	}

	earnings, err := service.RecordEvent(ctx, hostActor(), "program-a", CommissionEvent{
		TenantID:       "tenant-a",
		SourceType:     "payment.succeeded",
		SourceID:       "attributed-payment",
		OccurredAt:     now,
		Commissionable: Amount{Currency: "USD", Minor: 10_000},
	})
	if err != nil || len(earnings) != 1 || earnings[0].Beneficiary != updated.Beneficiary {
		t.Fatalf("RecordEvent(attributed) = %+v, %v; want override beneficiary", earnings, err)
	}

	listed, err := service.ListEarnings(ctx, tenantActor, "tenant-a", EarningFilter{Beneficiary: &updated.Beneficiary})
	if err != nil || len(listed) != 1 || listed[0].ID != earnings[0].ID {
		t.Fatalf("ListEarnings() = %+v, %v; want attributed earning", listed, err)
	}
	journal, err := service.ListJournalEntries(ctx, tenantActor, "tenant-a", earnings[0].ID)
	if err != nil || len(journal) != 2 || journal[0].Kind != JournalKindAccrual || journal[1].Kind != JournalKindAvailable {
		t.Fatalf("ListJournalEntries() = %+v, %v; want accrual then available", journal, err)
	}

	settlement, err := service.StartSettlement(ctx, tenantActor, "tenant-a", "settlement-coverage", []string{earnings[0].ID})
	if err != nil {
		t.Fatalf("StartSettlement() error = %v", err)
	}
	gotSettlement, err := service.GetSettlement(ctx, tenantActor, "tenant-a", settlement.ID)
	if err != nil || gotSettlement.ID != settlement.ID {
		t.Fatalf("GetSettlement() = %+v, %v; want %q", gotSettlement, err, settlement.ID)
	}

	events, err := service.ListOutbox(ctx, hostActor(), "tenant-a", OutboxFilter{UnpublishedOnly: true})
	if err != nil || len(events) == 0 {
		t.Fatalf("ListOutbox() = %+v, %v; want unpublished events", events, err)
	}
	publishedAt := now.Add(time.Minute)
	if err := service.MarkOutboxPublished(ctx, hostActor(), "tenant-a", events[0].ID, publishedAt); err != nil {
		t.Fatalf("MarkOutboxPublished() error = %v", err)
	}
	remaining, err := service.ListOutbox(ctx, hostActor(), "tenant-a", OutboxFilter{UnpublishedOnly: true})
	if err != nil {
		t.Fatalf("ListOutbox(after mark) error = %v", err)
	}
	for _, event := range remaining {
		if event.ID == events[0].ID {
			t.Fatalf("ListOutbox(after mark) still returned published event %q", event.ID)
		}
	}
}
