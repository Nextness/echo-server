package echo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

func TestHandlerEchoesRequest(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/nested/path?tag=go&tag=kubernetes&empty=",
		strings.NewReader(`{"message": "hello"}`),
	)
	request.Header.Add("X-Demo", "one")
	request.Header.Add("X-Demo", "two")
	request.Header.Add("Content-Type", "application/json")
	request.Host = "echo.test"

	recorder := httptest.NewRecorder()
	newTestHandler(t, DefaultMaxBodyBytes).ServeHTTP(recorder, request)

	expectedStatus := http.StatusOK
	if recorder.Code != expectedStatus {
		t.Fatalf("Got status = %d, but expected %d", recorder.Code, expectedStatus)
	}
	expectedContentType := "application/json"
	if got := recorder.Header().Get("Content-Type"); got != expectedContentType {
		t.Fatalf("Got Content-Type = %q, but expected %q", got, expectedContentType)
	}
	expectedContentTypeOptions := "nosniff"
	if got := recorder.Header().Get("X-Content-Type-Options"); got != expectedContentTypeOptions {
		t.Errorf("Got X-Content-Type-Options = %q, but expected %q", got, expectedContentTypeOptions)
	}

	response := decodeResponse(t, recorder)
	expectedPath := "/nested/path"
	if response.Path != expectedPath {
		t.Errorf("Got Path = %q, but expected %q", response.Path, expectedPath)
	}
	expectedBody := `{"message": "hello"}`
	if response.Body != expectedBody {
		t.Errorf("Got Body = %q, but expected %q", response.Body, expectedBody)
	}
	expectedTags := []string{"go", "kubernetes"}
	if !reflect.DeepEqual(response.Params["tag"], expectedTags) {
		t.Errorf("Got Tags = %#v, but expected %#v", response.Params["tag"], expectedTags)
	}
	expectedEmpty := []string{""}
	if !reflect.DeepEqual(response.Params["empty"], expectedEmpty) {
		t.Errorf("Got empty params = %#v, but expected %#v", response.Params["empty"], expectedEmpty)
	}
	expectedXDemo := []string{"one", "two"}
	if !reflect.DeepEqual(response.Headers.Values("X-Demo"), expectedXDemo) {
		t.Errorf("Got X-Demo = %#v, but expected %#v", response.Headers.Values("X-Demo"), expectedXDemo)
	}
	expectedHost := "echo.test"
	if got := response.Headers.Get("Host"); got != expectedHost {
		t.Errorf("Got Host = %q, but expected %q", got, expectedHost)
	}
	expectedError := ""
	if response.Error != expectedError {
		t.Errorf("Got Error = %q, but expected %q", response.Error, expectedError)
	}
}

func TestHandlerRejectsOversizeBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("12345"))
	recorder := httptest.NewRecorder()

	newTestHandler(t, 4).ServeHTTP(recorder, request)
	expectedStatus := http.StatusRequestEntityTooLarge
	if recorder.Code != expectedStatus {
		t.Fatalf("Got status = %d, but expected %d", recorder.Code, expectedStatus)
	}

	response := decodeResponse(t, recorder)
	expectedError := "request body exceeds the configured limit"
	if response.Error != expectedError {
		t.Errorf("Got Error = %q, but expected %q", response.Error, expectedError)
	}
	expectedBody := ""
	if response.Body != expectedBody {
		t.Errorf("Got Body = %q, but expected %q", response.Body, expectedBody)
	}
}

func TestHandlerAcceptsSupportedRequests(t *testing.T) {
	type localStruct struct {
		name         string
		method       string
		target       string
		body         string
		expectedPath string
		expectedBody string
	}
	tests := []localStruct{
		{name: "root GET", method: http.MethodGet, target: "/", expectedPath: "/", expectedBody: ""},
		{name: "plain text POST", method: http.MethodPost, target: "/plain", body: "hello", expectedPath: "/plain", expectedBody: "hello"},
		{name: "unicode PUT", method: http.MethodPut, target: "/caf%C3%A9", body: "olá, 世界", expectedPath: "/café", expectedBody: "olá, 世界"},
		{name: "PATCH", method: http.MethodPatch, target: "/nested/path", body: `{"enabled":true}`, expectedPath: "/nested/path", expectedBody: `{"enabled":true}`},
		{name: "DELETE", method: http.MethodDelete, target: "/resource/42", expectedPath: "/resource/42", expectedBody: ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			recorder := httptest.NewRecorder()

			newTestHandler(t, DefaultMaxBodyBytes).ServeHTTP(recorder, request)

			expectedStatus := http.StatusOK
			if recorder.Code != expectedStatus {
				t.Fatalf("Got status = %d, but expected %d", recorder.Code, expectedStatus)
			}
			response := decodeResponse(t, recorder)
			if response.Path != test.expectedPath {
				t.Errorf("Got Path = %q, but expected %q", response.Path, test.expectedPath)
			}
			if response.Body != test.expectedBody {
				t.Errorf("Got Body = %q, but expected %q", response.Body, test.expectedBody)
			}
		})
	}
}

