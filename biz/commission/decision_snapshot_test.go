package commission

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMemoryStoreEventIdempotencyIsProgramScopedAndSurvivesControlPlaneChanges(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 17, 14, 0, 0, 0, time.UTC)
	store, commit, program := decisionSnapshotStoreFixture(t, ctx, now)

	first, err := store.CommitEvent(ctx, commit)
	if err != nil {
		t.Fatalf("CommitEvent(first) error = %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("CommitEvent(first) = %+v, want one earning", first)
	}

	// A later program lifecycle change must not invalidate a retry of the
	// original decision. The idempotency key intentionally excludes this
	// mutable control-plane version and state.
	if _, err := store.TransitionProgram(ctx, program.TenantID, program.ID, program.Version, ProgramActionSuspend, now.Add(time.Minute)); err != nil {
		t.Fatalf("TransitionProgram(suspend) error = %v", err)
	}
	retry, err := store.CommitEvent(ctx, commit)
	if err != nil {
		t.Fatalf("CommitEvent(same-program retry) error = %v", err)
	}
	if len(retry) != 1 || retry[0].ID != first[0].ID {
		t.Fatalf("CommitEvent(same-program retry) = %+v, want original %+v", retry, first)
	}

	otherProgram := decisionSnapshotCommit(now, "program-b", program.Version)
	if _, err := store.CommitEvent(ctx, otherProgram); !errors.Is(err, ErrEventConflict) {
		t.Fatalf("CommitEvent(other program same source) error = %v, want ErrEventConflict", err)
	}
}

func TestDecisionSnapshotCapturesImmutableDecisionWithoutRawAttributes(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 17, 14, 0, 0, 0, time.UTC)
	store, commit, _ := decisionSnapshotStoreFixture(t, ctx, now)
	commit.Event.Attributes["private"] = "private-order-token"

	if _, err := store.CommitEvent(ctx, commit); err != nil {
		t.Fatalf("CommitEvent() error = %v", err)
	}

	key := eventKey{tenantID: commit.Event.TenantID, sourceType: commit.Event.SourceType, sourceID: commit.Event.SourceID}
	stored, ok := store.events[key]
	if !ok {
		t.Fatal("CommitEvent() did not retain an event decision")
	}
	snapshot := stored.decision
	if snapshot.FactFingerprint != eventFingerprint(commit.Event) {
		t.Fatalf("snapshot fact fingerprint = %q, want %q", snapshot.FactFingerprint, eventFingerprint(commit.Event))
	}
	if snapshot.IdempotencyFingerprint != eventIdempotencyFingerprint(commit.Event, commit.ProgramID) {
		t.Fatalf("snapshot idempotency fingerprint = %q, want program-scoped fingerprint", snapshot.IdempotencyFingerprint)
	}
	if snapshot.ProgramID != commit.ProgramID || snapshot.ProgramVersion != commit.ProgramVersion || snapshot.TemplateID != commit.TemplateID || snapshot.TemplateVersion != commit.TemplateVersion {
		t.Fatalf("snapshot controls = %+v, want program/template values from commit", snapshot)
	}
	if snapshot.AttributionVersions["referrer"] != 0 {
		t.Fatalf("snapshot attribution versions = %+v, want referrer version 0", snapshot.AttributionVersions)
	}
	if snapshot.CalculatorVersion != commissionCalculatorVersion {
		t.Fatalf("snapshot calculator version = %q, want %q", snapshot.CalculatorVersion, commissionCalculatorVersion)
	}
	if snapshot.OutcomeDigest != decisionOutcomeDigest(commit.Earnings) {
		t.Fatalf("snapshot outcome digest = %q, want deterministic earnings digest", snapshot.OutcomeDigest)
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("json.Marshal(snapshot) error = %v", err)
	}
	if strings.Contains(string(encoded), "private-order-token") {
		t.Fatalf("snapshot leaks raw event attribute: %s", encoded)
	}

	// Persisted SQL event payloads contain both the source event and this
	// separate immutable decision record.
	execer := &decisionSnapshotExecer{}
	storeSQL := &SQLStore{tables: defaultSQLTableNames(), dialect: SQLDialectMySQL}
	if err := storeSQL.insertEvent(ctx, execer, commit, snapshot.IdempotencyFingerprint); err != nil {
		t.Fatalf("insertEvent() error = %v", err)
	}
	if len(execer.args) != 10 {
		t.Fatalf("insertEvent args = %d, want 10", len(execer.args))
	}
	payload, ok := execer.args[9].(string)
	if !ok {
		t.Fatalf("event payload type = %T, want string", execer.args[9])
	}
	var record eventRecordPayload
	if err := json.Unmarshal([]byte(payload), &record); err != nil {
		t.Fatalf("event payload decode error = %v", err)
	}
	if record.Event.Attributes["private"] != "private-order-token" {
		t.Fatalf("event payload source facts = %+v, want original attributes", record.Event.Attributes)
	}
	if !reflect.DeepEqual(record.Decision, snapshot) {
		t.Fatalf("event payload decision = %+v, want %+v", record.Decision, snapshot)
	}

	// Retained decision data must not alias caller-owned mutable maps.
	commit.AttributionVersions["referrer"] = 99
	if stored.decision.AttributionVersions["referrer"] != 0 {
		t.Fatalf("stored attribution versions mutated with caller: %+v", stored.decision.AttributionVersions)
	}

	second := commit.Earnings[0]
	second.ID = second.ID + "-another"
	second.Slot = "affiliate"
	if got, want := decisionOutcomeDigest([]Earning{commit.Earnings[0], second}), decisionOutcomeDigest([]Earning{second, commit.Earnings[0]}); got != want {
		t.Fatalf("decisionOutcomeDigest() = %q, want ordering-independent %q", got, want)
	}
}

