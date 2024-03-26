package main

import (
	"context"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"log"
	"time"
)

type NoOpMetricExporter struct{}

func (e *NoOpMetricExporter) Temporality(sdkmetric.InstrumentKind) metricdata.Temporality { return 0 }

func (e *NoOpMetricExporter) Aggregation(sdkmetric.InstrumentKind) sdkmetric.Aggregation { return nil }

func (e *NoOpMetricExporter) Export(context.Context, *metricdata.ResourceMetrics) error { return nil }

func (e *NoOpMetricExporter) ForceFlush(context.Context) error { return nil }

func (e *NoOpMetricExporter) Shutdown(context.Context) error { return nil }

func GetMetricProvider(exporterType string, otelExporterOtlpEndpoint string) (*sdkmetric.MeterProvider, error) {

	var metricExporter sdkmetric.Exporter
	var err error

	if exporterType == "stdout" {
		metricExporter, err = stdoutmetric.New(
			stdoutmetric.WithPrettyPrint())
	} else if exporterType == "otel" {
		if otelExporterOtlpEndpoint == "" {
			log.Fatal("Failed to configure TracerProvider: METER_EXPORTER set to `otel` but OTEL_EXPORTER_OTLP_ENDPOINT is empty")
		}
		metricExporter, err = otlpmetricgrpc.New(context.Background(),
			otlpmetricgrpc.WithInsecure(),
			otlpmetricgrpc.WithEndpoint(otelExporterOtlpEndpoint),
		)
	} else {
		metricExporter = &NoOpMetricExporter{}
	}
	if err != nil {
		return nil, err
	}

	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(10*time.Second))),
		sdkmetric.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(ServiceName),
		)),
	)
	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}