func TestHandlerAcceptsBodyAtLimit(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("1234"))
	recorder := httptest.NewRecorder()

	newTestHandler(t, 4).ServeHTTP(recorder, request)

	expectedStatus := http.StatusOK
	if recorder.Code != expectedStatus {
		t.Fatalf("Got status = %d, but expected %d", recorder.Code, expectedStatus)
	}
	response := decodeResponse(t, recorder)
	expectedBody := "1234"
	if response.Body != expectedBody {
		t.Errorf("Got Body = %q, but expected %q", response.Body, expectedBody)
	}
}

func TestHandlerReturnsBadRequestWhenBodyReadFails(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/upload", nil)
	request.Body = failingReadCloser{}
	recorder := httptest.NewRecorder()

	newTestHandler(t, DefaultMaxBodyBytes).ServeHTTP(recorder, request)

	expectedStatus := http.StatusBadRequest
	if recorder.Code != expectedStatus {
		t.Fatalf("Got status = %d, but expected %d", recorder.Code, expectedStatus)
	}
	response := decodeResponse(t, recorder)
	expectedError := "could not read request body"
	if response.Error != expectedError {
		t.Errorf("Got Error = %q, but expected %q", response.Error, expectedError)
	}
	expectedBody := ""
	if response.Body != expectedBody {
		t.Errorf("Got Body = %q, but expected %q", response.Body, expectedBody)
	}
}

func TestHandlerHeadResponseHasNoBody(t *testing.T) {
	server := httptest.NewServer(newTestHandler(t, DefaultMaxBodyBytes))
	t.Cleanup(server.Close)

	response, err := server.Client().Head(server.URL + "/head")
	if err != nil {
		t.Fatalf("Got HEAD request error = %v, but expected nil", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("Got response body read error = %v, but expected nil", err)
	}
	expectedStatus := http.StatusOK
	if response.StatusCode != expectedStatus {
		t.Errorf("Got status = %d, but expected %d", response.StatusCode, expectedStatus)
	}
	expectedBodyLength := 0
	if len(body) != expectedBodyLength {
		t.Errorf("Got HEAD response body length = %d, but expected %d", len(body), expectedBodyLength)
	}
}

func TestHandlerServesConcurrentRequests(t *testing.T) {
	const requestCount = 32

	handler := newTestHandler(t, DefaultMaxBodyBytes)
	errorsChannel := make(chan error, requestCount)
	var waitGroup sync.WaitGroup
	expectedStatus := http.StatusOK

	for index := range requestCount {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()

			expectedBody := fmt.Sprintf("request-%d", index)
			request := httptest.NewRequest(http.MethodPost, "/concurrent", strings.NewReader(expectedBody))
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != expectedStatus {
				errorsChannel <- fmt.Errorf("Got status = %d, but expected %d", recorder.Code, expectedStatus)
				return
			}

			var response Response
			if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
				errorsChannel <- fmt.Errorf("Got decode response error = %v, but expected nil", err)
				return
			}
			if response.Body != expectedBody {
				errorsChannel <- fmt.Errorf("Got Body = %q, but expected %q", response.Body, expectedBody)
			}
		}()
	}

	waitGroup.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		t.Error(err)
	}
}

func TestNewHandlerRejectsInvalidBodyLimits(t *testing.T) {
	tests := []int64{0, -1}

	for _, value := range tests {
		t.Run(fmt.Sprintf("%d", value), func(t *testing.T) {
			_, err := NewHandler(value)
			if err == nil {
				t.Fatalf("Got NewHandler(%d) error = nil, but expected an error", value)
			}
			expectedErrorSubstring := "positive integer"
			if !strings.Contains(err.Error(), expectedErrorSubstring) {
				t.Errorf("Got error = %q, but expected it to contain %q", err, expectedErrorSubstring)
			}
		})
	}
}

func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) Response {
	t.Helper()

	var response Response
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Got decode response error = %v, but expected nil", err)
	}
	return response
}

func newTestHandler(t *testing.T, maxBodyBytes int64) *Handler {
	t.Helper()

	handler, err := NewHandler(maxBodyBytes)
	if err != nil {
		t.Fatalf("Got NewHandler() error = %v, but expected nil", err)
	}
	return handler
}

type failingReadCloser struct{}

func (failingReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func (failingReadCloser) Close() error {
	return nil
}
