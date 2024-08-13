package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"strconv"

	otelhandlers "github.com/bengetch/otelhandlers/sdk"
	"github.com/gin-gonic/gin"

	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

var (
	ServiceName string
	SelfPort    = os.Getenv("SELF_PORT")
)

func initServiceName() {
	ServiceName = os.Getenv("SERVICE_NAME")
	if ServiceName == "" {
		log.Fatal("SERVICE_NAME environment variable not set")
	}
}

func main() {

	initServiceName()

	logProvider := otelhandlers.SetupLogs(os.Getenv("LOGS_EXPORTER"), ServiceName)
	tracerProvider := otelhandlers.SetupTraces(os.Getenv("TRACES_EXPORTER"), ServiceName)
	meterProvider := otelhandlers.SetupMetrics(os.Getenv("METRICS_EXPORTER"), ServiceName)
	defer otelhandlers.CleanupTelemetryProviders(logProvider, tracerProvider, meterProvider)

	router := gin.Default()
	router.Use(otelgin.Middleware(ServiceName))

	// configure gin server API
	router.GET("/", hello)
	router.POST("/basicRequest", basicRequest)
	router.POST("/chainedRequest", chainedRequest)

	err := router.Run(fmt.Sprintf("0.0.0.0:%s", SelfPort))
	if err != nil {
		slog.ErrorContext(
			context.Background(),
			fmt.Sprintf("Failed to start the server: %v\n", err),
		)
	}
}

func hello(c *gin.Context) {

	slog.InfoContext(
		c.Request.Context(),
		fmt.Sprintf("hello from `/` API of service %s", ServiceName),
	)

	c.IndentedJSON(http.StatusOK, gin.H{"message": "hello from Service B"})
}

type BasicPayload struct {
	Message string `json:"message"`
	Number  int    `json:"number"`
}

func basicRequest(c *gin.Context) {

	slog.InfoContext(
		c.Request.Context(),
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
		"message": fmt.Sprintf("service B received number %v from Entrypoint service", payload.Number),
	})
}

func chainedRequest(c *gin.Context) {

	slog.InfoContext(
		c.Request.Context(),
		fmt.Sprintf("hello from `/chainedRequest` API of service %s", ServiceName),
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
		"message": "hello to A, and also to Entrypoint",
		"number":  strconv.Itoa(payload.Number + rand.Intn(11)),
	})
}
