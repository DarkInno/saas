package obs

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	tenantctx "github.com/DarkInno/saas/core/context"
	"github.com/DarkInno/saas/core/types"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestFields(t *testing.T) {
	tenantFields := Fields(tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"}))
	if tenantFields[TenantIDField] != "tenant-a" || tenantFields[TenantSideField] != tenantSide {
		t.Fatalf("tenant Fields() = %#v, want tenant-a tenant side", tenantFields)
	}

	hostFields := Fields(tenantctx.WithHost(context.Background()))
	if _, ok := hostFields[TenantIDField]; ok {
		t.Fatalf("host Fields() tenant id = %q, want absent", hostFields[TenantIDField])
	}
	if hostFields[TenantSideField] != hostSide {
		t.Fatalf("host Fields() = %#v, want host side", hostFields)
	}

	empty := Fields(context.Background())
	if len(empty) != 0 {
		t.Fatalf("background Fields() = %#v, want empty", empty)
	}
}

func TestRedact(t *testing.T) {
	input := map[string]string{"tenant_id": "tenant-a", "api-key": "secret", "Password": "pw"}
	got := Redact(input)
	if got["tenant_id"] != "tenant-a" {
		t.Fatalf("Redact() tenant_id = %q, want tenant-a", got["tenant_id"])
	}
	if got["api-key"] != redactedValue || got["Password"] != redactedValue {
		t.Fatalf("Redact() = %#v, want sensitive fields redacted", got)
	}

	got["tenant_id"] = "changed"
	if input["tenant_id"] != "tenant-a" {
		t.Fatal("Redact() mutated input")
	}
}

func TestSlogAttrs(t *testing.T) {
	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})
	attrs := SlogAttrs(ctx)
	if len(attrs) != 2 {
		t.Fatalf("SlogAttrs() len = %d, want 2", len(attrs))
	}
	if attrs[0].Key != TenantIDField || attrs[0].Value.String() != "tenant-a" {
		t.Fatalf("SlogAttrs()[0] = %#v, want tenant_id tenant-a", attrs[0])
	}
	if attrs[1].Key != TenantSideField || attrs[1].Value.String() != tenantSide {
		t.Fatalf("SlogAttrs()[1] = %#v, want tenant side", attrs[1])
	}
}

func TestRedactSlogAttrs(t *testing.T) {
	attrs := RedactSlogAttrs(
		slog.String("authorization", "Bearer token"),
		slog.Group("nested", slog.String("refresh-token", "secret"), slog.String("safe", "value")),
		slog.Any("lazy", sensitiveLogValue{}),
	)
	if attrs[0].Value.String() != redactedValue {
		t.Fatalf("RedactSlogAttrs()[0] = %#v, want redacted", attrs[0])
	}

	group := attrs[1].Value.Group()
	if group[0].Value.String() != redactedValue || group[1].Value.String() != "value" {
		t.Fatalf("RedactSlogAttrs() group = %#v, want nested sensitive redacted", group)
	}

	lazyGroup := attrs[2].Value.Group()
	if lazyGroup[0].Value.String() != redactedValue || lazyGroup[1].Value.String() != "value" {
		t.Fatalf("RedactSlogAttrs() log valuer group = %#v, want sensitive redacted", lazyGroup)
	}
}

func TestLogAttrsAddsTenantAndRedacts(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})

	LogAttrs(ctx, logger, slog.LevelInfo, "created", slog.String("api_key", "secret"), slog.String("safe", "value"))

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if record[TenantIDField] != "tenant-a" || record[TenantSideField] != tenantSide {
		t.Fatalf("LogAttrs() tenant fields = %#v", record)
	}
	if record["api_key"] != redactedValue || record["safe"] != "value" {
		t.Fatalf("LogAttrs() fields = %#v, want redacted api_key and safe value", record)
	}
}

func TestLoggerWithTenant(t *testing.T) {
	var buf bytes.Buffer
	base := slog.New(slog.NewJSONHandler(&buf, nil))
	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})

	LoggerWithTenant(ctx, base).InfoContext(context.Background(), "ready")

	var record map[string]any
	if err := json.Unmarshal(buf.Bytes(), &record); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if record[TenantIDField] != "tenant-a" || record[TenantSideField] != tenantSide {
		t.Fatalf("LoggerWithTenant() record = %#v", record)
	}
}

