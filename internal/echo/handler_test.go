package echo

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// TODO: '/', nested paths, and encoded path segments
// TODO: GET, POST, PUT, PATCH, and DELETE
// TODO: Empty bodies
// TODO: Plain text, JSON text, and Unicode
// TODO: Empty and repeated query parameters
// TODO: Repeated headers and `Host`
// TODO: Exactly-at-limit and over-limit bodies
// TODO: Correct JSON error status and shape

func TestHandleEchosRequest(t *testing.T) {
	request := httptest.NewRequest(
		http.MethodPost,
		"/nested/path?tag=go&tag=kubernetes&empty=",
		strings.NewReader(`{"message": "hello"}`),
	)
	request.Header.Add("X-Demo", "one")
	request.Header.Add("X-Demo", "two")
	request.Header.Add("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	NewHandler(DefaultMaxBodyBytes).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("Got status = %d, but expected %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Got Content-Type = %q, but expected 'application/json'", got)
	}

	var response Response
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("Unexpected decode response: %v", err)
	}

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

	expectedXDemo := []string{"one", "two"}
	if !reflect.DeepEqual(response.Headers.Values("X-Demo"), expectedXDemo) {
		t.Errorf("Got X-Demo = %#v, but expected %#v", response.Headers.Values("X-Demo"), expectedXDemo)
	}
}

func TestHandlerRejectsOversizeBody(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("12345"))
	recorder := httptest.NewRecorder()

	NewHandler(4).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("Got status = %d, but expected %d", recorder.Code, http.StatusRequestEntityTooLarge)
	}
}
