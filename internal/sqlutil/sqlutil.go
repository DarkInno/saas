package sqlutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Dialect controls SQL placeholder rendering.
type Dialect string

const (
	// DialectMySQL uses question-mark placeholders.
	DialectMySQL Dialect = "mysql"

	// DialectSQLite uses question-mark placeholders.
	DialectSQLite Dialect = "sqlite"

	// DialectPostgres uses numbered placeholders.
	DialectPostgres Dialect = "postgres"
)

// NormalizeDialect validates and defaults a SQL dialect.
func NormalizeDialect(dialect Dialect) (Dialect, bool) {
	switch dialect {
	case "":
		return DialectMySQL, true
	case DialectMySQL, DialectSQLite, DialectPostgres:
		return dialect, true
	default:
		return "", false
	}
}

// Placeholder returns a placeholder for a one-based parameter index.
func Placeholder(dialect Dialect, index int) string {
	if dialect == DialectPostgres {
		return fmt.Sprintf("$%d", index)
	}
	return "?"
}

// Placeholders returns count placeholders starting at a one-based parameter index.
func Placeholders(dialect Dialect, count int, start int) string {
	if count <= 0 {
		return ""
	}

	parts := make([]string, count)
	for i := range parts {
		parts[i] = Placeholder(dialect, start+i)
	}
	return strings.Join(parts, ", ")
}

// IsSafeQualifiedIdentifier reports whether value is safe to interpolate as a table name.
func IsSafeQualifiedIdentifier(value string) bool {
	if value == "" {
		return false
	}

	parts := strings.Split(value, ".")
	for _, part := range parts {
		if !IsSafeIdentifier(part) {
			return false
		}
	}
	return true
}

// IsSafeIdentifier reports whether value is safe to interpolate as a SQL identifier.
func IsSafeIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for i, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' {
			continue
		}
		if i > 0 && r >= '0' && r <= '9' {
			continue
		}
		return false
	}
	return true
}

// MarshalStringMap encodes a string map as JSON object text.
func MarshalStringMap(values map[string]string) (string, error) {
	if values == nil {
		values = map[string]string{}
	}
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// UnmarshalStringMap decodes a JSON object text into a string map.
func UnmarshalStringMap(raw string) (map[string]string, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]string{}, nil
	}

	values := map[string]string{}
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil, err
	}
	return values, nil
}

// NormalizeDuplicateKeyError maps common driver duplicate-key messages.
func NormalizeDuplicateKeyError(err error, duplicate error) error {
	if err == nil {
		return nil
	}
	if IsDuplicateKeyError(err) {
		return duplicate
	}
	return err
}

// IsDuplicateKeyError detects duplicate-key errors across common SQL drivers.
func IsDuplicateKeyError(err error) bool {
	for err != nil {
		message := strings.ToLower(err.Error())
		switch {
		case strings.Contains(message, "duplicate") && strings.Contains(message, "key"):
			return true
		case strings.Contains(message, "duplicate") && strings.Contains(message, "entry"):
			return true
		case strings.Contains(message, "unique constraint"):
			return true
		case strings.Contains(message, "constraint failed") && strings.Contains(message, "unique"):
			return true
		case strings.Contains(message, "primary key constraint"):
			return true
		}
		if joined, ok := err.(interface{ Unwrap() []error }); ok {
			for _, child := range joined.Unwrap() {
				if IsDuplicateKeyError(child) {
					return true
				}
			}
			return false
		}
		err = errors.Unwrap(err)
	}
	return false
}
