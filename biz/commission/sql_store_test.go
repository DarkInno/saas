package commission

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestNewSQLStoreValidatesConfiguration(t *testing.T) {
	t.Parallel()
	if _, err := NewSQLStore(nil); !errors.Is(err, ErrNilDB) {
		t.Fatalf("NewSQLStore(nil) error = %v, want ErrNilDB", err)
	}

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	names := defaultSQLTableNames()
	names.Earnings = "finance.commission_earnings"
	store, err := NewSQLStore(db, WithTableNames(names), WithSQLDialect(SQLDialectPostgres))
	if err != nil {
		t.Fatalf("NewSQLStore() error = %v", err)
	}
	if store.tables != names {
		t.Fatalf("SQLStore tables = %+v, want %+v", store.tables, names)
	}
	if got := store.placeholder(3); got != "$3" {
		t.Fatalf("Postgres placeholder = %q, want $3", got)
	}

	unsafe := defaultSQLTableNames()
	unsafe.Outbox = "commission_outbox; DROP TABLE users"
	if _, err := NewSQLStore(db, WithTableNames(unsafe)); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("NewSQLStore(unsafe table) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := NewSQLStore(db, WithSQLDialect(SQLDialect("oracle"))); !errors.Is(err, ErrUnsupportedSQLDialect) {
		t.Fatalf("NewSQLStore(unsupported dialect) error = %v, want ErrUnsupportedSQLDialect", err)
	}
}

