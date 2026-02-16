package otel

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"go.opentelemetry.io/otel"
)

func TestSetupOTelSDK_Disabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	shutdown, loggerProvider, err := SetupOTelSDK(context.Background(), "", "", "v1.0.0", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loggerProvider != nil {
		t.Error("expected loggerProvider to be nil when disabled")
	}

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestSetupOTelSDK_TracesEnabled(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	shutdown, _, err := SetupOTelSDK(context.Background(), "abc123", "false", "v1.0.0", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tp := otel.GetTracerProvider()
	if tp == nil {
		t.Error("expected global TracerProvider to be set")
	}

	tracer := otel.Tracer("test")
	if tracer == nil {
		t.Error("expected to get a tracer")
	}

	ctx, span := tracer.Start(context.Background(), "test-span")
	if span == nil {
		t.Error("expected to create a span")
	}
	if ctx == nil {
		t.Error("expected context from span")
	}
	span.End()

	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown error: %v", err)
	}
}

func TestSetupOTelSDK_LogsEnabled(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "http://localhost:4318")
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	shutdown, loggerProvider, err := SetupOTelSDK(context.Background(), "", "", "v1.0.0", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if loggerProvider == nil {
		t.Error("expected loggerProvider to be non-nil when logs are enabled")
	}

	_ = shutdown(context.Background())
}

func TestSetupOTelSDK_PropagatorSet(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	_, _, err := SetupOTelSDK(context.Background(), "", "", "v1.0.0", logger)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		t.Error("expected global TextMapPropagator to be set")
	}

	carrier := make(testCarrier)
	carrier["traceparent"] = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

	ctx := propagator.Extract(context.Background(), carrier)
	if ctx == nil {
		t.Error("expected context from Extract")
	}
}

type testCarrier map[string]string

func (c testCarrier) Get(key string) string {
	return c[key]
}

func (c testCarrier) Set(key, value string) {
	c[key] = value
}

func (c testCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for k := range c {
		keys = append(keys, k)
	}
	return keys
}
