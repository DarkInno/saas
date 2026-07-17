package commission

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"time"
)

func earningID(event CommissionEvent, programID string, slot string, beneficiary BeneficiaryRef) string {
	return stableID("earning", event.TenantID.String(), event.SourceType, event.SourceID, programID, slot, string(beneficiary.Kind), beneficiary.ID)
}

func journalID(earningID string, kind JournalKind, version int64) string {
	return stableID("journal", earningID, string(kind), strconv.FormatInt(version, 10))
}

func outboxID(tenantID string, aggregateID string, eventType string, version int64) string {
	return stableID("outbox", tenantID, aggregateID, eventType, strconv.FormatInt(version, 10))
}

func stableID(prefix string, parts ...string) string {
	// JSON encodes each field boundary explicitly, unlike delimiter-based
	// joining where identifiers containing that delimiter could collide.
	fields := make([]string, 1, len(parts)+1)
	fields[0] = prefix
	fields = append(fields, parts...)
	value, _ := json.Marshal(fields) // []string cannot fail to marshal.
	sum := sha256.Sum256(value)
	return prefix + "_" + hex.EncodeToString(sum[:16])
}

func eventFingerprint(event CommissionEvent) string {
	payload := struct {
		TenantID    string            `json:"tenant_id"`
		SourceType  string            `json:"source_type"`
		SourceID    string            `json:"source_id"`
		OccurredAt  string            `json:"occurred_at"`
		Currency    string            `json:"currency"`
		AmountMinor int64             `json:"amount_minor"`
		Attributes  map[string]string `json:"attributes"`
	}{
		TenantID:    event.TenantID.String(),
		SourceType:  event.SourceType,
		SourceID:    event.SourceID,
		OccurredAt:  event.OccurredAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"),
		Currency:    event.Commissionable.Currency,
		AmountMinor: event.Commissionable.Minor,
		Attributes:  event.Attributes,
	}
	encoded, _ := json.Marshal(payload)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

// eventIdempotencyFingerprint scopes a source-fact fingerprint to the program
// that made the commission decision. Program/template and attribution versions
// deliberately remain outside this key: once a source event has been decided
// for a program, a retry must return that original result even after later
// control-plane changes.
func eventIdempotencyFingerprint(event CommissionEvent, programID string) string {
	payload := struct {
		FactFingerprint string `json:"fact_fingerprint"`
		ProgramID       string `json:"program_id"`
	}{
		FactFingerprint: eventFingerprint(event),
		ProgramID:       programID,
	}
	return digestJSON(payload)
}

const commissionCalculatorVersion = "v1"

// DecisionSnapshot is the immutable, non-sensitive explanation of a recorded
// commission decision. It is embedded in the persisted event payload together
// with the original source event. Attributes remain only on that source event;
// the snapshot records their fact fingerprint rather than their raw values.
type DecisionSnapshot struct {
	FactFingerprint        string           `json:"fact_fingerprint"`
	IdempotencyFingerprint string           `json:"idempotency_fingerprint"`
	ProgramID              string           `json:"program_id"`
	ProgramVersion         int64            `json:"program_version"`
	TemplateID             string           `json:"template_id"`
	TemplateVersion        int64            `json:"template_version"`
	AttributionVersions    map[string]int64 `json:"attribution_versions"`
	CalculatorVersion      string           `json:"calculator_version"`
	OutcomeDigest          string           `json:"outcome_digest"`
}

type eventRecordPayload struct {
	Event    CommissionEvent  `json:"event"`
	Decision DecisionSnapshot `json:"decision"`
}

func decisionSnapshotFor(commit EventCommit) DecisionSnapshot {
	return DecisionSnapshot{
		FactFingerprint:        eventFingerprint(commit.Event),
		IdempotencyFingerprint: eventIdempotencyFingerprint(commit.Event, commit.ProgramID),
		ProgramID:              commit.ProgramID,
		ProgramVersion:         commit.ProgramVersion,
		TemplateID:             commit.TemplateID,
		TemplateVersion:        commit.TemplateVersion,
		AttributionVersions:    cloneAttributionVersions(commit.AttributionVersions),
		CalculatorVersion:      commissionCalculatorVersion,
		OutcomeDigest:          decisionOutcomeDigest(commit.Earnings),
	}
}

func cloneDecisionSnapshot(snapshot DecisionSnapshot) DecisionSnapshot {
	snapshot.AttributionVersions = cloneAttributionVersions(snapshot.AttributionVersions)
	return snapshot
}

func cloneAttributionVersions(versions map[string]int64) map[string]int64 {
	if versions == nil {
		return nil
	}
	cloned := make(map[string]int64, len(versions))
	for slot, version := range versions {
		cloned[slot] = version
	}
	return cloned
}

type decisionOutcomeEarning struct {
	ID              string `json:"id"`
	Slot            string `json:"slot"`
	BeneficiaryKind string `json:"beneficiary_kind"`
	BeneficiaryID   string `json:"beneficiary_id"`
	Currency        string `json:"currency"`
	AmountMinor     int64  `json:"amount_minor"`
	Status          string `json:"status"`
	AvailableAt     string `json:"available_at"`
}

func decisionOutcomeDigest(earnings []Earning) string {
	outcome := make([]decisionOutcomeEarning, 0, len(earnings))
	for _, earning := range earnings {
		outcome = append(outcome, decisionOutcomeEarning{
			ID:              earning.ID,
			Slot:            earning.Slot,
			BeneficiaryKind: string(earning.Beneficiary.Kind),
			BeneficiaryID:   earning.Beneficiary.ID,
			Currency:        earning.Amount.Currency,
			AmountMinor:     earning.Amount.Minor,
			Status:          string(earning.Status),
			AvailableAt:     canonicalTime(earning.AvailableAt),
		})
	}
	sort.Slice(outcome, func(left, right int) bool {
		if outcome[left].ID != outcome[right].ID {
			return outcome[left].ID < outcome[right].ID
		}
		if outcome[left].Slot != outcome[right].Slot {
			return outcome[left].Slot < outcome[right].Slot
		}
		if outcome[left].BeneficiaryKind != outcome[right].BeneficiaryKind {
			return outcome[left].BeneficiaryKind < outcome[right].BeneficiaryKind
		}
		return outcome[left].BeneficiaryID < outcome[right].BeneficiaryID
	})
	return digestJSON(struct {
		Earnings []decisionOutcomeEarning `json:"earnings"`
	}{Earnings: outcome})
}

func canonicalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func digestJSON(value any) string {
	encoded, _ := json.Marshal(value) // The snapshot inputs contain only JSON-safe values.
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
