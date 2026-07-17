package commission

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// These tests deliberately exercise public SQLStore paths with sqlmock rather
// than duplicating implementation details in a fake Store. Besides covering
// the durable read/list paths, they protect the SQL column contracts and the
// tenant predicates used by the host-managed schema.

func TestSQLStoreCoverageReadOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("template versions and not found mapping", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, _, template, _ := sqlStoreCommitFixture(t)

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE id = %s ORDER BY version DESC LIMIT 1",
			templateSelectColumns, store.tables.Templates, store.placeholder(1),
		))).WithArgs(template.ID).WillReturnRows(templateRows(template))
		latest, err := store.GetTemplate(ctx, template.ID, 0)
		if err != nil {
			t.Fatalf("GetTemplate(latest) error = %v", err)
		}
		if !reflect.DeepEqual(latest, template) {
			t.Fatalf("GetTemplate(latest) = %+v, want %+v", latest, template)
		}

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE id = %s AND version = %s",
			templateSelectColumns, store.tables.Templates, store.placeholder(1), store.placeholder(2),
		))).WithArgs(template.ID, template.Version).WillReturnRows(templateRows(template))
		exact, err := store.GetTemplate(ctx, template.ID, template.Version)
		if err != nil {
			t.Fatalf("GetTemplate(exact) error = %v", err)
		}
		if !reflect.DeepEqual(exact, template) {
			t.Fatalf("GetTemplate(exact) = %+v, want %+v", exact, template)
		}

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE id = %s AND version = %s",
			templateSelectColumns, store.tables.Templates, store.placeholder(1), store.placeholder(2),
		))).WithArgs(template.ID, template.Version+1).WillReturnError(sql.ErrNoRows)
		if _, err := store.GetTemplate(ctx, template.ID, template.Version+1); !errors.Is(err, ErrTemplateNotFound) {
			t.Fatalf("GetTemplate(missing) error = %v, want ErrTemplateNotFound", err)
		}
		if _, err := store.GetTemplate(ctx, "", 1); !errors.Is(err, ErrInvalidTemplate) {
			t.Fatalf("GetTemplate(invalid) error = %v, want ErrInvalidTemplate", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("program earning and settlement are tenant scoped", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, program, _, earning := sqlStoreCommitFixture(t)
		settlement := sqlCoverageSettlement(earning, earning.CreatedAt)

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s",
			programSelectColumns, store.tables.Programs, store.placeholder(1), store.placeholder(2),
		))).WithArgs(program.TenantID.String(), program.ID).WillReturnRows(programRows(program))
		gotProgram, err := store.GetProgram(ctx, program.TenantID, program.ID)
		if err != nil {
			t.Fatalf("GetProgram() error = %v", err)
		}
		if !reflect.DeepEqual(gotProgram, program) {
			t.Fatalf("GetProgram() = %+v, want %+v", gotProgram, program)
		}

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s",
			earningSelectColumns, store.tables.Earnings, store.placeholder(1), store.placeholder(2),
		))).WithArgs(earning.TenantID.String(), earning.ID).WillReturnRows(earningRows(earning))
		gotEarning, err := store.GetEarning(ctx, earning.TenantID, earning.ID)
		if err != nil {
			t.Fatalf("GetEarning() error = %v", err)
		}
		if gotEarning != earning {
			t.Fatalf("GetEarning() = %+v, want %+v", gotEarning, earning)
		}

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s",
			settlementColumns, store.tables.Settlements, store.placeholder(1), store.placeholder(2),
		))).WithArgs(settlement.TenantID.String(), settlement.ID).WillReturnRows(sqlCoverageSettlementRows(t, settlement))
		gotSettlement, err := store.GetSettlement(ctx, settlement.TenantID, settlement.ID)
		if err != nil {
			t.Fatalf("GetSettlement() error = %v", err)
		}
		if !reflect.DeepEqual(gotSettlement, settlement) {
			t.Fatalf("GetSettlement() = %+v, want %+v", gotSettlement, settlement)
		}

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s",
			settlementColumns, store.tables.Settlements, store.placeholder(1), store.placeholder(2),
		))).WithArgs(settlement.TenantID.String(), "missing-settlement").WillReturnError(sql.ErrNoRows)
		if _, err := store.GetSettlement(ctx, settlement.TenantID, "missing-settlement"); !errors.Is(err, ErrSettlementNotFound) {
			t.Fatalf("GetSettlement(missing) error = %v, want ErrSettlementNotFound", err)
		}
		if _, err := store.GetEarning(ctx, "", earning.ID); !errors.Is(err, ErrInvalidEarning) {
			t.Fatalf("GetEarning(invalid) error = %v, want ErrInvalidEarning", err)
		}
		if _, err := store.GetSettlement(ctx, settlement.TenantID, ""); !errors.Is(err, ErrInvalidSettlement) {
			t.Fatalf("GetSettlement(invalid) error = %v, want ErrInvalidSettlement", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})
}

func TestSQLStoreCoverageListOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("attributions and journals", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, program, _, earning := sqlStoreCommitFixture(t)
		attribution := Attribution{
			TenantID:    program.TenantID,
			ProgramID:   program.ID,
			Slot:        "referrer",
			Beneficiary: program.Rules[0].Beneficiary,
			Active:      true,
			Version:     2,
			CreatedAt:   earning.CreatedAt,
			UpdatedAt:   earning.UpdatedAt,
		}
		journal := JournalEntry{
			ID:        "journal-read-1",
			TenantID:  earning.TenantID,
			EarningID: earning.ID,
			Kind:      JournalKindAccrual,
			Amount:    earning.Amount,
			CreatedAt: earning.CreatedAt,
		}

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND program_id = %s ORDER BY slot",
			attributionColumns, store.tables.Attributions, store.placeholder(1), store.placeholder(2),
		))).WithArgs(program.TenantID.String(), program.ID).WillReturnRows(sqlCoverageAttributionRows(t, attribution))
		attributions, err := store.ListAttributions(ctx, program.TenantID, program.ID)
		if err != nil {
			t.Fatalf("ListAttributions() error = %v", err)
		}
		if !reflect.DeepEqual(attributions, []Attribution{attribution}) {
			t.Fatalf("ListAttributions() = %+v, want %+v", attributions, []Attribution{attribution})
		}

		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND earning_id = %s ORDER BY created_at, id",
			journalSelectColumns, store.tables.Journals, store.placeholder(1), store.placeholder(2),
		))).WithArgs(earning.TenantID.String(), earning.ID).WillReturnRows(sqlCoverageJournalRows(t, journal))
		journals, err := store.ListJournalEntries(ctx, earning.TenantID, earning.ID)
		if err != nil {
			t.Fatalf("ListJournalEntries() error = %v", err)
		}
		if !reflect.DeepEqual(journals, []JournalEntry{journal}) {
			t.Fatalf("ListJournalEntries() = %+v, want %+v", journals, []JournalEntry{journal})
		}
		if _, err := store.ListAttributions(ctx, program.TenantID, ""); !errors.Is(err, ErrInvalidAttribution) {
			t.Fatalf("ListAttributions(invalid) error = %v, want ErrInvalidAttribution", err)
		}
		if _, err := store.ListJournalEntries(ctx, earning.TenantID, ""); !errors.Is(err, ErrInvalidEarning) {
			t.Fatalf("ListJournalEntries(invalid) error = %v, want ErrInvalidEarning", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("earnings filters use postgres placeholders", func(t *testing.T) {
		db, mock, err := sqlmock.New()
		if err != nil {
			t.Fatalf("sqlmock.New() error = %v", err)
		}
		defer func() { _ = db.Close() }()
		store, err := NewSQLStore(db, WithSQLDialect(SQLDialectPostgres))
		if err != nil {
			t.Fatalf("NewSQLStore() error = %v", err)
		}
		_, _, _, earning := sqlStoreCommitFixture(t)
		beneficiary := earning.Beneficiary
		filter := EarningFilter{
			ProgramID:   earning.ProgramID,
			Beneficiary: &beneficiary,
			Statuses:    []EarningStatus{EarningStatusAvailable, EarningStatusHeld},
			Cursor:      "earning-before",
			Limit:       2,
		}
		query := fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND program_id = %s AND beneficiary_kind = %s AND beneficiary_id = %s AND status IN (%s) AND id > %s ORDER BY id LIMIT %s",
			earningSelectColumns,
			store.tables.Earnings,
			store.placeholder(1),
			store.placeholder(2),
			store.placeholder(3),
			store.placeholder(4),
			store.placeholders(2, 5),
			store.placeholder(7),
			store.placeholder(8),
		)
		mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(
			earning.TenantID.String(), earning.ProgramID, string(beneficiary.Kind), beneficiary.ID,
			string(EarningStatusAvailable), string(EarningStatusHeld), filter.Cursor, filter.Limit,
		).WillReturnRows(earningRows(earning))
		got, err := store.ListEarnings(ctx, earning.TenantID, filter)
		if err != nil {
			t.Fatalf("ListEarnings() error = %v", err)
		}
		if !reflect.DeepEqual(got, []Earning{earning}) {
			t.Fatalf("ListEarnings() = %+v, want %+v", got, []Earning{earning})
		}
		if _, err := store.ListEarnings(ctx, earning.TenantID, EarningFilter{Limit: -1}); !errors.Is(err, ErrInvalidEarningFilter) {
			t.Fatalf("ListEarnings(invalid) error = %v, want ErrInvalidEarningFilter", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("outbox cursor and error path", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, _, _, earning := sqlStoreCommitFixture(t)
		event := OutboxEvent{
			ID:          "outbox-read-1",
			TenantID:    earning.TenantID,
			Type:        "commission.earning.available",
			AggregateID: earning.ID,
			Payload:     map[string]string{"earning_id": earning.ID},
			CreatedAt:   earning.CreatedAt,
		}
		cursor := OutboxCursor{CreatedAt: event.CreatedAt.Add(-time.Minute), ID: "outbox-before"}
		filter := OutboxFilter{UnpublishedOnly: true, Cursor: cursor, Limit: 1}
		query := fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND published_at IS NULL AND (created_at > %s OR (created_at = %s AND id > %s)) ORDER BY created_at, id LIMIT %s",
			outboxSelectColumns,
			store.tables.Outbox,
			store.placeholder(1),
			store.placeholder(2),
			store.placeholder(3),
			store.placeholder(4),
			store.placeholder(5),
		)
		mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(
			event.TenantID.String(), cursor.CreatedAt, cursor.CreatedAt, cursor.ID, filter.Limit,
		).WillReturnRows(sqlCoverageOutboxRows(t, event))
		got, err := store.ListOutbox(ctx, event.TenantID, filter)
		if err != nil {
			t.Fatalf("ListOutbox() error = %v", err)
		}
		if !reflect.DeepEqual(got, []OutboxEvent{event}) {
			t.Fatalf("ListOutbox() = %+v, want %+v", got, []OutboxEvent{event})
		}

		query = fmt.Sprintf("SELECT %s FROM %s WHERE tenant_id = %s ORDER BY created_at, id", outboxSelectColumns, store.tables.Outbox, store.placeholder(1))
		readErr := errors.New("outbox read unavailable")
		mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(event.TenantID.String()).WillReturnError(readErr)
		if _, err := store.ListOutbox(ctx, event.TenantID, OutboxFilter{}); !errors.Is(err, readErr) {
			t.Fatalf("ListOutbox(query error) = %v, want %v", err, readErr)
		}
		if _, err := store.ListOutbox(ctx, event.TenantID, OutboxFilter{Cursor: OutboxCursor{ID: "missing-time"}}); !errors.Is(err, ErrInvalidOutboxFilter) {
			t.Fatalf("ListOutbox(invalid) error = %v, want ErrInvalidOutboxFilter", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})
}

func TestSQLStoreListMethodsPropagateCloseErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("journal entries", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, _, _, earning := sqlStoreCommitFixture(t)
		closeErr := errors.New("journal rows close failed")
		rows := sqlmock.NewRows([]string{
			"tenant_id", "id", "earning_id", "kind", "currency", "amount_minor", "created_at", "payload",
		}).AddRow(
			earning.TenantID.String(), "journal-close-error", earning.ID, string(JournalKindAccrual), earning.Amount.Currency, earning.Amount.Minor, earning.CreatedAt, "{",
		).CloseError(closeErr)
		query := fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND earning_id = %s ORDER BY created_at, id",
			journalSelectColumns, store.tables.Journals, store.placeholder(1), store.placeholder(2),
		)
		mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(earning.TenantID.String(), earning.ID).WillReturnRows(rows)

		_, err := store.ListJournalEntries(ctx, earning.TenantID, earning.ID)
		if !errors.Is(err, ErrInvalidEvent) || !errors.Is(err, closeErr) {
			t.Fatalf("ListJournalEntries() error = %v, want decode and close errors", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("outbox", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, _, _, earning := sqlStoreCommitFixture(t)
		closeErr := errors.New("outbox rows close failed")
		rows := sqlmock.NewRows([]string{
			"tenant_id", "id", "type", "aggregate_id", "created_at", "published_at", "payload",
		}).AddRow(
			earning.TenantID.String(), "outbox-close-error", "commission.earning.available", earning.ID, earning.CreatedAt, nil, "{",
		).CloseError(closeErr)
		query := fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s ORDER BY created_at, id",
			outboxSelectColumns, store.tables.Outbox, store.placeholder(1),
		)
		mock.ExpectQuery(regexp.QuoteMeta(query)).WithArgs(earning.TenantID.String()).WillReturnRows(rows)

		_, err := store.ListOutbox(ctx, earning.TenantID, OutboxFilter{})
		if !errors.Is(err, ErrInvalidOutbox) || !errors.Is(err, closeErr) {
			t.Fatalf("ListOutbox() error = %v, want decode and close errors", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})
}

func TestSQLStoreCoverageControlPlaneMutations(t *testing.T) {
	ctx := context.Background()

	t.Run("creates and transitions a template", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, _, template, _ := sqlStoreCommitFixture(t)
		template.Status = TemplateStatusDraft

		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Templates, templateSelectColumns, store.placeholders(6, 1),
		))).WithArgs(
			template.ID, template.Version, string(template.Status), template.CreatedAt, template.UpdatedAt, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		if err := store.CreateTemplate(ctx, template); err != nil {
			t.Fatalf("CreateTemplate() error = %v", err)
		}

		transitionAt := template.UpdatedAt.Add(time.Minute)
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE id = %s AND version = %s FOR UPDATE",
			templateSelectColumns, store.tables.Templates, store.placeholder(1), store.placeholder(2),
		))).WithArgs(template.ID, template.Version).WillReturnRows(templateRows(template))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"UPDATE %s SET status = %s, updated_at = %s, payload = %s WHERE id = %s AND version = %s AND status = %s",
			store.tables.Templates,
			store.placeholder(1), store.placeholder(2), store.placeholder(3), store.placeholder(4), store.placeholder(5), store.placeholder(6),
		))).WithArgs(
			string(TemplateStatusActive), transitionAt, sqlmock.AnyArg(), template.ID, template.Version, string(TemplateStatusDraft),
		).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		updated, err := store.TransitionTemplate(ctx, template.ID, template.Version, TemplateActionActivate, transitionAt)
		if err != nil {
			t.Fatalf("TransitionTemplate() error = %v", err)
		}
		if updated.Status != TemplateStatusActive || !updated.UpdatedAt.Equal(transitionAt) {
			t.Fatalf("TransitionTemplate() = %+v, want active at %s", updated, transitionAt)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("creates a program only after locking its template", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, program, template, _ := sqlStoreCommitFixture(t)
		program.Status = ProgramStatusDraft
		program.Version = 1

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
			programSelectColumns, store.tables.Programs, store.placeholder(1), store.placeholder(2),
		))).WithArgs(program.TenantID.String(), program.ID).WillReturnError(sql.ErrNoRows)
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE id = %s AND version = %s FOR UPDATE",
			templateSelectColumns, store.tables.Templates, store.placeholder(1), store.placeholder(2),
		))).WithArgs(template.ID, template.Version).WillReturnRows(templateRows(template))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Programs, programSelectColumns, store.placeholders(9, 1),
		))).WithArgs(
			program.TenantID.String(), program.ID, program.TemplateID, program.TemplateVersion, string(program.Status), program.Version,
			program.CreatedAt, program.UpdatedAt, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		if err := store.CreateProgram(ctx, program); err != nil {
			t.Fatalf("CreateProgram() error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("creates and updates an attribution with CAS", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, program, _, earning := sqlStoreCommitFixture(t)
		attribution := Attribution{
			TenantID:    program.TenantID,
			ProgramID:   program.ID,
			Slot:        "referrer",
			Beneficiary: program.Rules[0].Beneficiary,
			Active:      true,
			CreatedAt:   earning.CreatedAt,
			UpdatedAt:   earning.UpdatedAt,
		}

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND program_id = %s AND slot = %s FOR UPDATE",
			attributionColumns, store.tables.Attributions, store.placeholder(1), store.placeholder(2), store.placeholder(3),
		))).WithArgs(attribution.TenantID.String(), attribution.ProgramID, attribution.Slot).WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Attributions, attributionColumns, store.placeholders(10, 1),
		))).WithArgs(
			attribution.TenantID.String(), attribution.ProgramID, attribution.Slot,
			string(attribution.Beneficiary.Kind), attribution.Beneficiary.ID, attribution.Active, int64(1),
			attribution.CreatedAt, attribution.UpdatedAt, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		created, err := store.SetAttribution(ctx, attribution, 0)
		if err != nil {
			t.Fatalf("SetAttribution(create) error = %v", err)
		}
		if created.Version != 1 {
			t.Fatalf("SetAttribution(create) version = %d, want 1", created.Version)
		}

		current := created
		updatedAt := current.UpdatedAt.Add(time.Minute)
		updatedInput := current
		updatedInput.Beneficiary = BeneficiaryRef{Kind: BeneficiaryKindExternal, ID: "external-2"}
		updatedInput.Active = false
		updatedInput.UpdatedAt = updatedAt
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND program_id = %s AND slot = %s FOR UPDATE",
			attributionColumns, store.tables.Attributions, store.placeholder(1), store.placeholder(2), store.placeholder(3),
		))).WithArgs(current.TenantID.String(), current.ProgramID, current.Slot).WillReturnRows(sqlCoverageAttributionRows(t, current))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"UPDATE %s SET beneficiary_kind = %s, beneficiary_id = %s, active = %s, version = %s, updated_at = %s, payload = %s WHERE tenant_id = %s AND program_id = %s AND slot = %s AND version = %s",
			store.tables.Attributions,
			store.placeholder(1), store.placeholder(2), store.placeholder(3), store.placeholder(4), store.placeholder(5), store.placeholder(6),
			store.placeholder(7), store.placeholder(8), store.placeholder(9), store.placeholder(10),
		))).WithArgs(
			string(updatedInput.Beneficiary.Kind), updatedInput.Beneficiary.ID, updatedInput.Active, int64(2), updatedAt, sqlmock.AnyArg(),
			current.TenantID.String(), current.ProgramID, current.Slot, current.Version,
		).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		updated, err := store.SetAttribution(ctx, updatedInput, current.Version)
		if err != nil {
			t.Fatalf("SetAttribution(update) error = %v", err)
		}
		if updated.Version != 2 || updated.CreatedAt != current.CreatedAt || updated.Active {
			t.Fatalf("SetAttribution(update) = %+v, want version 2 preserving creation time", updated)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})
}

