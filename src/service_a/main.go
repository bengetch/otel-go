package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

func main() {
	router := gin.Default()
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
	// serialize struct to JSON and add it to response
	c.IndentedJSON(http.StatusOK, gin.H{"message": "Hello from Service A"})
}

type BasicPayload struct {
	Message string `json:"message"`
	Number  int    `json:"number"`
}

func makeRequest(c *gin.Context, r *BasicPayload, url string, method string, responseField string) (string, error) {
	/*
		send a `BasicPayload` to the target `url`. If the response JSON includes
		a field that matches the `responseField` string, return it
	*/

	jsonData, err := json.Marshal(r)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "failed to construct JSON for request",
			"error":   err.Error(),
		})
		return "", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "failed to create request",
			"error":   err.Error(),
		})
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: time.Second * 10}
	resp, err := client.Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "failed to send request",
			"error":   err.Error(),
		})
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "failed to read response body",
			"error":   err.Error(),
		})
		return "", err
	}

	var responseMap map[string]string
	if err := json.Unmarshal(body, &responseMap); err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "failed to parse response JSON",
			"error":   err.Error(),
		})
		return "", err
	}

	return responseMap[responseField], nil
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
	go makeAsyncRequest(c, &requestToB)

	c.IndentedJSON(http.StatusOK, gin.H{
		"message": "successfully sent asynchronous message to service B",
	})
}
