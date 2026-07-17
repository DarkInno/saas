package sqlutil

import (
	"errors"
	"reflect"
	"testing"
)

func TestDialectHelpers(t *testing.T) {
	tests := []struct {
		name        string
		dialect     Dialect
		wantDialect Dialect
		wantOK      bool
		wantOne     string
		wantMany    string
	}{
		{name: "default", dialect: "", wantDialect: DialectMySQL, wantOK: true, wantOne: "?", wantMany: "?, ?, ?"},
		{name: "mysql", dialect: DialectMySQL, wantDialect: DialectMySQL, wantOK: true, wantOne: "?", wantMany: "?, ?, ?"},
		{name: "sqlite", dialect: DialectSQLite, wantDialect: DialectSQLite, wantOK: true, wantOne: "?", wantMany: "?, ?, ?"},
		{name: "postgres", dialect: DialectPostgres, wantDialect: DialectPostgres, wantOK: true, wantOne: "$3", wantMany: "$3, $4, $5"},
		{name: "unsupported", dialect: "oracle", wantDialect: "", wantOK: false, wantOne: "?", wantMany: "?, ?, ?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotDialect, gotOK := NormalizeDialect(tt.dialect)
			if gotDialect != tt.wantDialect || gotOK != tt.wantOK {
				t.Fatalf("NormalizeDialect(%q) = %q, %v; want %q, %v", tt.dialect, gotDialect, gotOK, tt.wantDialect, tt.wantOK)
			}
			if got := Placeholder(tt.dialect, 3); got != tt.wantOne {
				t.Fatalf("Placeholder(%q, 3) = %q, want %q", tt.dialect, got, tt.wantOne)
			}
			if got := Placeholders(tt.dialect, 3, 3); got != tt.wantMany {
				t.Fatalf("Placeholders(%q, 3, 3) = %q, want %q", tt.dialect, got, tt.wantMany)
			}
		})
	}

	for _, count := range []int{-1, 0} {
		if got := Placeholders(DialectPostgres, count, 1); got != "" {
			t.Fatalf("Placeholders(postgres, %d, 1) = %q, want empty", count, got)
		}
	}
}

func TestSafeIdentifiers(t *testing.T) {
	tests := []struct {
		value string
		want  bool
	}{
		{value: "tenant_id", want: true},
		{value: "tenant2", want: true},
		{value: "Public.Tenant_2026", want: true},
		{value: "", want: false},
		{value: "2tenant", want: false},
		{value: "tenant-id", want: false},
		{value: "tenant id", want: false},
		{value: "tenant;DROP", want: false},
		{value: "public..tenants", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			if got := IsSafeQualifiedIdentifier(tt.value); got != tt.want {
				t.Fatalf("IsSafeQualifiedIdentifier(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}

	if IsSafeIdentifier("public.tenants") {
		t.Fatal("IsSafeIdentifier(qualified) = true, want false")
	}
}

func TestStringMapSerialization(t *testing.T) {
	raw, err := MarshalStringMap(nil)
	if err != nil {
		t.Fatalf("MarshalStringMap(nil) error = %v", err)
	}
	if raw != "{}" {
		t.Fatalf("MarshalStringMap(nil) = %q, want {}", raw)
	}

	values := map[string]string{"region": "eu", "tier": "pro"}
	raw, err = MarshalStringMap(values)
	if err != nil {
		t.Fatalf("MarshalStringMap() error = %v", err)
	}
	decoded, err := UnmarshalStringMap(raw)
	if err != nil {
		t.Fatalf("UnmarshalStringMap(%q) error = %v", raw, err)
	}
	if !reflect.DeepEqual(decoded, values) {
		t.Fatalf("UnmarshalStringMap(%q) = %#v, want %#v", raw, decoded, values)
	}

	for _, raw := range []string{"", " \t\n"} {
		decoded, err := UnmarshalStringMap(raw)
		if err != nil {
			t.Fatalf("UnmarshalStringMap(%q) error = %v", raw, err)
		}
		if len(decoded) != 0 {
			t.Fatalf("UnmarshalStringMap(%q) = %#v, want empty map", raw, decoded)
		}
	}

	for _, raw := range []string{"{", `{"count":1}`, `[]`} {
		if _, err := UnmarshalStringMap(raw); err == nil {
			t.Fatalf("UnmarshalStringMap(%q) error = nil, want decode error", raw)
		}
	}
}

func TestDuplicateKeyRecognitionAndNormalization(t *testing.T) {
	duplicate := errors.New("duplicate identity")
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "mysql", err: errors.New("Error 1062 (23000): Duplicate entry 'tenant-a' for key 'PRIMARY'"), want: true},
		{name: "postgres", err: errors.New("pq: duplicate key value violates unique constraint"), want: true},
		{name: "sqlite unique", err: errors.New("UNIQUE constraint failed: tenants.id"), want: true},
		{name: "sqlite primary key", err: errors.New("PRIMARY KEY constraint failed"), want: true},
		{name: "wrapped", err: fmtWrap(errors.New("duplicate key value violates unique constraint")), want: true},
		{name: "joined", err: errors.Join(errors.New("rollback failed"), errors.New("Duplicate entry for key")), want: true},
		{name: "ordinary", err: errors.New("connection refused"), want: false},
		{name: "nil", err: nil, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDuplicateKeyError(tt.err); got != tt.want {
				t.Fatalf("IsDuplicateKeyError(%v) = %v, want %v", tt.err, got, tt.want)
			}
			got := NormalizeDuplicateKeyError(tt.err, duplicate)
			if tt.want && !errors.Is(got, duplicate) {
				t.Fatalf("NormalizeDuplicateKeyError(%v) = %v, want duplicate sentinel", tt.err, got)
			}
			if !tt.want && got != tt.err {
				t.Fatalf("NormalizeDuplicateKeyError(%v) = %v, want original error", tt.err, got)
			}
		})
	}
}

type wrappedError struct{ error }

func fmtWrap(err error) error {
	return wrappedError{error: err}
}

func (err wrappedError) Unwrap() error {
	return err.error
}
