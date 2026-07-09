package sqlxtenant

import (
	"context"
	"strings"

	"github.com/DarkInno/gotenancy/data"
)

// Query adds the tenant filter to a base SQL statement.
func Query(ctx context.Context, baseSQL string, opts ...data.FilterOption) (string, []any, error) {
	return QueryWithArgs(ctx, baseSQL, nil, opts...)
}

// TenantCondition returns a parameterized tenant condition for explicit placement in complex SQL.
//
// Use this helper for JOIN, GROUP BY, ORDER BY, LIMIT, or locking queries where Query cannot
// safely rewrite SQL. The returned condition is empty for host contexts.
func TenantCondition(ctx context.Context, opts ...data.FilterOption) (data.Condition, error) {
	filter, err := data.NewFilter(ctx, opts...)
	if err != nil {
		return data.Condition{}, err
	}
	condition := filter.Condition()
	condition.Args = append([]any(nil), condition.Args...)
	return condition, nil
}

// QueryWithArgs adds the tenant filter to a base SQL statement and preserves existing args.
func QueryWithArgs(ctx context.Context, baseSQL string, baseArgs []any, opts ...data.FilterOption) (string, []any, error) {
	baseSQL = strings.TrimSpace(baseSQL)
	if baseSQL == "" {
		return "", nil, ErrUnsafeSQL
	}

	filter, err := data.NewFilter(ctx, opts...)
	if err != nil {
		return "", nil, err
	}
	if filter.IsHost() {
		return baseSQL, append([]any(nil), baseArgs...), nil
	}
	if err := validateTenantRewriteSQL(baseSQL); err != nil {
		return "", nil, err
	}

	condition := filter.Condition()
	if condition.Empty() {
		return baseSQL, append([]any(nil), baseArgs...), nil
	}

	keyword := " WHERE "
	if containsWhere(baseSQL) {
		keyword = " AND "
	}
	args := append([]any(nil), baseArgs...)
	args = append(args, condition.Args...)
	return baseSQL + keyword + condition.Expression, args, nil
}

func containsWhere(sql string) bool {
	return strings.Contains(paddedSQL(sql), " WHERE ")
}

func validateTenantRewriteSQL(sql string) error {
	if strings.Contains(sql, ";") ||
		strings.Contains(sql, "--") ||
		strings.Contains(sql, "/*") ||
		strings.Contains(sql, "*/") {
		return ErrUnsafeSQL
	}

	normalized := strings.Join(strings.Fields(sql), " ")
	upper := strings.ToUpper(normalized)
	fields := strings.Fields(upper)
	if len(fields) == 0 {
		return ErrUnsafeSQL
	}
	switch fields[0] {
	case "SELECT":
		if !strings.Contains(paddedSQL(normalized), " FROM ") {
			return ErrUnsafeSQL
		}
	case "UPDATE", "DELETE":
	default:
		return ErrUnsafeSQL
	}

	if strings.HasSuffix(upper, " WHERE") ||
		strings.HasSuffix(upper, " AND") ||
		strings.HasSuffix(upper, " OR") {
		return ErrUnsafeSQL
	}

	for _, phrase := range unsafeTenantRewritePhrases {
		if strings.Contains(paddedSQL(normalized), phrase) {
			return ErrUnsafeSQL
		}
	}
	return nil
}

func paddedSQL(sql string) string {
	return " " + strings.ToUpper(strings.Join(strings.Fields(sql), " ")) + " "
}

var unsafeTenantRewritePhrases = []string{
	" JOIN ",
	" UNION ",
	" INTERSECT ",
	" EXCEPT ",
	" RETURNING ",
	" ORDER BY ",
	" GROUP BY ",
	" HAVING ",
	" LIMIT ",
	" OFFSET ",
	" FETCH ",
	" FOR UPDATE ",
	" FOR SHARE ",
}
