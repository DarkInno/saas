package saas

import (
	"errors"
	"testing"
)

func TestPublicErrorsAreStableSentinels(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "no tenant", err: ErrNoTenant},
		{name: "forbidden", err: ErrForbidden},
		{name: "invalid state", err: ErrInvalidState},
		{name: "host required", err: ErrHostRequired},
		{name: "tenant mismatch", err: ErrTenantMismatch},
	}

	seen := map[string]string{}
	for _, tt := range tests {
		if tt.err == nil {
			t.Fatalf("%s error is nil", tt.name)
		}
		if !errors.Is(tt.err, tt.err) {
			t.Fatalf("%s error is not a stable sentinel", tt.name)
		}
		if other, ok := seen[tt.err.Error()]; ok {
			t.Fatalf("%s and %s share error text %q", tt.name, other, tt.err.Error())
		}
		seen[tt.err.Error()] = tt.name
	}
}
