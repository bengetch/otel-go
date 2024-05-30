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
	"go.opentelemetry.io/otel/propagation"
)

var (
	ServiceName      string
	Client           *http.Client
	EndpointServiceB = os.Getenv("ENDPOINT_SERVICE_B")
)

func initServiceName() {
	ServiceName = os.Getenv("SERVICE_NAME")
	if ServiceName == "" {
		log.Fatal("SERVICE_NAME environment variable not set")
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
	initHttpClient()

	logProvider := SetupLogs()
	tracerProvider := SetupTraces()
	meterProvider := SetupMetrics()
	defer CleanupTelemetryProviders(logProvider, tracerProvider, meterProvider)

	router := gin.Default()
	router.Use(otelgin.Middleware(ServiceName))

	router.GET("/", hello)
	router.POST("/basicRequest", basicRequest)
	router.POST("/chainedRequest", chainedRequest)
	router.POST("/chainedAsyncRequest", chainedAsyncRequest)
	router.POST("/addNumber", addNumber)

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

	c.IndentedJSON(http.StatusOK, gin.H{"message": "Hello from Service A"})
}

type BasicPayload struct {
	Message string `json:"message"`
	Number  int    `json:"number"`
}

func basicRequest(c *gin.Context) {

	otelzap.Ctx(c.Request.Context()).Info(
		fmt.Sprintf("hello from `/basicRequest` API of service %s", ServiceName),
	)

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

	otelzap.Ctx(c.Request.Context()).Info(
		fmt.Sprintf("hello from `/chainedRequest` API of service %s", ServiceName),
	)

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

	response, status, err := makeRequest(
		&requestToB,
		fmt.Sprintf("http://%s/chainedRequest", EndpointServiceB),
		"POST",
		"number",
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

	response, _, err := makeRequest(
		payload,
		fmt.Sprintf("http://%s/chainedRequest", EndpointServiceB),
		"POST",
		"number",
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

	otelzap.Ctx(c.Request.Context()).Info(
		fmt.Sprintf("hello from `/chainedAsyncRequest` API of service %s", ServiceName),
	)

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

func addNumber(c *gin.Context) {

	otelzap.Ctx(c.Request.Context()).Info(
		fmt.Sprintf("hello from `/addNumber` API of service %s", ServiceName),
	)

	var payload BasicPayload
	if err := c.BindJSON(&payload); err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message": "invalid request payload",
			"error":   err.Error(),
		})
		return
	}

	c.IndentedJSON(http.StatusOK, gin.H{
		"message": "hello from A, here is a number <= 10",
		"number":  strconv.Itoa(payload.Number + rand.Intn(6)),
	})
}
