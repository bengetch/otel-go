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
	router.GET("/basicA", callServiceA)
	router.GET("/basicB", callServiceB)
	router.GET("/chainedA", chainedCallServiceA)
	router.GET("/chainedAsyncA", chainedAsyncCallServiceA)

	err := router.Run("0.0.0.0:5000")
	if err != nil {
		log.Printf("Failed to start the server: %v\n", err)
		os.Exit(1)
	}
}

func hello(c *gin.Context) {

	_, childSpan := tracer.Start(c.Request.Context(), "span-entrypoint-hello")
	defer childSpan.End()

	// increment meter that tracks requests to `/` API of this service
	helloRequestCount.Add(c.Request.Context(), 1)

	c.IndentedJSON(http.StatusOK, gin.H{"message": "hello from Entrypoint service"})
}

type BasicPayload struct {
	Message string `json:"message"`
	Number  int    `json:"number"`
}

func callServiceA(c *gin.Context) {
	/*
		send a hello message and a random number to service A, return response from A to client
	*/

	_, childSpan := tracer.Start(c.Request.Context(), "span-entrypoint-call-service-a")
	defer childSpan.End()

	requestToA := BasicPayload{
		Message: "hello to A",
		Number:  rand.Intn(11),
	}

	api := "/basicRequest"
	responseField := "message"
	response, err := makeRequest(c, &requestToA, fmt.Sprintf("http://service_a:5000%s", api), "POST", responseField)
	if err == nil {
		if response != "" {
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("message from service A: %s", response),
			})
		} else {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("response from %s API of service A did not contain a `%s` key", api, responseField),
			})
		}
	}
}

func callServiceB(c *gin.Context) {
	/*
		send a hello message and a random number to service B, return response from B to client
	*/

	_, childSpan := tracer.Start(c.Request.Context(), "span-entrypoint-call-service-b")
	defer childSpan.End()

	requestToB := BasicPayload{
		Message: "Hello to B",
		Number:  rand.Intn(11),
	}

	api := "/basicRequest"
	responseField := "message"
	response, err := makeRequest(c, &requestToB, fmt.Sprintf("http://service_b:5000%s", api), "POST", responseField)
	if err == nil {
		if response != "" {
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("message from service B: %s", response),
			})
		} else {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": fmt.Sprintf("response from %s API of service B did not contain a `%s` key", api, responseField),
			})
		}
	}
}

func chainedCallServiceA(c *gin.Context) {
	/*
		send a hello message and a random number to service A, which sends the same to service B.
		service A waits for a response from service B, and relays that response back before it is
		returned to the client
	*/

	_, childSpan := tracer.Start(c.Request.Context(), "span-entrypoint-chained-call-service-a")
	defer childSpan.End()

	requestToA := BasicPayload{
		Message: "hello to A, and also to B",
		Number:  rand.Intn(11),
	}

	api := "/chainedRequest"
	responseField := "message"
	response, err := makeRequest(c, &requestToA, fmt.Sprintf("http://service_a:5000%s", api), "POST", responseField)
	if err == nil {
		if response != "" {
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("message from service A, from service B: %s", response),
			})
		} else {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": fmt.Sprintf("response from %s API of service A did not contain a `%s` key", api, responseField),
			})
		}
	}
}

func chainedAsyncCallServiceA(c *gin.Context) {
	/*
		send a hello message and a random number to service A, which sends the same to service B.
		service A does not wait for a response from service B before sending its response.
	*/

	_, childSpan := tracer.Start(c.Request.Context(), "span-entrypoint-chained-async-call-service-a")
	defer childSpan.End()

	requestToA := BasicPayload{
		Message: "asynchronous hello to A, and also to B",
		Number:  rand.Intn(11),
	}

	api := "/chainedAsyncRequest"
	responseField := "message"
	response, err := makeRequest(c, &requestToA, fmt.Sprintf("http://service_a:5000%s", api), "POST", responseField)
	if err == nil {
		if response != "" {
			c.IndentedJSON(http.StatusOK, gin.H{
				"message": fmt.Sprintf("message from service A, from service B: %s", response),
			})
		} else {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": fmt.Sprintf("response from %s API of service A did not contain a %s key", api, responseField),
			})
		}
	}
}
