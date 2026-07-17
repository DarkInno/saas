package obs

import (
	"context"
	"errors"
	"reflect"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// InstrumentationName is the OpenTelemetry instrumentation scope name for SaaS.
	InstrumentationName = "github.com/DarkInno/saas"

	defaultErrorDescription = "operation failed"

	errorTypeAttribute = "error.type"
)

// NewTracer returns an OpenTelemetry tracer for SaaS.
//
// Libraries should not initialize an SDK or exporter. When provider is nil, the
// process-global OpenTelemetry provider is used, which is no-op until the host
// application configures it.
func NewTracer(provider trace.TracerProvider) trace.Tracer {
	if provider == nil {
		return otel.Tracer(InstrumentationName)
	}
	return provider.Tracer(InstrumentationName)
}

// SpanAttributes returns tenant OpenTelemetry span attributes for ctx.
func SpanAttributes(ctx context.Context) []attribute.KeyValue {
	tenantID, tenantSide := fieldValues(ctx)
	if tenantID == "" && tenantSide == "" {
		return nil
	}

	attrs := make([]attribute.KeyValue, 0, 2)
	if tenantID != "" {
		attrs = append(attrs, attribute.String(TenantIDField, tenantID))
	}
	if tenantSide != "" {
		attrs = append(attrs, attribute.String(TenantSideField, tenantSide))
	}
	return attrs
}

// AddSpanAttributes adds tenant attributes to the current span in ctx.
func AddSpanAttributes(ctx context.Context) {
	attrs := SpanAttributes(ctx)
	if len(attrs) == 0 {
		return
	}

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}
	span.SetAttributes(attrs...)
}

// StartSpan starts a span with tenant attributes from ctx.
//
// The caller is responsible for ending the returned span.
func StartSpan(ctx context.Context, tracer trace.Tracer, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	if tracer == nil {
		tracer = NewTracer(nil)
	}

	attrs := SpanAttributes(ctx)
	if len(attrs) > 0 {
		startOpts := make([]trace.SpanStartOption, 0, len(opts)+1)
		startOpts = append(startOpts, trace.WithAttributes(attrs...))
		startOpts = append(startOpts, opts...)
		opts = startOpts
	}
	return tracer.Start(ctx, name, opts...)
}

// RecordSpanError records a sanitized error event on the current span and marks it as failed.
func RecordSpanError(ctx context.Context, err error, description string) {
	if err == nil {
		return
	}

	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return
	}

	if description == "" {
		description = defaultErrorDescription
	}
	span.RecordError(errors.New(description), trace.WithAttributes(attribute.String(errorTypeAttribute, errorType(err))))
	span.SetStatus(codes.Error, description)
}

func errorType(err error) string {
	if err == nil {
		return ""
	}
	return reflect.TypeOf(err).String()
}
