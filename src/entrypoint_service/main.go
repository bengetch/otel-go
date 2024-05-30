package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/agoda-com/opentelemetry-go/otelzap"
	"github.com/gin-gonic/gin"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var (
	ServiceName       string
	Meter             metric.Meter
	Tracer            trace.Tracer
	helloRequestCount metric.Int64Counter
	Client            *http.Client
	EndpointServiceA  = os.Getenv("ENDPOINT_SERVICE_A")
	EndpointServiceB  = os.Getenv("ENDPOINT_SERVICE_B")
)

func initServiceName() {
	ServiceName = os.Getenv("SERVICE_NAME")
	if ServiceName == "" {
		log.Fatal("SERVICE_NAME environment variable not set")
	}
}

func initTracerGlobal() {
	/*
		initialize global tracer instance, which is used to manually start traces when needed
	*/
	Tracer = otel.Tracer(fmt.Sprintf("%s.tracer", ServiceName))
}

func initMeterGlobal() {
	/*
		initialize global meter instance, which is used to manually construct various meter objects
	*/
	Meter = otel.Meter(fmt.Sprintf("%s.Meter", ServiceName))
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

func initHttpClient() {
	/*
		create an http.Client instance with otelhttp transport configured. this transport configuration
		ensures that trace context is correctly propagated across http requests
	*/

	Client = &http.Client{
		Timeout:   time.Second * 10,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
}

func main() {

	initServiceName()
	initTracerGlobal()
	initMeterGlobal()
	initHelloRequestCount()
	initHttpClient()

	logProvider := SetupLogs()
	tracerProvider := SetupTraces()
	meterProvider := SetupMetrics()
	defer CleanupTelemetryProviders(logProvider, tracerProvider, meterProvider)

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
		otelzap.Ctx(context.Background()).Fatal(
			fmt.Sprintf("Failed to start the server: %v\n", err),
		)
	}
}

func hello(c *gin.Context) {

	otelzap.Ctx(c.Request.Context()).Info(
		fmt.Sprintf("hello from `/` API of service %s", ServiceName),
	)

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

	otelzap.Ctx(c.Request.Context()).Info(
		fmt.Sprintf("hello from `/basicA` API of service %s", ServiceName),
	)

	requestToA := BasicPayload{
		Message: "hello to A",
		Number:  rand.Intn(11),
	}

	response, status, err := makeRequest(
		&requestToA,
		fmt.Sprintf("http://%s/basicRequest", EndpointServiceA),
		"POST",
		"message",
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

	otelzap.Ctx(c.Request.Context()).Info(
		fmt.Sprintf("hello from `/basicB` API of service %s", ServiceName),
	)

	requestToB := BasicPayload{
		Message: "Hello to B",
		Number:  rand.Intn(11),
	}

	response, status, err := makeRequest(
		&requestToB,
		fmt.Sprintf("http://%s/basicRequest", EndpointServiceB),
		"POST",
		"message",
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

	otelzap.Ctx(c.Request.Context()).Info(
		fmt.Sprintf("hello from `/chainedA` API of service %s", ServiceName),
	)

	requestToA := BasicPayload{
		Message: "hello to A, and also to B",
		Number:  rand.Intn(11),
	}

	response, status, err := makeRequest(
		&requestToA,
		fmt.Sprintf("http://%s/chainedRequest", EndpointServiceA),
		"POST",
		"message",
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

	otelzap.Ctx(c.Request.Context()).Info(
		fmt.Sprintf("hello from `/chainedAsyncA` API of service %s", ServiceName),
	)

	requestToA := BasicPayload{
		Message: "asynchronous hello to A, and also to B",
		Number:  rand.Intn(11),
	}

	response, status, err := makeRequest(
		&requestToA,
		fmt.Sprintf("http://%s/chainedAsyncRequest", EndpointServiceA),
		"POST",
		"message",
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

	otelzap.Ctx(c.Request.Context()).Info(
		fmt.Sprintf("hello from `/inlineTraceEx` API of service %s", ServiceName),
	)

	requestToA := BasicPayload{
		Message: "request for a number from A",
		Number:  rand.Intn(6),
	}

	response, status, err := makeRequest(
		&requestToA,
		fmt.Sprintf("http://%s/addNumber", EndpointServiceA),
		"POST",
		"number",
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
