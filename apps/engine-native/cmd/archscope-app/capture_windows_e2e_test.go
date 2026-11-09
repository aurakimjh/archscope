//go:build windows

package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
)

// This opt-in test uses both real SEC-10 preflights. The operator must isolate
// and subsequently restore Windows dump policy; no policy mock is installed.
func TestWindowsBodyCaptureE2E(t *testing.T) {
	if os.Getenv("ARCHSCOPE_RUN_BODY_E2E") != "1" {
		t.Skip("set ARCHSCOPE_RUN_BODY_E2E=1 under a verified temporary dump policy")
	}
	const requestBody = `{"marker":"T603-REQUEST","message":"본문 캡처 검증"}`
	const responseBody = `{"marker":"T603-RESPONSE","ok":true}`
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != requestBody {
			t.Errorf("origin request=%q err=%v", body, err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, responseBody)
	}))
	defer origin.Close()
	service := newCaptureService(t.TempDir(), certstore.New(memoryTrustBackend{}, nil))
	started, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0", CaptureBodies: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = service.StopCapture(string(started.ID)) })
	proxyURL, _ := url.Parse("http://" + started.ListenAddress)
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	response, err := client.Post(origin.URL+"/body", "application/json", strings.NewReader(requestBody))
	if err != nil {
		t.Fatal(err)
	}
	gotBody, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil || string(gotBody) != responseBody {
		t.Fatalf("response=%q read=%v close=%v", gotBody, readErr, closeErr)
	}
	deadline := time.Now().Add(5 * time.Second)
	var transactionID string
	for {
		page, err := service.FetchCaptureTransactions(CaptureFetchRequest{SessionID: string(started.ID), Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if len(page.Items) == 1 {
			transactionID = page.Items[0].Transaction.ID
			tx, err := service.GetCaptureTransaction(string(started.ID), page.Items[0].Transaction.ID)
			if err != nil || tx == nil {
				t.Fatalf("detail=%+v err=%v", tx, err)
			}
			if !strings.Contains(tx.Request.BodyPreview, "T603-REQUEST") ||
				!strings.Contains(tx.Request.BodyPreview, "본문 캡처 검증") ||
				!strings.Contains(tx.Response.BodyPreview, "T603-RESPONSE") ||
				tx.Request.BodyStorage == "omitted" || tx.Response.BodyStorage == "omitted" {
				t.Fatalf("request=%+v response=%+v", tx.Request, tx.Response)
			}
			if tx.Process == nil || tx.Process.Attribution != "confirmed" {
				t.Fatalf("process=%+v", tx.Process)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("body transaction was not persisted")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := service.StopCapture(string(started.ID)); err != nil {
		t.Fatal(err)
	}
	stored, err := service.GetCaptureTransaction(string(started.ID), transactionID)
	if err != nil || stored == nil || !strings.Contains(stored.Request.BodyPreview, "T603-REQUEST") ||
		!strings.Contains(stored.Response.BodyPreview, "T603-RESPONSE") {
		t.Fatalf("finalized detail=%+v err=%v", stored, err)
	}
}

// TestWindowsLiveCaptureE2E is the in-process Windows plain-HTTP proxy smoke.
// The external schema-v4 PowerShell harness separately proves the required
// browser/curl/JVM/Electron, HTTPS, unsupported-tier, re-entry, long-session,
// and recovery matrix. The Windows build tag is intentional because direct
// GetExtendedTcpTable attribution cannot be truthfully validated on another OS.
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
			if stats.Observed != 1 || stats.Captured != 1 ||
				stats.Unattributed != 0 || stats.Dropped != 0 {
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