func TestSQLStoreCoverageEarningAndSettlementMutations(t *testing.T) {
	ctx := context.Background()

	t.Run("manual earning transition appends financial facts", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, _, _, earning := sqlStoreCommitFixture(t)
		now := earning.UpdatedAt.Add(time.Minute)
		updated := earning
		updated.Status = EarningStatusHeld
		updated.Version++
		updated.UpdatedAt = now
		journal := JournalEntry{
			ID:        journalID(updated.ID, JournalKindHeld, updated.Version),
			TenantID:  updated.TenantID,
			EarningID: updated.ID,
			Kind:      JournalKindHeld,
			Amount:    updated.Amount,
			CreatedAt: now,
		}
		eventType := "commission.earning." + string(updated.Status)
		outbox := OutboxEvent{
			ID:          sqlScopedOutboxID(updated.TenantID, updated.ID, eventType, updated.Version),
			TenantID:    updated.TenantID,
			Type:        eventType,
			AggregateID: updated.ID,
			CreatedAt:   now,
		}

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
			earningSelectColumns, store.tables.Earnings, store.placeholder(1), store.placeholder(2),
		))).WithArgs(earning.TenantID.String(), earning.ID).WillReturnRows(earningRows(earning))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"UPDATE %s SET status = %s, version = %s, updated_at = %s, payload = %s WHERE tenant_id = %s AND id = %s AND version = %s",
			store.tables.Earnings,
			store.placeholder(1), store.placeholder(2), store.placeholder(3), store.placeholder(4), store.placeholder(5), store.placeholder(6), store.placeholder(7),
		))).WithArgs(
			string(updated.Status), updated.Version, now, sqlmock.AnyArg(), updated.TenantID.String(), updated.ID, earning.Version,
		).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT id FROM %s WHERE id = %s FOR UPDATE", store.tables.Journals, store.placeholder(1),
		))).WithArgs(journal.ID).WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Journals, journalSelectColumns, store.placeholders(8, 1),
		))).WithArgs(
			journal.TenantID.String(), journal.ID, journal.EarningID, string(journal.Kind), journal.Amount.Currency, journal.Amount.Minor, journal.CreatedAt, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Outbox, outboxSelectColumns, store.placeholders(7, 1),
		))).WithArgs(
			outbox.TenantID.String(), outbox.ID, outbox.Type, outbox.AggregateID, outbox.CreatedAt, nil, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		got, err := store.TransitionEarning(ctx, earning.TenantID, earning.ID, earning.Version, EarningActionHold, now)
		if err != nil {
			t.Fatalf("TransitionEarning() error = %v", err)
		}
		if got != updated {
			t.Fatalf("TransitionEarning() = %+v, want %+v", got, updated)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("due earnings are bounded and append available facts", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, _, _, earning := sqlStoreCommitFixture(t)
		now := earning.AvailableAt.Add(time.Hour)
		pending := earning
		pending.Status = EarningStatusPending
		pending.AvailableAt = now.Add(-time.Minute)
		pending.UpdatedAt = pending.CreatedAt
		available := pending
		available.Status = EarningStatusAvailable
		available.Version++
		available.UpdatedAt = now
		journal := JournalEntry{
			ID:        journalID(available.ID, JournalKindAvailable, available.Version),
			TenantID:  available.TenantID,
			EarningID: available.ID,
			Kind:      JournalKindAvailable,
			Amount:    available.Amount,
			CreatedAt: now,
		}
		eventType := "commission.earning." + string(available.Status)
		outbox := OutboxEvent{
			ID:          sqlScopedOutboxID(available.TenantID, available.ID, eventType, available.Version),
			TenantID:    available.TenantID,
			Type:        eventType,
			AggregateID: available.ID,
			CreatedAt:   now,
		}

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE status = %s AND available_at <= %s ORDER BY available_at, id LIMIT %s FOR UPDATE",
			earningSelectColumns, store.tables.Earnings, store.placeholder(1), store.placeholder(2), store.placeholder(3),
		))).WithArgs(string(EarningStatusPending), now, 1).WillReturnRows(earningRows(pending))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"UPDATE %s SET status = %s, version = %s, updated_at = %s, payload = %s WHERE tenant_id = %s AND id = %s AND version = %s",
			store.tables.Earnings,
			store.placeholder(1), store.placeholder(2), store.placeholder(3), store.placeholder(4), store.placeholder(5), store.placeholder(6), store.placeholder(7),
		))).WithArgs(
			string(available.Status), available.Version, now, sqlmock.AnyArg(), available.TenantID.String(), available.ID, pending.Version,
		).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT id FROM %s WHERE id = %s FOR UPDATE", store.tables.Journals, store.placeholder(1),
		))).WithArgs(journal.ID).WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Journals, journalSelectColumns, store.placeholders(8, 1),
		))).WithArgs(
			journal.TenantID.String(), journal.ID, journal.EarningID, string(journal.Kind), journal.Amount.Currency, journal.Amount.Minor, journal.CreatedAt, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Outbox, outboxSelectColumns, store.placeholders(7, 1),
		))).WithArgs(
			outbox.TenantID.String(), outbox.ID, outbox.Type, outbox.AggregateID, outbox.CreatedAt, nil, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		got, err := store.MakeAvailableDue(ctx, now, 1)
		if err != nil {
			t.Fatalf("MakeAvailableDue() error = %v", err)
		}
		if !reflect.DeepEqual(got, []Earning{available}) {
			t.Fatalf("MakeAvailableDue() = %+v, want %+v", got, []Earning{available})
		}
		if _, err := store.MakeAvailableDue(ctx, now, 0); !errors.Is(err, ErrInvalidEarning) {
			t.Fatalf("MakeAvailableDue(invalid) error = %v, want ErrInvalidEarning", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("starts and finishes a settlement atomically", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, _, _, earning := sqlStoreCommitFixture(t)
		settlement := sqlCoverageSettlement(earning, earning.CreatedAt.Add(time.Minute))
		expectedVersions := map[string]int64{earning.ID: earning.Version}
		claimed := earning
		claimed.Status = EarningStatusSettling
		claimed.Version++
		claimed.UpdatedAt = settlement.CreatedAt
		submittedEvent := OutboxEvent{
			ID:          sqlScopedOutboxID(settlement.TenantID, settlement.ID, "commission.settlement.submitted", settlement.Version),
			TenantID:    settlement.TenantID,
			Type:        "commission.settlement.submitted",
			AggregateID: settlement.ID,
			CreatedAt:   settlement.CreatedAt,
		}

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
			settlementColumns, store.tables.Settlements, store.placeholder(1), store.placeholder(2),
		))).WithArgs(settlement.TenantID.String(), settlement.ID).WillReturnError(sql.ErrNoRows)
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
			earningSelectColumns, store.tables.Earnings, store.placeholder(1), store.placeholder(2),
		))).WithArgs(earning.TenantID.String(), earning.ID).WillReturnRows(earningRows(earning))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Settlements, settlementColumns, store.placeholders(12, 1),
		))).WithArgs(
			settlement.TenantID.String(), settlement.ID, string(settlement.Beneficiary.Kind), settlement.Beneficiary.ID,
			settlement.Amount.Currency, settlement.Amount.Minor, string(settlement.Status), settlement.ProviderReference,
			settlement.Version, settlement.CreatedAt, settlement.UpdatedAt, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"UPDATE %s SET status = %s, version = %s, updated_at = %s, payload = %s WHERE tenant_id = %s AND id = %s AND version = %s",
			store.tables.Earnings,
			store.placeholder(1), store.placeholder(2), store.placeholder(3), store.placeholder(4), store.placeholder(5), store.placeholder(6), store.placeholder(7),
		))).WithArgs(
			string(claimed.Status), claimed.Version, claimed.UpdatedAt, sqlmock.AnyArg(), claimed.TenantID.String(), claimed.ID, earning.Version,
		).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (tenant_id, settlement_id, earning_id) VALUES (%s)", store.tables.SettlementItems, store.placeholders(3, 1),
		))).WithArgs(settlement.TenantID.String(), settlement.ID, earning.ID).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Outbox, outboxSelectColumns, store.placeholders(7, 1),
		))).WithArgs(
			submittedEvent.TenantID.String(), submittedEvent.ID, submittedEvent.Type, submittedEvent.AggregateID, submittedEvent.CreatedAt, nil, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		started, err := store.StartSettlement(ctx, settlement, expectedVersions)
		if err != nil {
			t.Fatalf("StartSettlement() error = %v", err)
		}
		if !reflect.DeepEqual(started, settlement) {
			t.Fatalf("StartSettlement() = %+v, want %+v", started, settlement)
		}

		finishAt := settlement.UpdatedAt.Add(time.Minute)
		finishedEarning := claimed
		finishedEarning.Status = EarningStatusSettled
		finishedEarning.Version++
		finishedEarning.UpdatedAt = finishAt
		settlementJournal := JournalEntry{
			ID:        journalID(finishedEarning.ID, JournalKindSettlement, finishedEarning.Version),
			TenantID:  finishedEarning.TenantID,
			EarningID: finishedEarning.ID,
			Kind:      JournalKindSettlement,
			Amount:    finishedEarning.Amount,
			CreatedAt: finishAt,
		}
		earningEventType := "commission.earning." + string(finishedEarning.Status)
		earningEvent := OutboxEvent{
			ID:          sqlScopedOutboxID(finishedEarning.TenantID, finishedEarning.ID, earningEventType, finishedEarning.Version),
			TenantID:    finishedEarning.TenantID,
			Type:        earningEventType,
			AggregateID: finishedEarning.ID,
			CreatedAt:   finishAt,
		}
		finished := settlement
		finished.Status = SettlementStatusSettled
		finished.ProviderReference = "provider-settlement-1"
		finished.Version++
		finished.UpdatedAt = finishAt
		finishedEventType := "commission.settlement." + string(finished.Status)
		finishedEvent := OutboxEvent{
			ID:          sqlScopedOutboxID(finished.TenantID, finished.ID, finishedEventType, finished.Version),
			TenantID:    finished.TenantID,
			Type:        finishedEventType,
			AggregateID: finished.ID,
			CreatedAt:   finishAt,
		}

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
			settlementColumns, store.tables.Settlements, store.placeholder(1), store.placeholder(2),
		))).WithArgs(settlement.TenantID.String(), settlement.ID).WillReturnRows(sqlCoverageSettlementRows(t, settlement))
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
			earningSelectColumns, store.tables.Earnings, store.placeholder(1), store.placeholder(2),
		))).WithArgs(claimed.TenantID.String(), claimed.ID).WillReturnRows(earningRows(claimed))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"UPDATE %s SET status = %s, version = %s, updated_at = %s, payload = %s WHERE tenant_id = %s AND id = %s AND version = %s",
			store.tables.Earnings,
			store.placeholder(1), store.placeholder(2), store.placeholder(3), store.placeholder(4), store.placeholder(5), store.placeholder(6), store.placeholder(7),
		))).WithArgs(
			string(finishedEarning.Status), finishedEarning.Version, finishAt, sqlmock.AnyArg(), finishedEarning.TenantID.String(), finishedEarning.ID, claimed.Version,
		).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT id FROM %s WHERE id = %s FOR UPDATE", store.tables.Journals, store.placeholder(1),
		))).WithArgs(settlementJournal.ID).WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Journals, journalSelectColumns, store.placeholders(8, 1),
		))).WithArgs(
			settlementJournal.TenantID.String(), settlementJournal.ID, settlementJournal.EarningID, string(settlementJournal.Kind), settlementJournal.Amount.Currency, settlementJournal.Amount.Minor, settlementJournal.CreatedAt, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Outbox, outboxSelectColumns, store.placeholders(7, 1),
		))).WithArgs(
			earningEvent.TenantID.String(), earningEvent.ID, earningEvent.Type, earningEvent.AggregateID, earningEvent.CreatedAt, nil, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"UPDATE %s SET status = %s, provider_reference = %s, version = %s, updated_at = %s, payload = %s WHERE tenant_id = %s AND id = %s AND version = %s",
			store.tables.Settlements,
			store.placeholder(1), store.placeholder(2), store.placeholder(3), store.placeholder(4), store.placeholder(5), store.placeholder(6), store.placeholder(7), store.placeholder(8),
		))).WithArgs(
			string(finished.Status), finished.ProviderReference, finished.Version, finishAt, sqlmock.AnyArg(), finished.TenantID.String(), finished.ID, settlement.Version,
		).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"INSERT INTO %s (%s) VALUES (%s)", store.tables.Outbox, outboxSelectColumns, store.placeholders(7, 1),
		))).WithArgs(
			finishedEvent.TenantID.String(), finishedEvent.ID, finishedEvent.Type, finishedEvent.AggregateID, finishedEvent.CreatedAt, nil, sqlmock.AnyArg(),
		).WillReturnResult(sqlmock.NewResult(1, 1))
		mock.ExpectCommit()
		completed, err := store.FinishSettlement(ctx, settlement.TenantID, settlement.ID, settlement.Version, true, finished.ProviderReference, finishAt)
		if err != nil {
			t.Fatalf("FinishSettlement() error = %v", err)
		}
		if !reflect.DeepEqual(completed, finished) {
			t.Fatalf("FinishSettlement() = %+v, want %+v", completed, finished)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})
}

