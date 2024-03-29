package main

import (
	"context"
	"fmt"
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
	"time"

	"github.com/gin-gonic/gin"
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

	router := gin.Default()
	router.Use(otelgin.Middleware(ServiceName))
	router.GET("/", hello)
	router.POST("/basicRequest", basicRequest)
	router.POST("/chainedRequest", chainedRequest)
	router.POST("/chainedAsyncRequest", chainedAsyncRequest)

	err := router.Run("0.0.0.0:5000")
	if err != nil {
		log.Printf("Failed to start the server: %v\n", err)
		os.Exit(1)
	}
}

func hello(c *gin.Context) {

	_, childSpan := tracer.Start(c.Request.Context(), "span-service-a-hello")
	defer childSpan.End()

	// increment meter that tracks requests to `/` API of this service
	helloRequestCount.Add(c.Request.Context(), 1)

	c.IndentedJSON(http.StatusOK, gin.H{"message": "Hello from Service A"})
}

type BasicPayload struct {
	Message string `json:"message"`
	Number  int    `json:"number"`
}

func basicRequest(c *gin.Context) {

	_, childSpan := tracer.Start(c.Request.Context(), "span-service-a-basic-request")
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
		"message": fmt.Sprintf("service A received number %v from Entrypoint service", payload.Number),
	})
}

func chainedRequest(c *gin.Context) {

	_, childSpan := tracer.Start(c.Request.Context(), "span-service-a-chained-request")
	defer childSpan.End()

	var payload BasicPayload
	if err := c.BindJSON(&payload); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "invalid request payload",
			"error":   err.Error(),
		})
		return
	}

	requestToB := BasicPayload{
		Message: "hello to B from A and also Entrypoint",
		Number:  payload.Number + rand.Intn(11),
	}

	api := "/chainedRequest"
	responseField := "number"
	response, err := makeRequest(c, &requestToB, fmt.Sprintf("http://service_b:5000%s", api), "POST", responseField)
	if err == nil {
		if response != "" {
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("number from service A, from service B: %s", response),
			})
		} else {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": fmt.Sprintf("response from %s API of service B did not contain a `%s` key", api, responseField),
			})
		}
	}
}

func makeAsyncRequest(c *gin.Context, payload *BasicPayload) {

	api := "/chainedRequest"
	responseField := "number"
	response, err := makeRequest(c, payload, fmt.Sprintf("http://service_b:5000%s", api), "POST", responseField)

	// wait 10 seconds to ensure that below logs fire after response has been sent
	time.Sleep(10 * time.Second)

	if err == nil {
		if response != "" {
			log.Printf("number from service B:  %s", response)
		} else {
			log.Printf("response from %s API of service B did not contain a %s key", api, responseField)
		}
	}
}

func chainedAsyncRequest(c *gin.Context) {

	_, childSpan := tracer.Start(c.Request.Context(), "span-service-a-chained-async-request")
	defer childSpan.End()

	var payload BasicPayload
	if err := c.BindJSON(&payload); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "invalid request payload",
			"error":   err.Error(),
		})
		return
	}

	requestToB := BasicPayload{
		Message: "asynchronous hello to B from A and also Entrypoint",
		Number:  payload.Number + rand.Intn(11),
	}
	// TODO: need to defer childSpan.End() to when this async call completes, not just the body of this function
	go makeAsyncRequest(c, &requestToB)

	c.IndentedJSON(http.StatusOK, gin.H{
		"message": "successfully sent asynchronous message to service B",
	})
}
