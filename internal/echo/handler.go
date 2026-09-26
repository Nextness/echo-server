// Package echo provides an http.Handler that echoes request headers, query
// parameters, raw body, and path as JSON.
package echo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// DefaultMaxBodyBytes is the default maximum request-body size of 1 MiB.
const DefaultMaxBodyBytes int64 = 1 << 20

// Response is the JSON envelope returned for every request. Body is empty and
// Error is set when the request body cannot be read.
type Response struct {
	Headers http.Header         `json:"headers"`
	Params  map[string][]string `json:"params"`
	Body    string              `json:"body"`
	Path    string              `json:"path"`
	Error   string              `json:"error,omitempty"`
}

// Handler is a stateless http.Handler that echoes requests.
type Handler struct {
	maxBodyBytes int64
}

// NewHandler returns a Handler that limits request bodies to maxBodyBytes,
// which must be positive.
func NewHandler(maxBodyBytes int64) (*Handler, error) {
	if maxBodyBytes < 1 {
		return nil, fmt.Errorf("max body bytes must be a positive integer")
	}
	return &Handler{maxBodyBytes}, nil
}

// ServeHTTP writes the request headers, query parameters, raw body, and path
// as JSON. It returns 413 when the body exceeds the configured limit and 400
// when the body cannot be read.
func (handler *Handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	headers := request.Header.Clone()

	if request.Host != "" {
		headers.Set("Host", request.Host)
	}

	body, err := io.ReadAll(http.MaxBytesReader(writer, request.Body, handler.maxBodyBytes))
	response := Response{
		Headers: headers,
		Params:  request.URL.Query(),
		Path:    request.URL.Path,
	}

	status := http.StatusOK
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			status = http.StatusRequestEntityTooLarge
			response.Error = "request body exceeds the configured limit"
		} else {
			status = http.StatusBadRequest
			response.Error = "could not read request body"
		}
	} else {
		response.Body = string(body)
	}

	writer.Header().Set("Content-Type", "application/json")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)

	_ = json.NewEncoder(writer).Encode(response)
}
