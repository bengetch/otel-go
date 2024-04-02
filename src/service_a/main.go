package main

import (
	"context"
	"fmt"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

var (
	ServiceName string
)

func initServiceName() {
	ServiceName = os.Getenv("SERVICE_NAME")
	if ServiceName == "" {
		log.Fatal("SERVICE_NAME environment variable not set")
	}
}

func initHttpClient() {
	Client = NewHttpClient()
}

func main() {

	initServiceName()
	initHttpClient()

	otelExporterOtlpEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	// configure tracer provider
	tp, tpErr := GetTraceProvider(os.Getenv("SPAN_EXPORTER"), otelExporterOtlpEndpoint)
	if tpErr != nil {
		log.Fatalf("Failed to get tracer provider: %v\n", tpErr)
	} else {
		otel.SetTracerProvider(tp)
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
	} else {
		otel.SetMeterProvider(mp)
	}
	defer func(mp *sdkmetric.MeterProvider, ctx context.Context) {
		err := mp.Shutdown(ctx)
		if err != nil {
			log.Printf("Error while shutting down Metric provider: %v\n", err)
		}
	}(mp, context.Background())

	// TODO: see if these .With<provider> opts are necessary
	textPropagator := GetTextPropagator()
	otel.SetTextMapPropagator(textPropagator)

	router := gin.Default()
	router.Use(otelgin.Middleware(
		ServiceName,
		otelgin.WithTracerProvider(tp),
		otelgin.WithPropagators(textPropagator)),
	)

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
	c.IndentedJSON(http.StatusOK, gin.H{"message": "Hello from Service A"})
}

type BasicPayload struct {
	Message string `json:"message"`
	Number  int    `json:"number"`
}

func basicRequest(c *gin.Context) {

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
	response, status, err := makeRequest(
		&requestToB,
		fmt.Sprintf("http://service_b:5000%s", api),
		"POST",
		responseField,
		c.Request.Context(),
	)
	if err != nil {
		c.AbortWithStatusJSON(status, gin.H{
			"message": fmt.Sprintf("%s: %v", response, err),
		})
	} else {
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("number from service A, from service B: %s", response),
		})
	}
}

func makeAsyncRequest(payload *BasicPayload, ctx context.Context) {

	api := "/chainedRequest"
	responseField := "number"

	response, _, err := makeRequest(
		payload,
		fmt.Sprintf("http://service_b:5000%s", api),
		"POST",
		responseField,
		ctx,
	)

	// wait 10 seconds to ensure that below logs fire after response has been sent
	time.Sleep(10 * time.Second)

	if err != nil {
		log.Printf("Err: %v", err)
	} else {
		log.Printf("number from service B: %s", response)
	}
}

func newContext(oldContext context.Context, header http.Header) context.Context {
	/*
		construct a new context that is not bound to the gin.Request.Context, but contains
		data for current trace
	*/

	propagator := otel.GetTextMapPropagator()
	extractedCtx := propagator.Extract(oldContext, propagation.HeaderCarrier(header))
	newCtx := context.Background()
	carrier := propagation.MapCarrier{}
	propagator.Inject(extractedCtx, carrier)

	return propagator.Extract(newCtx, carrier)
}

func chainedAsyncRequest(c *gin.Context) {

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

	go makeAsyncRequest(&requestToB, newContext(c.Request.Context(), c.Request.Header))

	c.IndentedJSON(http.StatusOK, gin.H{
		"message": "successfully sent asynchronous message to service B",
	})
}
