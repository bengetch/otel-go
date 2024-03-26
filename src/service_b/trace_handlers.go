package main

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"log"
	"time"
)

type NoOpSpanExporter struct{}

func (e *NoOpSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	return nil
}

func (e *NoOpSpanExporter) Shutdown(ctx context.Context) error { return nil }

func GetTraceProvider(exporterType string, otelExporterOtlpEndpoint string) (*sdktrace.TracerProvider, error) {
	/*
		resolve which trace provider to use from the environment variable SPAN_EXPORTER. If
		this variable is not set, a NoOp exporter is used
	*/

	var traceExporter sdktrace.SpanExporter
	var err error
	if exporterType == "stdout" {
		traceExporter, err = stdouttrace.New(
			stdouttrace.WithPrettyPrint())
	} else if exporterType == "otel" {
		if otelExporterOtlpEndpoint == "" {
			if otelExporterOtlpEndpoint == "" {
				log.Fatal("Failed to configure TracerProvider: SPAN_EXPORTER set to `otel` but OTEL_EXPORTER_OTLP_ENDPOINT is empty")
			}
		}
		traceExporter, err = otlptrace.New(context.Background(), otlptracegrpc.NewClient(
			otlptracegrpc.WithInsecure(),
			otlptracegrpc.WithEndpoint(otelExporterOtlpEndpoint),
		))
	} else {
		// if no exporter type is indicated, no spans will be exported
		traceExporter = &NoOpSpanExporter{}
	}
	if err != nil {
		return nil, err
	}

	traceProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter, sdktrace.WithBatchTimeout(time.Second)),
		sdktrace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(ServiceName),
		)),
	)
	otel.SetTracerProvider(traceProvider)

	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
		),
	)

	return traceProvider, nil
}
