package quota

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

func TestServiceConsumeCheckAndReset(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	limit := Limit{TenantID: "tenant-a", Resource: "api_calls", Limit: 10, Period: PeriodDay}

	usage, err := service.Consume(ctx, limit, 3)
	if err != nil {
		t.Fatalf("Consume(3) error = %v", err)
	}
	if usage.Used != 3 || usage.Limit != 10 {
		t.Fatalf("Consume(3) usage = %+v, want used 3 limit 10", usage)
	}

	if _, err := service.Check(ctx, limit, 8); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("Check(over) error = %v, want ErrQuotaExceeded", err)
	}
	if _, err := service.Consume(ctx, limit, 8); !errors.Is(err, ErrQuotaExceeded) {
		t.Fatalf("Consume(over) error = %v, want ErrQuotaExceeded", err)
	}

	if err := service.Reset(ctx, "tenant-a", "api_calls", PeriodDay); err != nil {
		t.Fatalf("Reset() error = %v", err)
	}
	usage, err = service.Check(ctx, limit, 10)
	if err != nil {
		t.Fatalf("Check(after reset) error = %v", err)
	}
	if usage.Used != 0 {
		t.Fatalf("usage after reset = %d, want 0", usage.Used)
	}
}

func TestServiceValidation(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())

	if _, err := service.Consume(ctx, Limit{}, 1); !errors.Is(err, ErrInvalidQuota) {
		t.Fatalf("Consume(invalid limit) error = %v, want ErrInvalidQuota", err)
	}
	if _, err := service.Consume(ctx, Limit{TenantID: "tenant-a", Resource: "api", Limit: 1, Period: PeriodDay}, -1); !errors.Is(err, ErrInvalidQuota) {
		t.Fatalf("Consume(negative amount) error = %v, want ErrInvalidQuota", err)
	}
	if _, err := NewService(nil).Check(ctx, Limit{TenantID: "tenant-a", Resource: "api", Limit: 1, Period: PeriodDay}, 1); !errors.Is(err, ErrNilStore) {
		t.Fatalf("Check(nil store) error = %v, want ErrNilStore", err)
	}
}

func TestServiceConsumeIsAtomicUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	service := NewService(NewMemoryStore())
	limit := Limit{TenantID: "tenant-a", Resource: "api_calls", Limit: 100, Period: PeriodDay}

	const workers = 200
	var wg sync.WaitGroup
	var successes int64
	unexpected := make(chan error, workers)

	wg.Add(workers)
	for range workers {
		go func() {
			defer wg.Done()

			_, err := service.Consume(ctx, limit, 1)
			if err == nil {
				atomic.AddInt64(&successes, 1)
				return
			}
			if !errors.Is(err, ErrQuotaExceeded) {
				unexpected <- err
			}
		}()
	}
	wg.Wait()
	close(unexpected)

	for err := range unexpected {
		t.Errorf("Consume() unexpected error = %v", err)
	}
	if got := atomic.LoadInt64(&successes); got != limit.Limit {
		t.Fatalf("successful consumes = %d, want %d", got, limit.Limit)
	}

	usage, err := service.Check(ctx, limit, 0)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}
	if usage.Used != limit.Limit {
		t.Fatalf("usage.Used = %d, want %d", usage.Used, limit.Limit)
	}
}

func TestMemoryStoreScopesByTenantResourceAndPeriod(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	if _, err := store.Add(ctx, "tenant-a", "api", PeriodDay, 1); err != nil {
		t.Fatalf("Add tenant-a error = %v", err)
	}
	if _, err := store.Add(ctx, "tenant-b", "api", PeriodDay, 2); err != nil {
		t.Fatalf("Add tenant-b error = %v", err)
	}
	if _, err := store.Add(ctx, "tenant-a", "api", PeriodMonth, 3); err != nil {
		t.Fatalf("Add tenant-a month error = %v", err)
	}

	got, err := store.Get(ctx, "tenant-a", "api", PeriodDay)
	if err != nil {
		t.Fatalf("Get tenant-a day error = %v", err)
	}
	if got != 1 {
		t.Fatalf("tenant-a day usage = %d, want 1", got)
	}
}

func TestNewSQLStoreValidation(t *testing.T) {
	if _, err := NewSQLStore(nil); !errors.Is(err, ErrNilDB) {
		t.Fatalf("NewSQLStore(nil) error = %v, want ErrNilDB", err)
	}

	db := &sql.DB{}
	store, err := NewSQLStore(db)
	if err != nil {
		t.Fatalf("NewSQLStore() error = %v", err)
	}
	if store.table != DefaultSQLTableName {
		t.Fatalf("default table = %q, want %q", store.table, DefaultSQLTableName)
	}

	store, err = NewSQLStore(db, WithTableName("public.saas_quota_usage"), WithSQLDialect(SQLDialectPostgres))
	if err != nil {
		t.Fatalf("NewSQLStore(custom) error = %v", err)
	}
	if store.table != "public.saas_quota_usage" || store.dialect != SQLDialectPostgres {
		t.Fatalf("SQLStore = %+v, want custom table and postgres dialect", store)
	}
	if got := store.placeholders(2, 4); got != "$4, $5" {
		t.Fatalf("postgres placeholders = %q, want $4, $5", got)
	}

	if _, err := NewSQLStore(db, WithTableName("saas_quota_usage;drop")); !errors.Is(err, ErrInvalidTableName) {
		t.Fatalf("NewSQLStore(unsafe table) error = %v, want ErrInvalidTableName", err)
	}
	if _, err := NewSQLStore(db, WithSQLDialect("oracle")); !errors.Is(err, ErrUnsupportedSQLDialect) {
		t.Fatalf("NewSQLStore(unsupported dialect) error = %v, want ErrUnsupportedSQLDialect", err)
	}
}
