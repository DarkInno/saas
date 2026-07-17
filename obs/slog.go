package obs

import (
	"context"
	"log/slog"
)

// SlogAttrs returns tenant observability attributes for slog.
func SlogAttrs(ctx context.Context) []slog.Attr {
	tenantID, tenantSide, deploymentUnitID := fieldValues(ctx)
	if tenantID == "" && tenantSide == "" && deploymentUnitID == "" {
		return nil
	}

	attrs := make([]slog.Attr, 0, 3)
	if tenantID != "" {
		attrs = append(attrs, slog.String(TenantIDField, tenantID))
	}
	if tenantSide != "" {
		attrs = append(attrs, slog.String(TenantSideField, tenantSide))
	}
	if deploymentUnitID != "" {
		attrs = append(attrs, slog.String(DeploymentUnitIDField, deploymentUnitID))
	}
	return attrs
}

// RedactSlogAttrs returns a redacted copy of attrs.
func RedactSlogAttrs(attrs ...slog.Attr) []slog.Attr {
	if len(attrs) == 0 {
		return nil
	}

	redacted := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		redacted = append(redacted, redactSlogAttr(attr))
	}
	return redacted
}

// LoggerWithTenant returns logger enriched with tenant fields from ctx.
func LoggerWithTenant(ctx context.Context, logger *slog.Logger) *slog.Logger {
	if logger == nil {
		logger = slog.Default()
	}

	attrs := SlogAttrs(ctx)
	if len(attrs) == 0 {
		return logger
	}
	return logger.With(attrsToArgs(attrs)...)
}

// LogAttrs logs attrs with tenant fields from ctx and redacts sensitive attrs.
func LogAttrs(ctx context.Context, logger *slog.Logger, level slog.Level, msg string, attrs ...slog.Attr) {
	if logger == nil {
		logger = slog.Default()
	}

	tenantAttrs := SlogAttrs(ctx)
	logAttrs := make([]slog.Attr, 0, len(tenantAttrs)+len(attrs))
	logAttrs = append(logAttrs, tenantAttrs...)
	logAttrs = append(logAttrs, RedactSlogAttrs(attrs...)...)
	logger.LogAttrs(ctx, level, msg, logAttrs...)
}

func attrsToArgs(attrs []slog.Attr) []any {
	args := make([]any, len(attrs))
	for i, attr := range attrs {
		args[i] = attr
	}
	return args
}

func redactSlogAttr(attr slog.Attr) slog.Attr {
	if isSensitiveKey(attr.Key) {
		return slog.String(attr.Key, redactedValue)
	}

	value := attr.Value.Resolve()
	if value.Kind() == slog.KindGroup {
		return slog.Group(attr.Key, attrsToArgs(RedactSlogAttrs(value.Group()...))...)
	}
	attr.Value = value
	return attr
}
