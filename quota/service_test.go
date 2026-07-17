package quota

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestWaitForQuotaMutationRetryHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := waitForQuotaMutationRetry(ctx, 4); !errors.Is(err, context.Canceled) {
		t.Fatalf("waitForQuotaMutationRetry() error = %v, want context.Canceled", err)
	}
}

func TestRetryQuotaMutation(t *testing.T) {
	retryable := errors.New("pq: could not serialize access due to concurrent update")

	t.Run("retries and succeeds", func(t *testing.T) {
		attempts := 0
		waits := 0
		used, err := retryQuotaMutation(context.Background(), func() (int64, error) {
			attempts++
			if attempts == 1 {
				return 0, retryable
			}
			return 7, nil
		}, func(context.Context, int) error {
			waits++
			return nil
		})
		if err != nil {
			t.Fatalf("retryQuotaMutation() error = %v", err)
		}
		if used != 7 || attempts != 2 || waits != 1 {
			t.Fatalf("retryQuotaMutation() = used %d, attempts %d, waits %d; want 7, 2, 1", used, attempts, waits)
		}
	})

	t.Run("returns original error after retry exhaustion", func(t *testing.T) {
		attempts := 0
		waits := 0
		used, err := retryQuotaMutation(context.Background(), func() (int64, error) {
			attempts++
			return 0, retryable
		}, func(context.Context, int) error {
			waits++
			return nil
		})
		if err != retryable {
			t.Fatalf("retryQuotaMutation() error = %v, want original retryable error", err)
		}
		if used != 0 || attempts != quotaMutationMaxAttempts || waits != quotaMutationMaxAttempts-1 {
			t.Fatalf("retryQuotaMutation() = used %d, attempts %d, waits %d; want 0, %d, %d", used, attempts, waits, quotaMutationMaxAttempts, quotaMutationMaxAttempts-1)
		}
	})

	t.Run("cancels while waiting to retry", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		waitStarted := make(chan struct{})
		result := make(chan error, 1)
		go func() {
			_, err := retryQuotaMutation(ctx, func() (int64, error) {
				return 0, retryable
			}, func(ctx context.Context, _ int) error {
				close(waitStarted)
				<-ctx.Done()
				return ctx.Err()
			})
			result <- err
		}()

		select {
		case <-waitStarted:
		case <-time.After(time.Second):
			t.Fatal("retryQuotaMutation() did not begin retry backoff")
		}
		cancel()
		select {
		case err := <-result:
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("retryQuotaMutation() error = %v, want context.Canceled", err)
			}
		case <-time.After(time.Second):
			t.Fatal("retryQuotaMutation() did not stop after context cancellation")
		}
	})
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

func TestSQLStoreAddZeroSkipsMySQLNoopUpdate(t *testing.T) {
	state := &quotaSQLTestState{used: 7}
	db := sql.OpenDB(quotaSQLTestConnector{state: state})
	t.Cleanup(func() { _ = db.Close() })
	store, err := NewSQLStore(db)
	if err != nil {
		t.Fatalf("NewSQLStore() error = %v", err)
	}

	used, err := store.Add(context.Background(), "tenant-a", "api", PeriodDay, 0)
	if err != nil {
		t.Fatalf("Add(0) error = %v", err)
	}
	if used != 7 {
		t.Fatalf("Add(0) used = %d, want 7", used)
	}
	if state.execs != 0 {
		t.Fatalf("Add(0) UPDATE calls = %d, want zero", state.execs)
	}
}

type quotaSQLTestState struct {
	used  int64
	execs int
}

type quotaSQLTestConnector struct {
	state *quotaSQLTestState
}

func (connector quotaSQLTestConnector) Connect(context.Context) (driver.Conn, error) {
	connection := quotaSQLTestConn(connector)
	return &connection, nil
}

func (connector quotaSQLTestConnector) Driver() driver.Driver {
	return quotaSQLTestDriver(connector)
}

type quotaSQLTestDriver struct {
	state *quotaSQLTestState
}

func (driver quotaSQLTestDriver) Open(string) (driver.Conn, error) {
	connection := quotaSQLTestConn(driver)
	return &connection, nil
}

type quotaSQLTestConn struct {
	state *quotaSQLTestState
}

func (*quotaSQLTestConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare is not supported")
}

func (*quotaSQLTestConn) Close() error { return nil }

func (*quotaSQLTestConn) Begin() (driver.Tx, error) { return quotaSQLTestTx{}, nil }

func (*quotaSQLTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return quotaSQLTestTx{}, nil
}

func (conn *quotaSQLTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &quotaSQLTestRows{used: conn.state.used}, nil
}

func (conn *quotaSQLTestConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	conn.state.execs++
	return driver.RowsAffected(0), nil
}

type quotaSQLTestTx struct{}

func (quotaSQLTestTx) Commit() error   { return nil }
func (quotaSQLTestTx) Rollback() error { return nil }

type quotaSQLTestRows struct {
	used int64
	done bool
}

func (*quotaSQLTestRows) Columns() []string { return []string{"used"} }
func (*quotaSQLTestRows) Close() error      { return nil }

func (rows *quotaSQLTestRows) Next(dest []driver.Value) error {
	if rows.done {
		return io.EOF
	}
	rows.done = true
	dest[0] = rows.used
	return nil
}
