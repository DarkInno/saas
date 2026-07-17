package types

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// TenantID is the default public tenant identifier type.
type TenantID string

// TenantIDStrategy describes how incoming tenant identifiers should be
// validated before they are accepted by resolver and store implementations.
type TenantIDStrategy string

const (
	// TenantIDStrategyString accepts any non-empty string.
	TenantIDStrategyString TenantIDStrategy = "string"

	// TenantIDStrategyInt accepts base-10 integer identifiers.
	TenantIDStrategyInt TenantIDStrategy = "int"

	// TenantIDStrategyUUID accepts canonical UUID strings.
	TenantIDStrategyUUID TenantIDStrategy = "uuid"
)

var (
	// ErrEmptyTenantID reports an empty tenant identifier.
	ErrEmptyTenantID = errors.New("saas/types: empty tenant id")

	// ErrInvalidTenantID reports a tenant identifier that does not match the configured strategy.
	ErrInvalidTenantID = errors.New("saas/types: invalid tenant id")
)

// String returns the string form of the tenant identifier.
func (id TenantID) String() string {
	return string(id)
}

// Int64 returns the integer form of an int-strategy tenant identifier.
func (id TenantID) Int64() (int64, error) {
	value, err := strconv.ParseInt(id.String(), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q is not a base-10 int64", ErrInvalidTenantID, id)
	}
	return value, nil
}

// NewTenantIDFromInt creates a tenant identifier from an int64 value.
func NewTenantIDFromInt(value int64) TenantID {
	return TenantID(strconv.FormatInt(value, 10))
}

// ParseTenantID validates raw with the configured strategy and returns a TenantID.
func ParseTenantID(raw string, strategy TenantIDStrategy) (TenantID, error) {
	normalized := strings.TrimSpace(raw)
	if normalized == "" {
		return "", ErrEmptyTenantID
	}

	switch strategy {
	case "", TenantIDStrategyString:
		return TenantID(normalized), nil
	case TenantIDStrategyInt:
		if _, err := strconv.ParseInt(normalized, 10, 64); err != nil {
			return "", fmt.Errorf("%w: %q is not a base-10 int64", ErrInvalidTenantID, raw)
		}
		return TenantID(normalized), nil
	case TenantIDStrategyUUID:
		if !isCanonicalUUID(normalized) {
			return "", fmt.Errorf("%w: %q is not a canonical UUID", ErrInvalidTenantID, raw)
		}
		return TenantID(strings.ToLower(normalized)), nil
	default:
		return "", fmt.Errorf("%w: unsupported tenant id strategy %q", ErrInvalidTenantID, strategy)
	}
}

func isCanonicalUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !isHex(r) {
				return false
			}
		}
	}
	return true
}

func isHex(r rune) bool {
	return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
}
