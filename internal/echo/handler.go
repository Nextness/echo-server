package echo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

const DefaultMaxBodyBytes int64 = 1 << 20 // 1 MiB sounds fine

type Response struct {
	Headers http.Header         `json:"headers"`
	Params  map[string][]string `json:"params"`
	Body    string              `json:"body"`
	Path    string              `json:"path"`
	Error   string              `json:"error,omitempty"`
}

type Handler struct {
	maxBodyBytes int64
}

func NewHandler(maxBodyBytes int64) (*Handler, error) {
	if maxBodyBytes < 0 {
		return nil, fmt.Errorf("maxBodyBytes must not be negative")
	}
	return &Handler{maxBodyBytes}, nil
}

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
