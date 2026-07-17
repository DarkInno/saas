package commission

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestAmountValidate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		amount Amount
		want   error
	}{
		{name: "valid", amount: Amount{Currency: "USD", Minor: 0}},
		{name: "valid nonzero", amount: Amount{Currency: "CNY", Minor: 1}},
		{name: "lowercase", amount: Amount{Currency: "usd", Minor: 1}, want: ErrInvalidAmount},
		{name: "wrong length", amount: Amount{Currency: "US", Minor: 1}, want: ErrInvalidAmount},
		{name: "non letter", amount: Amount{Currency: "U1D", Minor: 1}, want: ErrInvalidAmount},
		{name: "negative", amount: Amount{Currency: "USD", Minor: -1}, want: ErrInvalidAmount},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.amount.Validate(); !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestCalculate(t *testing.T) {
	t.Parallel()
	baseEvent := CommissionEvent{
		TenantID:   "tenant-a",
		SourceType: "payment.succeeded",
		SourceID:   "payment-1",
		OccurredAt: time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC),
		Commissionable: Amount{
			Currency: "USD",
			Minor:    15_000,
		},
	}
	beneficiary := BeneficiaryRef{Kind: BeneficiaryKindUser, ID: "user-1"}
	baseTemplate := Template{
		AllowedEventTypes: []string{"payment.succeeded"},
		MaxCommission:     Amount{Currency: "USD", Minor: 3_000},
		Rules: []Rule{{
			Slot:        "referrer",
			Beneficiary: beneficiary,
			Tiers: []Tier{
				{MinMinor: 10_000, MaxMinor: 0, BasisPoints: 2_000},
				{MinMinor: 0, MaxMinor: 10_000, BasisPoints: 1_000},
			},
		}},
	}
	tests := []struct {
		name     string
		template Template
		event    CommissionEvent
		want     []Calculation
		wantErr  error
	}{
		{
			name:     "progressive percentage sorts tiers without mutating source",
			template: baseTemplate,
			event:    baseEvent,
			want: []Calculation{{
				Slot:        "referrer",
				Beneficiary: beneficiary,
				Amount:      Amount{Currency: "USD", Minor: 2_000},
			}},
		},
		{
			name: "fixed applies once when event is in tier",
			template: Template{
				AllowedEventTypes: []string{"payment.succeeded"},
				Rules: []Rule{{
					Slot:        "affiliate",
					Beneficiary: beneficiary,
					Tiers:       []Tier{{MinMinor: 0, MaxMinor: 20_000, BasisPoints: 500, FixedMinor: 75}},
				}},
			},
			event: baseEvent,
			want: []Calculation{{
				Slot:        "affiliate",
				Beneficiary: beneficiary,
				Amount:      Amount{Currency: "USD", Minor: 825},
			}},
		},
		{
			name:     "disabled event type",
			template: Template{AllowedEventTypes: []string{"subscription.renewed"}, Rules: baseTemplate.Rules},
			event:    baseEvent,
			wantErr:  ErrEventTypeDisabled,
		},
		{
			name:     "cap exceeded",
			template: Template{AllowedEventTypes: baseTemplate.AllowedEventTypes, MaxCommission: Amount{Currency: "USD", Minor: 1_999}, Rules: baseTemplate.Rules},
			event:    baseEvent,
			wantErr:  ErrCommissionCapExceeded,
		},
		{
			name:     "currency mismatch",
			template: Template{AllowedEventTypes: baseTemplate.AllowedEventTypes, MaxCommission: Amount{Currency: "CNY", Minor: 3_000}, Rules: baseTemplate.Rules},
			event:    baseEvent,
			wantErr:  ErrCurrencyMismatch,
		},
		{
			name:     "empty beneficiary is a template cap but not an effective rule",
			template: Template{AllowedEventTypes: baseTemplate.AllowedEventTypes, Rules: []Rule{{Slot: "platform-cap", Tiers: []Tier{{MinMinor: 0, BasisPoints: 100}}}}},
			event:    baseEvent,
			wantErr:  ErrInvalidBeneficiary,
		},
		{
			name:     "invalid tier",
			template: Template{AllowedEventTypes: baseTemplate.AllowedEventTypes, Rules: []Rule{{Slot: "referrer", Beneficiary: beneficiary, Tiers: []Tier{{MinMinor: -1, BasisPoints: 100}}}}},
			event:    baseEvent,
			wantErr:  ErrInvalidTier,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := cloneTemplate(tt.template)
			got, err := Calculate(tt.template, tt.event)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Calculate() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Calculate() = %#v, want %#v", got, tt.want)
			}
			if !reflect.DeepEqual(tt.template, before) {
				t.Fatalf("Calculate() mutated template: got %#v, want %#v", tt.template, before)
			}
		})
	}
}

