package otel

import (
	"context"
	"errors"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
)

// SetupOTelSDK initializes the OpenTelemetry SDK with tracing and logs support
func SetupOTelSDK(ctx context.Context, gitSHA, dirty, version string, logger *slog.Logger) (shutdown func(context.Context) error, loggerProvider *sdklog.LoggerProvider, err error) {
	var shutdownFuncs []func(context.Context) error

	shutdown = func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		return err
	}

	config := NewConfig(gitSHA, dirty, version)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	if config.TracesEnabled() {
		traceProvider, err := NewProvider(ctx, config)
		if err != nil {
			return shutdown, nil, err
		}
		shutdownFuncs = append(shutdownFuncs, traceProvider.Shutdown)
		otel.SetTracerProvider(traceProvider.TracerProvider())
		logger.Info("OpenTelemetry tracing enabled", "endpoint", config.TracesEndpoint())
	}

	if config.LogsEnabled() {
		logsProvider, err := NewLogsProvider(ctx, config)
		if err != nil {
			return shutdown, nil, err
		}
		shutdownFuncs = append(shutdownFuncs, logsProvider.Shutdown)
		loggerProvider = logsProvider.LoggerProvider()
		logger.Info("OpenTelemetry logs enabled", "endpoint", config.LogsEndpoint())
	}

	return shutdown, loggerProvider, nil
}
