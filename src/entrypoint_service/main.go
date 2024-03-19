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
	c.IndentedJSON(http.StatusOK, gin.H{"message": "hello from Entrypoint service"})
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

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
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
