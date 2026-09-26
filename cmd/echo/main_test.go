package main

import (
	"context"
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

	client := &http.Client{Timeout: 100 * time.Millisecond}
	url := fmt.Sprintf("http://127.0.0.1:%d/readyz", port)
	deadline := time.Now().Add(2 * time.Second)
	expectedStatus := http.StatusOK
	for {
		response, err := client.Get(url)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == expectedStatus {
				break
			}
		}

		select {
		case err := <-errorsChannel:
			t.Fatalf("Got run() result = %v before cancellation, but expected the server to remain active", err)
		default:
		}

		if time.Now().After(deadline) {
			cancel()
			t.Fatal("Got server startup timeout, but expected it to accept requests")
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
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
