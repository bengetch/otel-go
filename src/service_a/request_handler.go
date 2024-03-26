package main

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"io"
	"log"
	"net/http"
	"time"
)

func makeRequest(c *gin.Context, r *BasicPayload, url string, method string, responseField string) (string, error) {
	/*
		send a `BasicPayload` to the target `url`. If the response JSON includes
		a field that matches the `responseField` string, return it
	*/

	// construct Request JSON from BasicPayload data
	jsonData, err := json.Marshal(r)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "failed to construct JSON for request",
			"error":   err.Error(),
		})
		return "", err
	}

	// construct new Request object from JSON
	req, err := http.NewRequestWithContext(c.Request.Context(), method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "failed to create request",
			"error":   err.Error(),
		})
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	// instantiate HTTP Client object and send request
	client := &http.Client{
		Timeout:   time.Second * 10,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	resp, err := client.Do(req)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "failed to send request",
			"error":   err.Error(),
		})
		return "", err
	}
	defer func(resp *http.Response) {
		err := resp.Body.Close()
		if err != nil {
			log.Printf("Error while closing HTTP Response body: %v\n", err)
		}
	}(resp)

	// read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"message": "failed to read response body",
			"error":   err.Error(),
		})
		return "", err
	}

	// construct string map from response body
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
