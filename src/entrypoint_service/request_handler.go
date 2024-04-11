package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

func makeRequest(r *BasicPayload, url string, method string, responseField string, ctx context.Context) (string, int, error) {
	/*
		send a `BasicPayload` reqeust to the target `url`. If the response JSON includes
		a field that matches the `responseField` string, return it
	*/

	// construct Request JSON from BasicPayload data
	jsonData, err := json.Marshal(r)
	if err != nil {
		return "failed to construct JSON for request", http.StatusInternalServerError, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "failed to create request", http.StatusInternalServerError, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := Client.Do(req)
	// TODO: more granular HTTP status handling
	if err != nil {
		return "failed to send request", http.StatusInternalServerError, err
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
		return "failed to read response body", http.StatusInternalServerError, err
	}

	// construct string map from response body
	var responseMap map[string]string
	if err := json.Unmarshal(body, &responseMap); err != nil {
		return "failed to parse response JSON", http.StatusInternalServerError, err
	}

	if responseMap[responseField] == "" {
		return fmt.Sprintf("response from %s API of service B did not contain a `%s` key", url, responseField),
			http.StatusInternalServerError,
			errors.New("response empty")
	}

	return responseMap[responseField], http.StatusOK, nil
}