func TestSQLStoreCommitEventInsertsAtomically(t *testing.T) {
	store, mock, db := newMockCommissionStore(t)
	defer func() { _ = db.Close() }()
	commit, program, template, earning := sqlStoreCommitFixture(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT fingerprint FROM %s WHERE tenant_id = %s AND source_type = %s AND source_id = %s FOR UPDATE",
		store.tables.Events, store.placeholder(1), store.placeholder(2), store.placeholder(3),
	))).WithArgs(commit.Event.TenantID.String(), commit.Event.SourceType, commit.Event.SourceID).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
		programSelectColumns, store.tables.Programs, store.placeholder(1), store.placeholder(2),
	))).WithArgs(commit.Event.TenantID.String(), commit.ProgramID).WillReturnRows(programRows(program))
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT %s FROM %s WHERE tenant_id = %s AND program_id = %s AND slot = %s FOR UPDATE",
		attributionColumns, store.tables.Attributions, store.placeholder(1), store.placeholder(2), store.placeholder(3),
	))).WithArgs(commit.Event.TenantID.String(), program.ID, "referrer").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT %s FROM %s WHERE id = %s AND version = %s FOR UPDATE",
		templateSelectColumns, store.tables.Templates, store.placeholder(1), store.placeholder(2),
	))).WithArgs(commit.TemplateID, commit.TemplateVersion).WillReturnRows(templateRows(template))
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT id FROM %s WHERE id = %s FOR UPDATE",
		store.tables.Journals, store.placeholder(1),
	))).WithArgs(commit.Journals[0].ID).WillReturnError(sql.ErrNoRows)

	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
		"INSERT INTO %s (tenant_id, source_type, source_id, fingerprint, program_id, program_version, template_id, template_version, occurred_at, payload) VALUES (%s)",
		store.tables.Events, store.placeholders(10, 1),
	))).WithArgs(
		commit.Event.TenantID.String(), commit.Event.SourceType, commit.Event.SourceID, eventIdempotencyFingerprint(commit.Event, commit.ProgramID),
		commit.ProgramID, commit.ProgramVersion, commit.TemplateID, commit.TemplateVersion, commit.Event.OccurredAt, sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", store.tables.Earnings, earningSelectColumns, store.placeholders(18, 1)))).WithArgs(
		earning.TenantID.String(), earning.ID, earning.ProgramID, earning.TemplateID, earning.TemplateVersion,
		earning.SourceType, earning.SourceID, earning.Slot, string(earning.Beneficiary.Kind), earning.Beneficiary.ID,
		earning.Amount.Currency, earning.Amount.Minor, string(earning.Status), earning.AvailableAt, earning.Version,
		earning.CreatedAt, earning.UpdatedAt, sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	journal := commit.Journals[0]
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", store.tables.Journals, journalSelectColumns, store.placeholders(8, 1)))).WithArgs(
		journal.TenantID.String(), journal.ID, journal.EarningID, string(journal.Kind), journal.Amount.Currency, journal.Amount.Minor, journal.CreatedAt, sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	outbox := commit.Outbox[0]
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", store.tables.Outbox, outboxSelectColumns, store.placeholders(7, 1)))).WithArgs(
		outbox.TenantID.String(), outbox.ID, outbox.Type, outbox.AggregateID, outbox.CreatedAt, nil, sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	got, err := store.CommitEvent(ctx, commit)
	if err != nil {
		t.Fatalf("CommitEvent() error = %v", err)
	}
	if len(got) != 1 || got[0] != earning {
		t.Fatalf("CommitEvent() = %+v, want %+v", got, []Earning{earning})
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestSQLStoreCommitEventReturnsOriginalForMatchingDuplicate(t *testing.T) {
	store, mock, db := newMockCommissionStore(t)
	defer func() { _ = db.Close() }()
	commit, _, _, earning := sqlStoreCommitFixture(t)
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT fingerprint FROM %s WHERE tenant_id = %s AND source_type = %s AND source_id = %s FOR UPDATE",
		store.tables.Events, store.placeholder(1), store.placeholder(2), store.placeholder(3),
	))).WithArgs(commit.Event.TenantID.String(), commit.Event.SourceType, commit.Event.SourceID).WillReturnRows(
		sqlmock.NewRows([]string{"fingerprint"}).AddRow(eventIdempotencyFingerprint(commit.Event, commit.ProgramID)),
	)
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT %s FROM %s WHERE tenant_id = %s AND source_type = %s AND source_id = %s ORDER BY id",
		earningSelectColumns, store.tables.Earnings, store.placeholder(1), store.placeholder(2), store.placeholder(3),
	))).WithArgs(commit.Event.TenantID.String(), commit.Event.SourceType, commit.Event.SourceID).WillReturnRows(earningRows(earning))
	mock.ExpectCommit()

	got, err := store.CommitEvent(ctx, commit)
	if err != nil {
		t.Fatalf("CommitEvent(duplicate) error = %v", err)
	}
	if len(got) != 1 || got[0] != earning {
		t.Fatalf("CommitEvent(duplicate) = %+v, want original %+v", got, earning)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestSQLStoreCommitEventRecordsZeroCommissionAndReturnsEmptyDuplicate(t *testing.T) {
	store, mock, db := newMockCommissionStore(t)
	defer func() { _ = db.Close() }()
	commit, program, template, _ := sqlStoreCommitFixture(t)
	commit.Earnings = nil
	commit.Journals = nil
	commit.Outbox = nil
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT fingerprint FROM %s WHERE tenant_id = %s AND source_type = %s AND source_id = %s FOR UPDATE",
		store.tables.Events, store.placeholder(1), store.placeholder(2), store.placeholder(3),
	))).WithArgs(commit.Event.TenantID.String(), commit.Event.SourceType, commit.Event.SourceID).WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
		programSelectColumns, store.tables.Programs, store.placeholder(1), store.placeholder(2),
	))).WithArgs(commit.Event.TenantID.String(), commit.ProgramID).WillReturnRows(programRows(program))
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT %s FROM %s WHERE tenant_id = %s AND program_id = %s AND slot = %s FOR UPDATE",
		attributionColumns, store.tables.Attributions, store.placeholder(1), store.placeholder(2), store.placeholder(3),
	))).WithArgs(commit.Event.TenantID.String(), program.ID, "referrer").WillReturnError(sql.ErrNoRows)
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT %s FROM %s WHERE id = %s AND version = %s FOR UPDATE",
		templateSelectColumns, store.tables.Templates, store.placeholder(1), store.placeholder(2),
	))).WithArgs(commit.TemplateID, commit.TemplateVersion).WillReturnRows(templateRows(template))
	mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
		"INSERT INTO %s (tenant_id, source_type, source_id, fingerprint, program_id, program_version, template_id, template_version, occurred_at, payload) VALUES (%s)",
		store.tables.Events, store.placeholders(10, 1),
	))).WithArgs(
		commit.Event.TenantID.String(), commit.Event.SourceType, commit.Event.SourceID, eventIdempotencyFingerprint(commit.Event, commit.ProgramID),
		commit.ProgramID, commit.ProgramVersion, commit.TemplateID, commit.TemplateVersion, commit.Event.OccurredAt, sqlmock.AnyArg(),
	).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	first, err := store.CommitEvent(ctx, commit)
	if err != nil {
		t.Fatalf("CommitEvent(zero commission) error = %v", err)
	}
	if first == nil || len(first) != 0 {
		t.Fatalf("CommitEvent(zero commission) = %#v, want non-nil empty slice", first)
	}

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT fingerprint FROM %s WHERE tenant_id = %s AND source_type = %s AND source_id = %s FOR UPDATE",
		store.tables.Events, store.placeholder(1), store.placeholder(2), store.placeholder(3),
	))).WithArgs(commit.Event.TenantID.String(), commit.Event.SourceType, commit.Event.SourceID).WillReturnRows(
		sqlmock.NewRows([]string{"fingerprint"}).AddRow(eventIdempotencyFingerprint(commit.Event, commit.ProgramID)),
	)
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT %s FROM %s WHERE tenant_id = %s AND source_type = %s AND source_id = %s ORDER BY id",
		earningSelectColumns, store.tables.Earnings, store.placeholder(1), store.placeholder(2), store.placeholder(3),
	))).WithArgs(commit.Event.TenantID.String(), commit.Event.SourceType, commit.Event.SourceID).WillReturnRows(emptyEarningRows())
	mock.ExpectCommit()

	duplicate, err := store.CommitEvent(ctx, commit)
	if err != nil {
		t.Fatalf("CommitEvent(zero duplicate) error = %v", err)
	}
	if duplicate == nil || len(duplicate) != 0 {
		t.Fatalf("CommitEvent(zero duplicate) = %#v, want non-nil empty slice", duplicate)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func TestSQLStoreCreateProgramRequiresInitialDraftVersion(t *testing.T) {
	store, mock, db := newMockCommissionStore(t)
	defer func() { _ = db.Close() }()
	_, program, _, _ := sqlStoreCommitFixture(t)

	if err := store.CreateProgram(context.Background(), program); !errors.Is(err, ErrInvalidProgram) {
		t.Fatalf("CreateProgram(active/version=3) error = %v, want ErrInvalidProgram", err)
	}
	program.Status = ProgramStatusDraft
	program.Version = 2
	if err := store.CreateProgram(context.Background(), program); !errors.Is(err, ErrInvalidProgram) {
		t.Fatalf("CreateProgram(draft/version=2) error = %v, want ErrInvalidProgram", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unexpected SQL for invalid initial programs: %v", err)
	}
}

func TestSQLStoreTransitionProgramLocksTemplateBeforeEnteringActive(t *testing.T) {
	tests := []struct {
		name   string
		status ProgramStatus
		action ProgramAction
	}{
		{name: "approve", status: ProgramStatusPendingApproval, action: ProgramActionApprove},
		{name: "resume", status: ProgramStatusSuspended, action: ProgramActionResume},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store, mock, db := newMockCommissionStore(t)
			defer func() { _ = db.Close() }()
			_, program, template, _ := sqlStoreCommitFixture(t)
			program.Status = tt.status
			template.Status = TemplateStatusDraft

			mock.ExpectBegin()
			mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
				"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
				programSelectColumns, store.tables.Programs, store.placeholder(1), store.placeholder(2),
			))).WithArgs(program.TenantID.String(), program.ID).WillReturnRows(programRows(program))
			mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
				"SELECT %s FROM %s WHERE id = %s AND version = %s FOR UPDATE",
				templateSelectColumns, store.tables.Templates, store.placeholder(1), store.placeholder(2),
			))).WithArgs(program.TemplateID, program.TemplateVersion).WillReturnRows(templateRows(template))
			mock.ExpectRollback()

			_, err := store.TransitionProgram(context.Background(), program.TenantID, program.ID, program.Version, tt.action, program.UpdatedAt.Add(time.Minute))
			if !errors.Is(err, ErrTemplateNotActive) {
				t.Fatalf("TransitionProgram(%s) error = %v, want ErrTemplateNotActive", tt.action, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatalf("unmet SQL expectations: %v", err)
			}
		})
	}
}

