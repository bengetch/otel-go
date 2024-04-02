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
	Meter             metric.Meter
	Tracer            trace.Tracer
	helloRequestCount metric.Int64Counter
)

func initServiceName() {
	ServiceName = os.Getenv("SERVICE_NAME")
	if ServiceName == "" {
		log.Fatal("SERVICE_NAME environment variable not set")
	}
}

func initTracer() {
	tracerName := fmt.Sprintf("%s.tracer", ServiceName)
	Tracer = otel.Tracer(tracerName)
}

func initMeter() {
	meterName := fmt.Sprintf("%s.Meter", ServiceName)
	Meter = otel.Meter(meterName)
}

func initHttpClient() {
	Client = NewHttpClient()
}

func initHelloRequestCount() {
	/*
		initialize an int counter Meter that tracks the number of requests to the `/` API of this service
	*/

	var err error
	meterName := fmt.Sprintf("%s.hello.requests", ServiceName)

	helloRequestCount, err = Meter.Int64Counter(meterName,
		metric.WithDescription("The number of requests to the `/` API"),
	)
	if err != nil {
		log.Fatalf("Failed to initialize %s.hello.requests Meter: %v\n", ServiceName, err)
	}
}

func main() {

	initServiceName()
	initTracer()
	initMeter()
	initHelloRequestCount()
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

	// configure Meter provider
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

	textPropagator := GetTextPropagator()
	otel.SetTextMapPropagator(textPropagator)

	router := gin.Default()
	router.Use(otelgin.Middleware(ServiceName))

	router.GET("/", hello)
	router.GET("/basicA", callServiceA)
	router.GET("/basicB", callServiceB)
	router.GET("/chainedA", chainedCallServiceA)
	router.GET("/chainedAsyncA", chainedAsyncCallServiceA)
	router.GET("/inlineTraceEx", inlineTracesExample)

	err := router.Run("0.0.0.0:5000")
	if err != nil {
		log.Printf("Failed to start the server: %v\n", err)
		os.Exit(1)
	}
}

func hello(c *gin.Context) {

	// increment Meter that tracks requests to `/` API of this service
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

	requestToA := BasicPayload{
		Message: "hello to A",
		Number:  rand.Intn(11),
	}

	api := "/basicRequest"
	responseField := "message"
	response, status, err := makeRequest(
		&requestToA,
		fmt.Sprintf("http://service_a:5000%s", api),
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
			"message": fmt.Sprintf("message from service A: %s", response),
		})
	}
}

func callServiceB(c *gin.Context) {
	/*
		send a hello message and a random number to service B, return response from B to client
	*/

	requestToB := BasicPayload{
		Message: "Hello to B",
		Number:  rand.Intn(11),
	}

	api := "/basicRequest"
	responseField := "message"
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
			"message": fmt.Sprintf("message from service B: %s", response),
		})
	}
}

func chainedCallServiceA(c *gin.Context) {
	/*
		send a hello message and a random number to service A, which sends the same to service B.
		service A waits for a response from service B, and relays that response back before it is
		returned to the client
	*/

	requestToA := BasicPayload{
		Message: "hello to A, and also to B",
		Number:  rand.Intn(11),
	}

	api := "/chainedRequest"
	responseField := "message"
	response, status, err := makeRequest(
		&requestToA,
		fmt.Sprintf("http://service_a:5000%s", api),
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
			"message": fmt.Sprintf("message from service A, from service B: %s", response),
		})
	}
}

func chainedAsyncCallServiceA(c *gin.Context) {
	/*
		send a hello message and a random number to service A, which sends the same to service B.
		service A does not wait for a response from service B before sending its response.
	*/

	requestToA := BasicPayload{
		Message: "asynchronous hello to A, and also to B",
		Number:  rand.Intn(11),
	}

	api := "/chainedAsyncRequest"
	responseField := "message"
	response, status, err := makeRequest(
		&requestToA,
		fmt.Sprintf("http://service_a:5000%s", api),
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
			"message": fmt.Sprintf("message from service A, from service B: %s", response),
		})
	}
}

func inlineTracesExample(c *gin.Context) {

	requestToA := BasicPayload{
		Message: "request for a number from A",
		Number:  rand.Intn(6),
	}

	api := "/addNumber"
	responseField := "number"
	response, status, err := makeRequest(
		&requestToA,
		fmt.Sprintf("http://service_a:5000%s", api),
		"POST",
		responseField,
		c.Request.Context(),
	)
	if err != nil {
		c.AbortWithStatusJSON(status, gin.H{
			"message": fmt.Sprintf("%s: %v", response, err),
		})
	} else {
		responseInt, err := strconv.Atoi(response)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
				"message": fmt.Sprintf("couldn't convert response %s to int: %v", response, err),
			})
		}
		if responseInt <= 5 {
			_, childSpan := Tracer.Start(c.Request.Context(), "span-entrypoint-add-number-less-than-5")
			// do some work here under trace defined above
			defer childSpan.End()
		} else {
			_, childSpan := Tracer.Start(c.Request.Context(), "span-entrypoint-add-number-more-than-5")
			// same thing, but with this other trace
			defer childSpan.End()
		}
		c.IndentedJSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("number from Entrypoint service added to number from service A: %s", response),
		})
	}

}
