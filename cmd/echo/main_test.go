package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRunRejectsInvalidConfiguration(t *testing.T) {
	t.Setenv("PORT", "invalid")
	t.Setenv("MAX_BODY_BYTES", "")

	err := run(context.Background())
	if err == nil {
		t.Fatal("Got nil error, but expected an invalid configuration error")
	}
	expectedErrorSubstring := "load configuration"
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Got error = %q, but expected it to contain %q", err, expectedErrorSubstring)
	}
}

func TestRunReturnsListenError(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Got listener error = %v, but expected nil", err)
	}
	defer listener.Close()

	port := listener.Addr().(*net.TCPAddr).Port
	t.Setenv("PORT", strconv.Itoa(port))
	t.Setenv("MAX_BODY_BYTES", "")

	err = run(context.Background())
	if err == nil {
		t.Fatal("Got nil error, but expected a listen error")
	}
	expectedErrorSubstring := "serve HTTP"
	if !strings.Contains(err.Error(), expectedErrorSubstring) {
		t.Errorf("Got error = %q, but expected it to contain %q", err, expectedErrorSubstring)
	}
}

func TestRunShutsDownWhenContextIsCancelled(t *testing.T) {
	port := availablePort(t)
	t.Setenv("PORT", strconv.Itoa(port))
	t.Setenv("MAX_BODY_BYTES", "")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	errorsChannel := make(chan error, 1)
	go func() {
		errorsChannel <- run(ctx)
	}()

	waitForServer(t, port, errorsChannel)

	cancel()
	waitForShutdown(t, errorsChannel)
}

func TestRunServesGeneralOptionsRequest(t *testing.T) {
	port := availablePort(t)
	t.Setenv("PORT", strconv.Itoa(port))
	t.Setenv("MAX_BODY_BYTES", "")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	errorsChannel := make(chan error, 1)
	go func() {
		errorsChannel <- run(ctx)
	}()

	waitForServer(t, port, errorsChannel)

	connection, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("Got dial error = %v, but expected nil", err)
	}
	t.Cleanup(func() {
		connection.Close()
	})

	rawRequest := fmt.Sprintf(
		"OPTIONS * HTTP/1.1\r\nHost: 127.0.0.1:%d\r\nConnection: close\r\n\r\n",
		port,
	)
	if _, err := connection.Write([]byte(rawRequest)); err != nil {
		t.Fatalf("Got request write error = %v, but expected nil", err)
	}

	response, err := http.ReadResponse(bufio.NewReader(connection), nil)
	if err != nil {
		t.Fatalf("Got response read error = %v, but expected nil", err)
	}
	t.Cleanup(func() {
		response.Body.Close()
	})

	expectedStatus := http.StatusOK
	if response.StatusCode != expectedStatus {
		t.Fatalf("Got status = %d, but expected %d", response.StatusCode, expectedStatus)
	}
	expectedContentType := "application/json"
	if got := response.Header.Get("Content-Type"); got != expectedContentType {
		t.Errorf("Got Content-Type = %q, but expected %q", got, expectedContentType)
	}

	var payload struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatalf("Got decode response error = %v, but expected nil", err)
	}
	expectedPath := "*"
	if payload.Path != expectedPath {
		t.Errorf("Got Path = %q, but expected %q", payload.Path, expectedPath)
	}

	cancel()
	waitForShutdown(t, errorsChannel)
}

func waitForServer(t *testing.T, port int, errorsChannel <-chan error) {
	t.Helper()

	client := &http.Client{Timeout: 100 * time.Millisecond}
	url := fmt.Sprintf("http://127.0.0.1:%d/readyz", port)
	deadline := time.Now().Add(2 * time.Second)
	expectedStatus := http.StatusOK
	for {
		response, err := client.Get(url)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == expectedStatus {
				return
			}
		}

		select {
		case err := <-errorsChannel:
			t.Fatalf("Got run() result = %v before cancellation, but expected the server to remain active", err)
		default:
		}

		if time.Now().After(deadline) {
			t.Fatal("Got server startup timeout, but expected it to accept requests")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func waitForShutdown(t *testing.T, errorsChannel <-chan error) {
	t.Helper()

	select {
	case err := <-errorsChannel:
		if err != nil {
			t.Fatalf("Got run() error = %v, but expected nil", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Got shutdown timeout, but expected run() to return")
	}
}

func availablePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Got listener error = %v, but expected nil", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatalf("Got listener close error = %v, but expected nil", err)
	}
	return port
}
