package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestValidateTarget(t *testing.T) {
	t.Parallel()

	for _, target := range []string{"192.168.0.42:18080", "listener.internal:8080", "[::1]:9000"} {
		if err := validateTarget(target); err != nil {
			t.Fatalf("validateTarget(%q): %v", target, err)
		}
	}
	for _, target := range []string{"", "listener.internal", ":8080", "listener.internal:0", "listener.internal:70000"} {
		if err := validateTarget(target); err == nil {
			t.Fatalf("validateTarget(%q) unexpectedly succeeded", target)
		}
	}
}

func TestTransactionHandler(t *testing.T) {
	t.Parallel()

	for _, path := range []string{"/tx", "/healthz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		transactionHandler().ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d, want %d", path, rec.Code, http.StatusOK)
		}
		body, err := io.ReadAll(rec.Result().Body)
		if err != nil {
			t.Fatalf("%s body: %v", path, err)
		}
		if string(body) != "ok" {
			t.Fatalf("%s body = %q, want %q", path, body, "ok")
		}
	}
}

func TestRunWorkerAgainstRemoteListener(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(transactionHandler())
	defer server.Close()

	result := runWorker(
		"worker-test",
		strings.TrimPrefix(server.URL, "http://"),
		true,
		200,
		30*time.Millisecond,
	)

	if result.PID == 0 {
		t.Fatal("worker PID was not recorded")
	}
	if result.LocalPort == 0 {
		t.Fatal("worker local port was not recorded")
	}
	if result.OK == 0 {
		t.Fatalf("worker completed no requests: %+v", result)
	}
	if result.Attempted != result.OK+result.Failed {
		t.Fatalf("attempt accounting mismatch: %+v", result)
	}
	if !result.Bypass {
		t.Fatal("worker bypass marker was not preserved")
	}
}
