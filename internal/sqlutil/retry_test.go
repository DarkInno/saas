package sqlutil

import (
	"errors"
	"testing"
)

func TestIsRetryableTransactionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "postgres serialization SQLSTATE", err: sqlStateError{state: "40001"}, want: true},
		{name: "postgres deadlock SQLSTATE", err: sqlStateError{state: "40P01"}, want: true},
		{name: "postgres lock not available SQLSTATE", err: sqlStateError{state: "55P03"}, want: true},
		{name: "postgres serialization message", err: errors.New("pq: could not serialize access due to concurrent update"), want: true},
		{name: "postgres lock timeout message", err: errors.New("pq: canceling statement due to lock timeout"), want: true},
		{name: "mysql deadlock message", err: errors.New("Error 1213 (40001): Deadlock found when trying to get lock; try restarting transaction"), want: true},
		{name: "joined retryable error", err: errors.Join(errors.New("rollback failed"), sqlStateError{state: "40001"}), want: true},
		{name: "ordinary query error", err: errors.New("connection refused"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRetryableTransactionError(tt.err); got != tt.want {
				t.Fatalf("IsRetryableTransactionError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

type sqlStateError struct {
	state string
}

func (err sqlStateError) Error() string {
	return "database error with SQLSTATE " + err.state
}

func (err sqlStateError) SQLState() string {
	return err.state
}
