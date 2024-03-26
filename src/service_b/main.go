package main

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
)

var (
	ServiceName       string
	meter             metric.Meter
	tracer            trace.Tracer
	helloRequestCount metric.Int64Counter
)

func init() {
	initServiceName()
	initTracer()
	initMeter()
	initHelloRequestCount()
}

func initServiceName() {
	ServiceName = os.Getenv("SERVICE_NAME")
	if ServiceName == "" {
		log.Fatal("SERVICE_NAME environment variable not set")
	}
}

func initTracer() {
	tracerName := fmt.Sprintf("%s.tracer", ServiceName)
	tracer = otel.Tracer(tracerName)
}

func initMeter() {
	meterName := fmt.Sprintf("%s.meter", ServiceName)
	meter = otel.Meter(meterName)
}

func initHelloRequestCount() {
	/*
		initialize an int counter meter that tracks the number of requests to the `/` API of this service
	*/

	var err error
	meterName := fmt.Sprintf("%s.hello.requests", ServiceName)

	helloRequestCount, err = meter.Int64Counter(meterName,
		metric.WithDescription("The number of requests to the `/` API"),
	)
	if err != nil {
		log.Fatalf("Failed to initialize %s.hello.requests meter: %v\n", ServiceName, err)
	}
}

func main() {

	otelExporterOtlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	// configure tracer provider
	tp, tpErr := GetTraceProvider(os.Getenv("SPAN_EXPORTER"), otelExporterOtlpEndpoint)
	if tpErr != nil {
		log.Fatalf("Failed to get tracer provider: %v\n", tpErr)
	}
	defer func(tp *sdktrace.TracerProvider, ctx context.Context) {
		err := tp.Shutdown(ctx)
		if err != nil {
			log.Printf("Error while shutting down Tracer provider: %v\n", err)
		}
	}(tp, context.Background())

	// configure meter provider
	mp, mpErr := GetMetricProvider(os.Getenv("METER_EXPORTER"), otelExporterOtlpEndpoint)
	if mpErr != nil {
		log.Fatalf("Failed to get metric provider: %v\n", mpErr)
	}
	defer func(mp *sdkmetric.MeterProvider, ctx context.Context) {
		err := mp.Shutdown(ctx)
		if err != nil {
			log.Printf("Error while shutting down Metric provider: %v\n", err)
		}
	}(mp, context.Background())

	// configure gin server and ass otelgin instrumentation
	router := gin.Default()
	router.Use(otelgin.Middleware(ServiceName))

	// configure gin server API
	router.GET("/", hello)
	router.POST("/basicRequest", basicRequest)
	router.POST("/chainedRequest", chainedRequest)

	err := router.Run("0.0.0.0:5000")
	if err != nil {
		log.Printf("Failed to start the server: %v\n", err)
		os.Exit(1)
	}
}

func hello(c *gin.Context) {

	_, childSpan := tracer.Start(c.Request.Context(), "span-service-b-hello")
	defer childSpan.End()

	// increment meter that tracks requests to `/` API of this service
	helloRequestCount.Add(c.Request.Context(), 1)

	c.IndentedJSON(http.StatusOK, gin.H{"message": "hello from Service B"})
}

type BasicPayload struct {
	Message string `json:"message"`
	Number  int    `json:"number"`
}

func basicRequest(c *gin.Context) {

	_, childSpan := tracer.Start(c.Request.Context(), "span-service-b-basic-request")
	defer childSpan.End()

	var payload BasicPayload
	if err := c.BindJSON(&payload); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "invalid request payload",
			"error":   err.Error()},
		)
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{
		"message": fmt.Sprintf("service B received number %v from Entrypoint service", payload.Number),
	})
}

func chainedRequest(c *gin.Context) {

	_, childSpan := tracer.Start(c.Request.Context(), "span-service-b-chained-request")
	defer childSpan.End()

	var payload BasicPayload
	if err := c.BindJSON(&payload); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "invalid request payload",
			"error":   err.Error()},
		)
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{
		"message": "hello to A, and also to Entrypoint",
		"number":  strconv.Itoa(payload.Number + rand.Intn(11)),
	})
}