func TestSpanAttributes(t *testing.T) {
	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})
	attrs := SpanAttributes(ctx)
	assertAttribute(t, attrs, TenantIDField, "tenant-a")
	assertAttribute(t, attrs, TenantSideField, tenantSide)
}

func TestAddSpanAttributes(t *testing.T) {
	span := &recordingSpan{recording: true}
	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})
	ctx = trace.ContextWithSpan(ctx, span)

	AddSpanAttributes(ctx)

	assertAttribute(t, span.attrs, TenantIDField, "tenant-a")
	assertAttribute(t, span.attrs, TenantSideField, tenantSide)
}

func TestStartSpanAddsTenantAttributes(t *testing.T) {
	tracer := &recordingTracer{}
	ctx := tenantctx.WithTenant(context.Background(), types.Tenant{ID: "tenant-a"})

	_, span := StartSpan(ctx, tracer, "tenant.operation")
	span.End()

	if tracer.name != "tenant.operation" {
		t.Fatalf("StartSpan() name = %q, want tenant.operation", tracer.name)
	}
	config := trace.NewSpanStartConfig(tracer.opts...)
	assertAttribute(t, config.Attributes(), TenantIDField, "tenant-a")
	assertAttribute(t, config.Attributes(), TenantSideField, tenantSide)
}

func TestRecordSpanError(t *testing.T) {
	span := &recordingSpan{recording: true}
	ctx := trace.ContextWithSpan(context.Background(), span)
	err := sensitiveTestError{}

	RecordSpanError(ctx, err, "")

	if span.err == nil || span.err.Error() != defaultErrorDescription {
		t.Fatalf("RecordSpanError() err = %v, want sanitized default description", span.err)
	}
	if span.err.Error() == err.Error() {
		t.Fatalf("RecordSpanError() recorded raw error text")
	}
	if span.status != codes.Error || span.statusDescription != defaultErrorDescription {
		t.Fatalf("RecordSpanError() status = %v %q, want error default description", span.status, span.statusDescription)
	}
	assertAttribute(t, span.eventAttrs, errorTypeAttribute, "obs.sensitiveTestError")
}

func assertAttribute(t *testing.T, attrs []attribute.KeyValue, key, value string) {
	t.Helper()
	for _, attr := range attrs {
		if string(attr.Key) == key {
			if attr.Value.AsString() != value {
				t.Fatalf("attribute %s = %q, want %q", key, attr.Value.AsString(), value)
			}
			return
		}
	}
	t.Fatalf("attribute %s missing in %#v", key, attrs)
}

type recordingTracer struct {
	noop.Tracer

	name string
	opts []trace.SpanStartOption
	span *recordingSpan
}

func (tracer *recordingTracer) Start(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	tracer.name = name
	tracer.opts = append([]trace.SpanStartOption(nil), opts...)
	tracer.span = &recordingSpan{recording: true}
	return trace.ContextWithSpan(ctx, tracer.span), tracer.span
}

type recordingSpan struct {
	noop.Span

	recording         bool
	attrs             []attribute.KeyValue
	eventAttrs        []attribute.KeyValue
	err               error
	status            codes.Code
	statusDescription string
}

func (span *recordingSpan) IsRecording() bool {
	return span.recording
}

func (span *recordingSpan) SetAttributes(attrs ...attribute.KeyValue) {
	span.attrs = append(span.attrs, attrs...)
}

func (span *recordingSpan) RecordError(err error, opts ...trace.EventOption) {
	span.err = err
	config := trace.NewEventConfig(opts...)
	span.eventAttrs = append(span.eventAttrs, config.Attributes()...)
}

func (span *recordingSpan) SetStatus(code codes.Code, description string) {
	span.status = code
	span.statusDescription = description
}

type sensitiveTestError struct{}

func (sensitiveTestError) Error() string {
	return "password=secret"
}

type sensitiveLogValue struct{}

func (sensitiveLogValue) LogValue() slog.Value {
	return slog.GroupValue(slog.String("api_key", "secret"), slog.String("safe", "value"))
}