func TestSQLStoreTransitionProgramVersionConflict(t *testing.T) {
	store, mock, db := newMockCommissionStore(t)
	defer func() { _ = db.Close() }()
	_, program, _, _ := sqlStoreCommitFixture(t)
	program.Version = 2
	ctx := context.Background()

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
		"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
		programSelectColumns, store.tables.Programs, store.placeholder(1), store.placeholder(2),
	))).WithArgs(program.TenantID.String(), program.ID).WillReturnRows(programRows(program))
	mock.ExpectRollback()

	_, err := store.TransitionProgram(ctx, program.TenantID, program.ID, 1, ProgramActionSubmit, program.UpdatedAt.Add(time.Minute))
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("TransitionProgram() error = %v, want ErrVersionConflict", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet SQL expectations: %v", err)
	}
}

func newMockCommissionStore(t *testing.T) (*SQLStore, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	store, err := NewSQLStore(db)
	if err != nil {
		_ = db.Close()
		t.Fatalf("NewSQLStore() error = %v", err)
	}
	return store, mock, db
}

func sqlStoreCommitFixture(t *testing.T) (EventCommit, Program, Template, Earning) {
	t.Helper()
	now := time.Date(2026, 7, 17, 9, 30, 0, 0, time.UTC)
	beneficiary := BeneficiaryRef{Kind: BeneficiaryKindUser, ID: "user-1"}
	template := Template{
		ID:        "template-1",
		Version:   1,
		Status:    TemplateStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
		Rules: []Rule{{
			Slot:        "referrer",
			Beneficiary: beneficiary,
			Tiers:       []Tier{{MinMinor: 0, BasisPoints: 500}},
		}},
	}
	program := Program{
		ID:              "program-1",
		TenantID:        "tenant-1",
		TemplateID:      template.ID,
		TemplateVersion: template.Version,
		Status:          ProgramStatusActive,
		Version:         3,
		Rules:           cloneRules(template.Rules),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	event := CommissionEvent{
		TenantID:       program.TenantID,
		SourceType:     "payment.succeeded",
		SourceID:       "payment-1",
		OccurredAt:     now,
		Commissionable: Amount{Currency: "USD", Minor: 10_000},
		Attributes:     map[string]string{"order_id": "order-1"},
	}
	earning := Earning{
		ID:              earningID(event, program.ID, "referrer", beneficiary),
		TenantID:        program.TenantID,
		ProgramID:       program.ID,
		TemplateID:      template.ID,
		TemplateVersion: template.Version,
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
		ID:          outboxID(earning.TenantID.String(), earning.ID, string(earning.Status), earning.Version),
		TenantID:    earning.TenantID,
		Type:        "commission.earning." + string(earning.Status),
		AggregateID: earning.ID,
		Payload: map[string]string{
			"earning_id": earning.ID,
			"status":     string(earning.Status),
		},
		CreatedAt: now,
	}
	commit := EventCommit{
		Event:               event,
		ProgramID:           program.ID,
		ProgramVersion:      program.Version,
		TemplateID:          template.ID,
		TemplateVersion:     template.Version,
		AttributionVersions: map[string]int64{"referrer": 0},
		Earnings:            []Earning{earning},
		Journals:            []JournalEntry{journal},
		Outbox:              []OutboxEvent{outbox},
	}
	return commit, program, template, earning
}

func templateRows(template Template) *sqlmock.Rows {
	payload, err := marshalPayload(template)
	if err != nil {
		panic(err)
	}
	return sqlmock.NewRows([]string{"id", "version", "status", "created_at", "updated_at", "payload"}).AddRow(
		template.ID, template.Version, string(template.Status), template.CreatedAt, template.UpdatedAt, payload,
	)
}

func programRows(program Program) *sqlmock.Rows {
	payload, err := marshalPayload(program)
	if err != nil {
		panic(err)
	}
	return sqlmock.NewRows([]string{"tenant_id", "id", "template_id", "template_version", "status", "version", "created_at", "updated_at", "payload"}).AddRow(
		program.TenantID.String(), program.ID, program.TemplateID, program.TemplateVersion, string(program.Status), program.Version, program.CreatedAt, program.UpdatedAt, payload,
	)
}

func earningRows(earning Earning) *sqlmock.Rows {
	payload, err := marshalPayload(earning)
	if err != nil {
		panic(err)
	}
	return sqlmock.NewRows([]string{
		"tenant_id", "id", "program_id", "template_id", "template_version", "source_type", "source_id", "slot", "beneficiary_kind", "beneficiary_id", "currency", "amount_minor", "status", "available_at", "version", "created_at", "updated_at", "payload",
	}).AddRow(
		earning.TenantID.String(), earning.ID, earning.ProgramID, earning.TemplateID, earning.TemplateVersion,
		earning.SourceType, earning.SourceID, earning.Slot, string(earning.Beneficiary.Kind), earning.Beneficiary.ID,
		earning.Amount.Currency, earning.Amount.Minor, string(earning.Status), earning.AvailableAt, earning.Version,
		earning.CreatedAt, earning.UpdatedAt, payload,
	)
}

func emptyEarningRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{
		"tenant_id", "id", "program_id", "template_id", "template_version", "source_type", "source_id", "slot", "beneficiary_kind", "beneficiary_id", "currency", "amount_minor", "status", "available_at", "version", "created_at", "updated_at", "payload",
	})
}
