package echo

import (
	"encoding/json"
	"errors"
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

func NewHandler(maxBodyBytes int64) *Handler {
	return &Handler{maxBodyBytes}
}

// TODO: Validate 'maxBodyBytes > 0' in configuration before constructing the handler
// TODO: Decide whether an error response should return the partial body or an empty body
// TODO: A 'HEAD' request should follow standard HTTP semantics: headers and status are returned, but the transport suppresses the response body
// TODO: Consider a small `writeJSON` helper if error handling would otherwise be duplicated
func (handler *Handler) ServeHTTP(writter http.ResponseWriter, request *http.Request) {
	headers := request.Header.Clone()

	if request.Host != "" {
		headers.Set("Host", request.Host)
	}

	// TODO: Should we handle error immediately?
	body, err := io.ReadAll(http.MaxBytesReader(writter, request.Body, handler.maxBodyBytes))
	response := Response{
		Headers: headers,
		Params:  request.URL.Query(),
		Body:    string(body),
		Path:    request.URL.Path,
	}

	status := http.StatusOK
	if err != nil {
		var maxBytesError *http.MaxBytesError
		// TODO: does it need to be a pointer to the pointer? Check later
		if errors.As(err, &maxBytesError) {
			status = http.StatusRequestEntityTooLarge
			response.Error = "request body exceeds the configured limit"
		} else {
			status = http.StatusBadRequest
			response.Error = "could not read request body"
		}
	}

	writter.Header().Set("Content-Type", "application/json")
	// TODO: Do we need nosniff? Check just in case
	writter.Header().Set("X-Content-Type-Options", "nosniff")
	writter.WriteHeader(status)

	_ = json.NewEncoder(writter).Encode(response)
}
