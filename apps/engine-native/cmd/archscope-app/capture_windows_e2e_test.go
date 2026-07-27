//go:build windows

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
)

// TestWindowsLiveCaptureE2E exercises the same explicit-proxy path used by
// browser/curl/JVM/Electron clients. The Windows build tag is intentional:
// the acceptance condition includes direct GetExtendedTcpTable attribution,
// which cannot be truthfully validated on another OS.
func TestWindowsLiveCaptureE2E(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Content-Type", "text/plain")
		response.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(response, "t581-ok")
	}))
	defer origin.Close()

	service := newCaptureService(t.TempDir(), certstore.New(memoryTrustBackend{}, nil))
	started, err := service.StartCapture(capture.Config{
		ListenAddress: "127.0.0.1:0",
		ReserveBytes:  0,
		LiveWindow:    50,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = service.StopCapture(string(started.ID))
	})

	proxyURL, err := url.Parse("http://" + started.ListenAddress)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
	}
	response, err := client.Get(origin.URL + "/supported-tier")
	if err != nil {
		t.Fatal(err)
	}
	body, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read=%v close=%v", readErr, closeErr)
	}
	if response.StatusCode != http.StatusOK || string(body) != "t581-ok" {
		t.Fatalf("status=%d body=%q", response.StatusCode, body)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		stats, statsErr := service.GetCaptureStats(string(started.ID))
		if statsErr != nil {
			t.Fatal(statsErr)
		}
		if stats.Persisted == 1 {
			if stats.Unattributed != 0 || stats.Dropped != 0 {
				t.Fatalf("confirmed Windows flow stats=%+v", stats)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("transaction was not persisted before deadline: %+v", stats)
		}
		time.Sleep(20 * time.Millisecond)
	}

	stopped, err := service.StopCapture(string(started.ID))
	if err != nil {
		t.Fatal(err)
	}
	if stopped.State != capture.StateFinalized {
		t.Fatalf("stopped=%+v", stopped)
	}
	page, err := service.FetchCaptureTransactions(CaptureFetchRequest{
		SessionID: string(started.ID),
		Limit:     10,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("page=%+v", page)
	}
	transaction := page.Items[0].Transaction
	if transaction.Process == nil || transaction.Process.Attribution != "confirmed" {
		t.Fatalf("process attribution=%+v", transaction.Process)
	}
	if transaction.Fidelity != "decoded_wire" || transaction.CaptureMode != "proxy_mitm" {
		t.Fatalf("mode=%q fidelity=%q", transaction.CaptureMode, transaction.Fidelity)
	}
	if transaction.Request.BodyStorage != "omitted" ||
		transaction.Response.BodyStorage != "omitted" {
		t.Fatalf("body storage request=%q response=%q",
			transaction.Request.BodyStorage, transaction.Response.BodyStorage)
	}
}
