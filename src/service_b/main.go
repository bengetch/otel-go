package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
)

func main() {
	router := gin.Default()
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
	// serialize struct to JSON and add it to response
	c.IndentedJSON(http.StatusOK, gin.H{"message": "hello from Service B"})
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
		"message": fmt.Sprintf("service B received number %v from Entrypoint service", payload.Number),
	})
}

func chainedRequest(c *gin.Context) {

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