func TestStoreCreateTemplateRequiresDraft(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 7, 17, 14, 0, 0, 0, time.UTC)
	active := decisionSnapshotTemplate(now)
	active.Status = TemplateStatusActive

	memory := NewMemoryStore()
	if err := memory.CreateTemplate(ctx, active); !errors.Is(err, ErrInvalidTemplate) {
		t.Fatalf("MemoryStore.CreateTemplate(active) error = %v, want ErrInvalidTemplate", err)
	}
	draft := active
	draft.Status = TemplateStatusDraft
	if err := memory.CreateTemplate(ctx, draft); err != nil {
		t.Fatalf("MemoryStore.CreateTemplate(draft) error = %v", err)
	}

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	sqlStore, err := NewSQLStore(db)
	if err != nil {
		t.Fatalf("NewSQLStore() error = %v", err)
	}
	if err := sqlStore.CreateTemplate(ctx, active); !errors.Is(err, ErrInvalidTemplate) {
		t.Fatalf("SQLStore.CreateTemplate(active) error = %v, want ErrInvalidTemplate", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQLStore.CreateTemplate(active) issued SQL: %v", err)
	}
}

type decisionSnapshotExecer struct {
	args []any
}

func (execer *decisionSnapshotExecer) ExecContext(_ context.Context, _ string, args ...any) (sql.Result, error) {
	execer.args = append([]any(nil), args...)
	return sqlmock.NewResult(1, 1), nil
}

func decisionSnapshotStoreFixture(t *testing.T, ctx context.Context, now time.Time) (*MemoryStore, EventCommit, Program) {
	t.Helper()
	store := NewMemoryStore()
	template := decisionSnapshotTemplate(now)
	if err := store.CreateTemplate(ctx, template); err != nil {
		t.Fatalf("CreateTemplate() error = %v", err)
	}
	activeTemplate, err := store.TransitionTemplate(ctx, template.ID, template.Version, TemplateActionActivate, now.Add(time.Second))
	if err != nil {
		t.Fatalf("TransitionTemplate(activate) error = %v", err)
	}
	program := Program{
		ID:              "program-a",
		TenantID:        "tenant-a",
		TemplateID:      activeTemplate.ID,
		TemplateVersion: activeTemplate.Version,
		Status:          ProgramStatusDraft,
		Version:         1,
		Rules:           cloneRules(activeTemplate.Rules),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := store.CreateProgram(ctx, program); err != nil {
		t.Fatalf("CreateProgram() error = %v", err)
	}
	program, err = store.TransitionProgram(ctx, program.TenantID, program.ID, program.Version, ProgramActionSubmit, now.Add(2*time.Second))
	if err != nil {
		t.Fatalf("TransitionProgram(submit) error = %v", err)
	}
	program, err = store.TransitionProgram(ctx, program.TenantID, program.ID, program.Version, ProgramActionApprove, now.Add(3*time.Second))
	if err != nil {
		t.Fatalf("TransitionProgram(approve) error = %v", err)
	}
	return store, decisionSnapshotCommit(now, program.ID, program.Version), program
}

func decisionSnapshotTemplate(now time.Time) Template {
	return Template{
		ID:        "template-decision",
		Version:   1,
		Status:    TemplateStatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
		Rules: []Rule{{
			Slot:        "referrer",
			Beneficiary: BeneficiaryRef{Kind: BeneficiaryKindUser, ID: "user-a"},
			Tiers:       []Tier{{MinMinor: 0, BasisPoints: 500}},
		}},
	}
}

func decisionSnapshotCommit(now time.Time, programID string, programVersion int64) EventCommit {
	event := CommissionEvent{
		TenantID:       "tenant-a",
		SourceType:     "payment.succeeded",
		SourceID:       "payment-decision",
		OccurredAt:     now,
		Commissionable: Amount{Currency: "USD", Minor: 10_000},
		Attributes:     map[string]string{"order_id": "order-decision"},
	}
	beneficiary := BeneficiaryRef{Kind: BeneficiaryKindUser, ID: "user-a"}
	earning := Earning{
		ID:              earningID(event, programID, "referrer", beneficiary),
		TenantID:        event.TenantID,
		ProgramID:       programID,
		TemplateID:      "template-decision",
		TemplateVersion: 1,
		SourceType:      event.SourceType,
		SourceID:        event.SourceID,
		Slot:            "referrer",
		Beneficiary:     beneficiary,
		Amount:          Amount{Currency: "USD", Minor: 500},
		Status:          EarningStatusAvailable,
		AvailableAt:     now,
		Version:         1,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	journal := JournalEntry{
		ID:        journalID(earning.ID, JournalKindAccrual, earning.Version),
		TenantID:  earning.TenantID,
		EarningID: earning.ID,
		Kind:      JournalKindAccrual,
		Amount:    earning.Amount,
		CreatedAt: now,
	}
	outbox := OutboxEvent{
		ID:          outboxID(event.TenantID.String(), earning.ID, "commission.earning.available", earning.Version),
		TenantID:    event.TenantID,
		Type:        "commission.earning.available",
		AggregateID: earning.ID,
		Payload:     map[string]string{"earning_id": earning.ID},
		CreatedAt:   now,
	}
	return EventCommit{
		Event:               event,
		ProgramID:           programID,
		ProgramVersion:      programVersion,
		TemplateID:          "template-decision",
		TemplateVersion:     1,
		AttributionVersions: map[string]int64{"referrer": 0},
		Earnings:            []Earning{earning},
		Journals:            []JournalEntry{journal},
		Outbox:              []OutboxEvent{outbox},
	}
}
