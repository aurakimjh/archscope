package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
)

// This is a local real-proxy soak, not native Windows/UI parity. Set
// ARCHSCOPE_CAPTURE_SOAK=60s (and a longer go test -timeout) for a timed run.
func TestCaptureProxySoakAcrossClearPauseAndReentry(t *testing.T) {
	duration := time.Duration(0)
	if value := os.Getenv("ARCHSCOPE_CAPTURE_SOAK"); value != "" {
		var err error
		duration, err = time.ParseDuration(value)
		if err != nil || duration < time.Second || duration > time.Hour {
			t.Fatal("ARCHSCOPE_CAPTURE_SOAK must be 1s..1h")
		}
	}
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	defer service.shutdownAutomation()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Fixture", "resilience")
		io.WriteString(w, "synthetic response")
	}))
	defer origin.Close()
	begin := time.Now()
	cycles, total := 0, 0
	for cycles < 3 || time.Since(begin) < duration {
		session, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0", LiveWindow: 64, RetainUnattributedMetadata: true})
		if err != nil {
			t.Fatal(err)
		}
		proxyURL, _ := url.Parse("http://" + session.ListenAddress)
		transport := &http.Transport{Proxy: http.ProxyURL(proxyURL)}
		client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
		send := func(count int) {
			t.Helper()
			var wait sync.WaitGroup
			for worker := 0; worker < 4; worker++ {
				wait.Add(1)
				go func(worker int) {
					defer wait.Done()
					for i := worker; i < count; i += 4 {
						response, err := client.Get(fmt.Sprintf("%s/fixture?request=%d", origin.URL, i))
						if err != nil {
							t.Error(err)
							return
						}
						_, copyErr := io.Copy(io.Discard, response.Body)
						response.Body.Close()
						if copyErr != nil {
							t.Error(copyErr)
							return
						}
					}
				}(worker)
			}
			wait.Wait()
			total += count
		}
		send(128)
		if _, err = service.SetCapturePaused(string(session.ID), true); err != nil {
			t.Fatal(err)
		}
		send(4)
		if _, err = service.SetCapturePaused(string(session.ID), false); err != nil {
			t.Fatal(err)
		}
		cleared, err := service.ClearCapture(string(session.ID))
		if err != nil || cleared.Generation != 1 {
			t.Fatal("clear generation", err)
		}
		done := make(chan struct{})
		var reads sync.WaitGroup
		reads.Add(1)
		go func() {
			defer reads.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				if _, err := service.RecoverCaptureSessions(); err != nil {
					t.Error(err)
					return
				}
				if _, err := service.GetCaptureStats(string(session.ID)); err != nil {
					t.Error(err)
					return
				}
				if got := service.GetCurrentCaptureSession(); got.ID != session.ID {
					t.Error("reentry identity changed")
					return
				}
				select {
				case <-done:
					return
				case <-time.After(time.Millisecond):
				}
			}
		}()
		send(128)
		close(done)
		reads.Wait()
		if rows, err := service.GetCaptureLiveWindow(string(session.ID)); err != nil || len(rows) > 64 {
			t.Fatal("unbounded live window", err)
		}
		stopped, err := service.StopCapture(string(session.ID))
		transport.CloseIdleConnections()
		if err != nil || stopped.State != capture.StateFinalized {
			t.Fatal("stop failed", err)
		}
		stats, err := service.GetCaptureStats(string(session.ID))
		if err != nil || stats.Captured != 128 || stats.Persisted != 128 || stats.Generation != 1 {
			t.Fatalf("inconsistent final counters: %+v %v", stats, err)
		}
		if _, err = service.StopCapture(string(session.ID)); err != nil {
			t.Fatal("idempotent stop", err)
		}
		// The OS must let the exact port be reused after graceful source shutdown.
		listener, err := net.Listen("tcp4", session.ListenAddress)
		if err != nil {
			t.Fatal("listener leaked after stop", err)
		}
		listener.Close()
		cycles++
		if t.Failed() {
			return
		}
	}
	t.Logf("local proxy soak: elapsed=%s cycles=%d requests=%d; no native Windows/UI claim", time.Since(begin).Round(time.Millisecond), cycles, total)
}