func TestTemplateValidationAndCloning(t *testing.T) {
	t.Parallel()
	created := time.Date(2026, 7, 17, 0, 0, 0, 0, time.UTC)
	template := Template{
		ID:        "template-a",
		Version:   1,
		Status:    TemplateStatusDraft,
		CreatedAt: created,
		UpdatedAt: created,
		Rules: []Rule{{
			Slot:  "platform-cap",
			Tiers: []Tier{{MinMinor: 0, BasisPoints: 100}},
		}},
	}
	if err := validateTemplate(template); err != nil {
		t.Fatalf("validateTemplate() error = %v", err)
	}
	cloned := cloneTemplate(template)
	cloned.Rules[0].Tiers[0].BasisPoints = 200
	if template.Rules[0].Tiers[0].BasisPoints != 100 {
		t.Fatal("cloneTemplate() shares tier storage")
	}

	event := CommissionEvent{Attributes: map[string]string{"source": "checkout"}}
	eventClone := cloneCommissionEvent(event)
	eventClone.Attributes["source"] = "changed"
	if event.Attributes["source"] != "checkout" {
		t.Fatal("cloneCommissionEvent() shares attributes")
	}
}

func TestTransitionEarning(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		status  EarningStatus
		action  EarningAction
		want    EarningStatus
		wantErr error
	}{
		{name: "pending available", status: EarningStatusPending, action: EarningActionMakeAvailable, want: EarningStatusAvailable},
		{name: "pending held", status: EarningStatusPending, action: EarningActionHold, want: EarningStatusHeld},
		{name: "available settling", status: EarningStatusAvailable, action: EarningActionStartSettlement, want: EarningStatusSettling},
		{name: "held release", status: EarningStatusHeld, action: EarningActionRelease, want: EarningStatusAvailable},
		{name: "settling settled", status: EarningStatusSettling, action: EarningActionSettle, want: EarningStatusSettled},
		{name: "settling reject", status: EarningStatusSettling, action: EarningActionRejectSettlement, want: EarningStatusAvailable},
		{name: "settling cannot reverse directly", status: EarningStatusSettling, action: EarningActionReverse, wantErr: ErrInvalidEarningTransition},
		{name: "settled reverse becomes recovery", status: EarningStatusSettled, action: EarningActionReverse, want: EarningStatusRecoveryDue},
		{name: "terminal reversed", status: EarningStatusReversed, action: EarningActionReverse, wantErr: ErrInvalidEarningTransition},
		{name: "terminal recovery due", status: EarningStatusRecoveryDue, action: EarningActionSettle, wantErr: ErrInvalidEarningTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := TransitionEarning(tt.status, tt.action)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("TransitionEarning() error = %v, want %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("TransitionEarning() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStableIDPreservesFieldBoundaries(t *testing.T) {
	t.Parallel()
	left := stableID("earning", "tenant\x00source", "event")
	right := stableID("earning", "tenant", "source\x00event")
	if left == right {
		t.Fatalf("stableID() collision across field boundaries: %q", left)
	}
	if again := stableID("earning", "tenant\x00source", "event"); again != left {
		t.Fatalf("stableID() = %q, want deterministic %q", again, left)
	}
}
