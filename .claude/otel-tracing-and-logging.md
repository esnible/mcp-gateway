# OpenTelemetry Tracing and Logging

## Current Setup

OTel is configured via environment variables. When no `OTEL_EXPORTER_OTLP_ENDPOINT` is set,
OTel is fully disabled (zero overhead). The setup lives in `internal/otel/`:

- `config.go` -- reads env vars, determines which signals are enabled
- `otel.go` -- `SetupOTelSDK()` wires up propagator, tracer provider, logger provider, and shutdown
- `provider.go` -- creates trace exporter (HTTP or gRPC based on URL scheme)
- `logs.go` -- creates log exporter (same scheme dispatch)
- `logging.go` -- `TracingHandler` injects `trace_id`/`span_id` into slog records; `MultiHandler` fans out to stdout + OTLP
- `resource.go` -- shared resource with `service.name`, `service.version`, `vcs.revision`

The global tracer provider is set in `cmd/mcp-broker-router/main.go` via `otel.SetTracerProvider()`.
The logger is created with `NewTracingLogger()` which wraps slog with trace context injection.

## Tracing Conventions

### Span Naming
All spans use the prefix `mcp-router.` followed by a descriptive name:
- `mcp-router.process` -- root span for the full ext_proc stream lifecycle
- `mcp-router.route-decision` -- routing decision (tool-call vs broker passthrough)
- `mcp-router.tool-call` -- full tool call handling
- `mcp-router.broker-passthrough` -- pass-through to broker
- `mcp-router.broker.get-server-info` -- broker lookup for server info
- `mcp-router.session-cache.get` -- session cache read
- `mcp-router.session-cache.store` -- session cache write
- `mcp-router.session-init` -- hairpin initialize to backend

When adding new spans, follow this naming pattern: `mcp-router.<component>.<action>`.


### Span Attributes
Follow [OpenTelemetry MCP Semantic Conventions](https://opentelemetry.io/docs/specs/semconv/gen-ai/mcp/#server):

- `mcp.method.name` -- MCP method (initialize, tools/call, tools/list)
- `gen_ai.tool.name` -- tool name
- `gen_ai.operation.name` -- same as mcp.method.name
- `mcp.session.id` -- gateway session ID
- `mcp.server` -- backend server name
- `mcp.server.hostname` -- backend server hostname
- `jsonrpc.request.id` -- JSON-RPC request ID
- `jsonrpc.protocol.version` -- always "2.0"
- `http.method`, `http.path`, `http.request_id`, `http.status_code`
- `client.address` -- from x-forwarded-for

For new attributes, check OTel semantic conventions first before inventing custom ones.

### Error Recording
Use the `recordError()` helper for consistent error attributes:

```go
recordError(span, err, 500)
```

This sets `error.type`, `error_source`, and `http.status_code` on the span. For inline
error recording without the helper:

```go
span.RecordError(err)
span.SetStatus(codes.Error, "description")
span.SetAttributes(attribute.String("error.type", "specific_error_type"))
```

### Trace Context Propagation
The router extracts W3C `traceparent` from Envoy headers via `extractTraceContext()`.
This uses `otel.GetTextMapPropagator().Extract()` with a custom `headerCarrier` adapter
for Envoy's `HeaderMap`. Do not manually parse `traceparent` -- use the propagator.

### No-op Span Pattern
In `Process()`, the span is initialized as a no-op via `trace.SpanFromContext(ctx)` and
replaced with a real span when headers arrive. The `defer func() { span.End() }()` closure
captures the variable by reference, so it always ends the correct span. This avoids nil
checks throughout the function.

## Logging Conventions

### Always Use Context-Aware Logging
Use `s.Logger.InfoContext(ctx, ...)` instead of `s.Logger.Info(...)`. The `ctx` parameter
carries the active span, which allows the `TracingHandler` to inject `trace_id` and `span_id`
into log lines automatically.

```go
s.Logger.DebugContext(ctx, "processing request", "tool", toolName)
s.Logger.ErrorContext(ctx, "failed to resolve server", "error", err)
```

### Structured Key-Value Pairs
Use slog's key-value pairs, not `fmt.Sprintf`:

```go
// correct
s.Logger.InfoContext(ctx, "tool resolved", "tool", toolName, "server", serverName)

// avoid
s.Logger.InfoContext(ctx, fmt.Sprintf("tool %s resolved to server %s", toolName, serverName))
```

## Adding OTel to a New Package

1. Create a `tracer()` function with a package-specific tracer name
2. Accept `context.Context` as the first parameter in functions that need tracing
3. Create spans with `tracer().Start(ctx, "package-name.operation")`
4. Use `defer span.End()`
5. Add relevant attributes using OTel semantic conventions
6. Use `s.Logger.InfoContext(ctx, ...)` for log correlation
7. Propagate `ctx` returned by `tracer().Start()` to downstream calls

## Testing Spans

Use `go.opentelemetry.io/otel/sdk/trace/tracetest` (already a dependency) to verify spans:

```go
exporter := tracetest.NewInMemoryExporter()
tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))
otel.SetTracerProvider(tp)
t.Cleanup(func() { otel.SetTracerProvider(prev); tp.Shutdown(ctx) })

// ... run code ...

spans := exporter.GetSpans()
// assert span names, attributes, parent-child relationships
```

See `TestProcessSpanEnded` in `internal/mcp-router/server_test.go` for a working example.
