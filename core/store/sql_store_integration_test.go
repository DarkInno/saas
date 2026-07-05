package store_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"gotenancy/core/store"
	"gotenancy/internal/testcontract"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

func TestSQLStoreMySQLIntegration(t *testing.T) {
	dsn := os.Getenv("GOTENANCY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("set GOTENANCY_MYSQL_DSN to run MySQL integration test")
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error = %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := pingUntilReady(ctx, db); err != nil {
		t.Fatalf("mysql not ready: %v", err)
	}
	resetTenantsTable(t, ctx, db)

	testcontract.RunStoreContract(t, func() store.Store {
		sqlStore, err := store.NewSQLStore(db)
		if err != nil {
			t.Fatalf("NewSQLStore() error = %v", err)
		}
		return sqlStore
	})
}

func TestSQLStorePostgresIntegration(t *testing.T) {
	dsn := os.Getenv("GOTENANCY_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set GOTENANCY_POSTGRES_DSN to run PostgreSQL integration test")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("db.Close() error = %v", err)
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := pingUntilReady(ctx, db); err != nil {
		t.Fatalf("postgres not ready: %v", err)
	}
	resetPostgresTenantsTable(t, ctx, db)

	testcontract.RunStoreContract(t, func() store.Store {
		sqlStore, err := store.NewSQLStore(db, store.WithSQLDialect(store.SQLDialectPostgres))
		if err != nil {
			t.Fatalf("NewSQLStore() error = %v", err)
		}
		return sqlStore
	})
}

func pingUntilReady(ctx context.Context, db *sql.DB) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		if err := db.PingContext(ctx); err == nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func resetPostgresTenantsTable(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	statements := []string{
		"DROP TABLE IF EXISTS tenants",
		`CREATE TABLE tenants (
			id VARCHAR(191) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			status VARCHAR(32) NOT NULL,
			plan_id VARCHAR(191) NOT NULL,
			config JSONB NOT NULL
		)`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("exec %q error = %v", statement, err)
		}
	}
}

func resetTenantsTable(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()

	statements := []string{
		"DROP TABLE IF EXISTS tenants",
		`CREATE TABLE tenants (
			id VARCHAR(191) NOT NULL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			status VARCHAR(32) NOT NULL,
			plan_id VARCHAR(191) NOT NULL,
			config JSON NOT NULL
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}
	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("exec %q error = %v", statement, err)
		}
	}
}
