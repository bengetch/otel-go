package main

import (
	"context"
	"fmt"
	"os"

	"github.com/agoda-com/opentelemetry-go/otelzap"
	"github.com/agoda-com/opentelemetry-logs-go/sdk/logs"
	logshandler "github.com/bengetch/otelhandlers/logs"
	metricshandler "github.com/bengetch/otelhandlers/metrics"
	traceshandler "github.com/bengetch/otelhandlers/traces"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"go.uber.org/zap"
)

func SetupLogs() *logs.LoggerProvider {
	/*
		configure logger provider instance, which is responsible for (1) injecting trace context data into logs
		where applicable and (2) exporting logs to the backend indicated by the LOGS_EXPORTER environment variable
	*/

	lp, lpErr := logshandler.GetLogProvider(os.Getenv("LOGS_EXPORTER"), ServiceName)
	if lpErr != nil {
		otelzap.Ctx(context.Background()).Fatal(
			fmt.Sprintf("Failed to get log provider: %v\n", lpErr),
		)
	} else {
		logger := zap.New(otelzap.NewOtelCore(lp))
		zap.ReplaceGlobals(logger)
	}

	return lp
}

func SetupTraces() *sdktrace.TracerProvider {
	/*
		configure tracer provider instance, which is responsible for exporting traces to the backend indicated by
		the TRACES_EXPORTER environment variable. the text map propagator configured here also ensures that trace
		context is propagated correctly across API calls
	*/

	tp, tpErr := traceshandler.GetTracerProvider(os.Getenv("TRACES_EXPORTER"), ServiceName)
	if tpErr != nil {
		otelzap.Ctx(context.Background()).Fatal(
			fmt.Sprintf("Failed to get tracer provider: %v\n", tpErr),
		)
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

	mp, mpErr := metricshandler.GetMeterProvider(os.Getenv("METRICS_EXPORTER"), ServiceName)
	if mpErr != nil {
		otelzap.Ctx(context.Background()).Fatal(
			fmt.Sprintf("Failed to get metric provider: %v\n", mpErr),
		)
	} else {
		otel.SetMeterProvider(mp)
	}

	return mp
}

func CleanupTelemetryProviders(lp *logs.LoggerProvider, tp *sdktrace.TracerProvider, mp *sdkmetric.MeterProvider) {
	/*
		call shutdown function on all telemetry provider types
	*/

	ctx := context.Background()
	if lpShutdownErr := lp.Shutdown(ctx); lpShutdownErr != nil {
		otelzap.Ctx(context.Background()).Error(
			fmt.Sprintf("error while shutting down logger provider: %v\n", lpShutdownErr),
		)
	}

	if tpShutdownErr := tp.Shutdown(ctx); tpShutdownErr != nil {
		otelzap.Ctx(context.Background()).Error(
			fmt.Sprintf("error while shutting down tracer provider: %v\n", tpShutdownErr),
		)
	}

	if mpShutdownErr := mp.Shutdown(ctx); mpShutdownErr != nil {
		otelzap.Ctx(context.Background()).Error(
			fmt.Sprintf("error while shutting down meter provider: %v\n", mpShutdownErr),
		)
	}
}
