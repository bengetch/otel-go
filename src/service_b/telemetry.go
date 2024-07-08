package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	logshandler "github.com/bengetch/otelhandlers/logs"
	metricshandler "github.com/bengetch/otelhandlers/metrics"
	traceshandler "github.com/bengetch/otelhandlers/traces"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func SetupLogs() *sdklog.LoggerProvider {
	lp, lpErr := logshandler.GetProvider(os.Getenv("LOGS_EXPORTER"), ServiceName)
	if lpErr != nil {
		slog.ErrorContext(context.Background(), fmt.Sprintf("Failed to get log provider: %v\n", lpErr))
	} else {
		logger := otelslog.NewLogger(
			ServiceName,
			otelslog.WithLoggerProvider(lp),
		)
		slog.SetDefault(logger)
	}

	return lp
}

func SetupTraces() *sdktrace.TracerProvider {
	/*
		configure tracer provider instance, which is responsible for exporting traces to the backend indicated by
		the TRACES_EXPORTER environment variable. the text map propagator configured here also ensures that trace
		context is propagated correctly across API calls
	*/

	tp, tpErr := traceshandler.GetProvider(os.Getenv("TRACES_EXPORTER"), ServiceName)
	if tpErr != nil {
		slog.ErrorContext(context.Background(), fmt.Sprintf("Failed to get tracer provider: %v\n", tpErr))
	} else {
		otel.SetTracerProvider(tp)
		textPropagator := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{})
		otel.SetTextMapPropagator(textPropagator)
	}

	return tp
}

func SetupMetrics() *sdkmetric.MeterProvider {
	/*
		configure meter provider instance, which is responsible for exporting metrics to the backend indicated by
		the METRICS_EXPORTER environment variable
	*/

	mp, mpErr := metricshandler.GetProvider(os.Getenv("METRICS_EXPORTER"), ServiceName)
	if mpErr != nil {
		slog.ErrorContext(context.Background(), fmt.Sprintf("Failed to get metric provider: %v\n", mpErr))
	} else {
		otel.SetMeterProvider(mp)
	}

	return mp
}

func CleanupTelemetryProviders(lp *sdklog.LoggerProvider, tp *sdktrace.TracerProvider, mp *sdkmetric.MeterProvider) {
	/*
		call shutdown function on all telemetry provider types
	*/

	ctx := context.Background()
	if lpShutdownErr := lp.Shutdown(ctx); lpShutdownErr != nil {
		slog.ErrorContext(
			context.Background(),
			fmt.Sprintf("error while shutting down logger provider: %v\n", lpShutdownErr),
		)
	}

	if tpShutdownErr := tp.Shutdown(ctx); tpShutdownErr != nil {
		slog.ErrorContext(
			context.Background(),
			fmt.Sprintf("error while shutting down tracer provider: %v\n", tpShutdownErr),
		)
	}

	if mpShutdownErr := mp.Shutdown(ctx); mpShutdownErr != nil {
		slog.ErrorContext(
			context.Background(),
			fmt.Sprintf("error while shutting down meter provider: %v\n", mpShutdownErr),
		)
	}
}