func TestSQLStoreCoverageMarkOutboxPublished(t *testing.T) {
	ctx := context.Background()

	t.Run("publishes an unpublished event and accepts a repeated delivery", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, _, _, earning := sqlStoreCommitFixture(t)
		event := OutboxEvent{
			ID:          "outbox-publish-1",
			TenantID:    earning.TenantID,
			Type:        "commission.earning.available",
			AggregateID: earning.ID,
			Payload:     map[string]string{"earning_id": earning.ID},
			CreatedAt:   earning.CreatedAt,
		}
		publishedAt := event.CreatedAt.Add(time.Minute)

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
			outboxSelectColumns, store.tables.Outbox, store.placeholder(1), store.placeholder(2),
		))).WithArgs(event.TenantID.String(), event.ID).WillReturnRows(sqlCoverageOutboxRows(t, event))
		mock.ExpectExec(regexp.QuoteMeta(fmt.Sprintf(
			"UPDATE %s SET published_at = %s, payload = %s WHERE tenant_id = %s AND id = %s AND published_at IS NULL",
			store.tables.Outbox, store.placeholder(1), store.placeholder(2), store.placeholder(3), store.placeholder(4),
		))).WithArgs(publishedAt, sqlmock.AnyArg(), event.TenantID.String(), event.ID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()
		if err := store.MarkOutboxPublished(ctx, event.TenantID, event.ID, publishedAt); err != nil {
			t.Fatalf("MarkOutboxPublished() error = %v", err)
		}

		event.PublishedAt = &publishedAt
		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
			outboxSelectColumns, store.tables.Outbox, store.placeholder(1), store.placeholder(2),
		))).WithArgs(event.TenantID.String(), event.ID).WillReturnRows(sqlCoverageOutboxRows(t, event))
		mock.ExpectCommit()
		if err := store.MarkOutboxPublished(ctx, event.TenantID, event.ID, publishedAt.Add(time.Minute)); err != nil {
			t.Fatalf("MarkOutboxPublished(repeated) error = %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})

	t.Run("maps missing rows and rejects invalid input before opening a transaction", func(t *testing.T) {
		store, mock, db := newMockCommissionStore(t)
		defer func() { _ = db.Close() }()
		_, _, _, earning := sqlStoreCommitFixture(t)
		now := earning.CreatedAt.Add(time.Minute)

		mock.ExpectBegin()
		mock.ExpectQuery(regexp.QuoteMeta(fmt.Sprintf(
			"SELECT %s FROM %s WHERE tenant_id = %s AND id = %s FOR UPDATE",
			outboxSelectColumns, store.tables.Outbox, store.placeholder(1), store.placeholder(2),
		))).WithArgs(earning.TenantID.String(), "missing-outbox").WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()
		if err := store.MarkOutboxPublished(ctx, earning.TenantID, "missing-outbox", now); !errors.Is(err, ErrOutboxNotFound) {
			t.Fatalf("MarkOutboxPublished(missing) error = %v, want ErrOutboxNotFound", err)
		}
		if err := store.MarkOutboxPublished(ctx, earning.TenantID, "", now); !errors.Is(err, ErrInvalidOutbox) {
			t.Fatalf("MarkOutboxPublished(invalid) error = %v, want ErrInvalidOutbox", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatalf("unmet SQL expectations: %v", err)
		}
	})
}

func sqlCoverageAttributionRows(t *testing.T, attribution Attribution) *sqlmock.Rows {
	t.Helper()
	payload, err := marshalPayload(attribution)
	if err != nil {
		t.Fatalf("marshal attribution payload: %v", err)
	}
	return sqlmock.NewRows([]string{
		"tenant_id", "program_id", "slot", "beneficiary_kind", "beneficiary_id", "active", "version", "created_at", "updated_at", "payload",
	}).AddRow(
		attribution.TenantID.String(), attribution.ProgramID, attribution.Slot,
		string(attribution.Beneficiary.Kind), attribution.Beneficiary.ID, attribution.Active, attribution.Version,
		attribution.CreatedAt, attribution.UpdatedAt, payload,
	)
}

func sqlCoverageJournalRows(t *testing.T, journal JournalEntry) *sqlmock.Rows {
	t.Helper()
	payload, err := marshalPayload(journal)
	if err != nil {
		t.Fatalf("marshal journal payload: %v", err)
	}
	return sqlmock.NewRows([]string{
		"tenant_id", "id", "earning_id", "kind", "currency", "amount_minor", "created_at", "payload",
	}).AddRow(
		journal.TenantID.String(), journal.ID, journal.EarningID, string(journal.Kind), journal.Amount.Currency, journal.Amount.Minor, journal.CreatedAt, payload,
	)
}

func sqlCoverageOutboxRows(t *testing.T, event OutboxEvent) *sqlmock.Rows {
	t.Helper()
	payload, err := marshalPayload(event)
	if err != nil {
		t.Fatalf("marshal outbox payload: %v", err)
	}
	var publishedAt any
	if event.PublishedAt != nil {
		publishedAt = *event.PublishedAt
	}
	return sqlmock.NewRows([]string{
		"tenant_id", "id", "type", "aggregate_id", "created_at", "published_at", "payload",
	}).AddRow(
		event.TenantID.String(), event.ID, event.Type, event.AggregateID, event.CreatedAt, publishedAt, payload,
	)
}

func sqlCoverageSettlementRows(t *testing.T, settlement Settlement) *sqlmock.Rows {
	t.Helper()
	payload, err := marshalPayload(settlement)
	if err != nil {
		t.Fatalf("marshal settlement payload: %v", err)
	}
	return sqlmock.NewRows([]string{
		"tenant_id", "id", "beneficiary_kind", "beneficiary_id", "currency", "amount_minor", "status", "provider_reference", "version", "created_at", "updated_at", "payload",
	}).AddRow(
		settlement.TenantID.String(), settlement.ID, string(settlement.Beneficiary.Kind), settlement.Beneficiary.ID,
		settlement.Amount.Currency, settlement.Amount.Minor, string(settlement.Status), settlement.ProviderReference,
		settlement.Version, settlement.CreatedAt, settlement.UpdatedAt, payload,
	)
}

func sqlCoverageSettlement(earning Earning, now time.Time) Settlement {
	return Settlement{
		ID:          "settlement-coverage-1",
		TenantID:    earning.TenantID,
		Beneficiary: earning.Beneficiary,
		Amount:      earning.Amount,
		EarningIDs:  []string{earning.ID},
		Status:      SettlementStatusSubmitted,
		Version:     1,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
