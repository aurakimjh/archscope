package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/store"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// This is a local real-proxy soak, not native Windows/UI parity. Set
// ARCHSCOPE_CAPTURE_SOAK=60s (and a longer go test -timeout) for a timed run.
func TestCaptureProxySoakAcrossClearPauseAndReentry(t *testing.T) {
	duration := time.Duration(0)
	if value := os.Getenv("ARCHSCOPE_CAPTURE_SOAK"); value != "" {
		var err error
		duration, err = time.ParseDuration(value)
		if err != nil || duration < 2*time.Second || duration > 6*time.Hour {
			t.Fatal("ARCHSCOPE_CAPTURE_SOAK must be 2s..6h")
		}
	}
	service := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	defer service.shutdownAutomation()
	report := struct {
		Schema         int     `json:"schema"`
		OS             string  `json:"os"`
		Arch           string  `json:"arch"`
		Commit         string  `json:"commit"`
		Scope          string  `json:"scope"`
		Passed         bool    `json:"passed"`
		Cycles         int     `json:"cycles"`
		Requests       int     `json:"requests"`
		ElapsedSeconds float64 `json:"elapsedSeconds"`
		PeakHeapBytes  uint64  `json:"sampledPeakHeapBytes"`
		PeakGoroutines int     `json:"sampledPeakGoroutines"`
		PeakStoreBytes int64   `json:"peakStoreBytes"`
		MaxRetained    uint64  `json:"maxRetained"`
		RetentionRows  int     `json:"retentionRows"`
		Admission      bool    `json:"admission"`
	}{Schema: 1, OS: runtime.GOOS, Arch: runtime.GOARCH, Commit: os.Getenv("ARCHSCOPE_SOAK_COMMIT"), Scope: "synthetic loopback common engine; no native GUI/driver/body-tier acceptance"}
	if duration > 0 {
		report.RetentionRows = 64
		report.Admission = true
	}
	key := ""
	if report.Admission {
		var release func()
		var e error
		key, release, e = service.manager.Admissions().Register(func(_ context.Context, _ string, tx models.CaptureTransaction) (bool, error) {
			u, e := url.Parse(tx.URL)
			if e != nil {
				return false, e
			}
			n, e := strconv.Atoi(u.Query().Get("request"))
			return e == nil && n%11 != 0, nil
		})
		if e != nil {
			t.Fatal(e)
		}
		defer release()
	}
	sample := func() {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		report.PeakHeapBytes = max(report.PeakHeapBytes, m.HeapAlloc)
		report.PeakGoroutines = max(report.PeakGoroutines, runtime.NumGoroutine())
	}
	sample()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Fixture", "resilience")
		io.WriteString(w, "synthetic response")
	}))
	defer origin.Close()
	portable := filepath.Join(t.TempDir(), "soak.httpsession")
	begin := time.Now()
	t.Cleanup(func() {
		report.Passed = !t.Failed()
		report.ElapsedSeconds = time.Since(begin).Seconds()
		if path := os.Getenv("ARCHSCOPE_SOAK_REPORT"); path != "" {
			data, e := json.MarshalIndent(report, "", "  ")
			if e == nil {
				e = os.WriteFile(path, append(data, '\n'), 0600)
			}
			if e != nil {
				t.Error("write soak report", e)
			}
		}
	})
	cycles, total := 0, 0
	for cycles < 3 || time.Since(begin) < duration {
		session, err := service.StartCapture(capture.Config{AdmissionOwner: key, RetentionRows: report.RetentionRows, ListenAddress: "127.0.0.1:0", LiveWindow: 64, RetainUnattributedMetadata: true})
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
		expected := uint64(128)
		if report.Admission {
			expected = 116
		}
		if err != nil || stats.Captured != expected || stats.Persisted != expected || stats.Generation != 1 {
			t.Fatalf("inconsistent final counters: %+v %v", stats, err)
		}
		if report.Admission && stats.Dropped != 12 {
			t.Fatalf("admission counter mismatch: %+v", stats)
		}
		if duration > 0 {
			meta, e := service.manager.Manifest(session.ID)
			if e != nil || meta.RetainedRecords != 64 || meta.Counters.Persisted != expected || meta.Counters.Rotated != expected-64 {
				t.Fatal("retention invariant", meta, e)
			}
			report.MaxRetained = max(report.MaxRetained, meta.RetainedRecords)
			entries, e := os.ReadDir(filepath.Join(session.StorePath, "rolling"))
			if e != nil || len(entries) != 64 {
				t.Fatal("physical row leak", len(entries), e)
			}
			reopened, e := store.Open(session.StorePath)
			if e != nil {
				t.Fatal(e)
			}
			page, e := reopened.Fetch(store.Filter{}, "", 100)
			closeErr := reopened.Close()
			if e != nil || closeErr != nil || len(page.Items) != 64 {
				t.Fatal("reopen", len(page.Items), e, closeErr)
			}
			destination := portable
			n, e := service.ExportCaptureFile(CaptureExportRequest{Source: CaptureFileSource{SessionID: string(session.ID)}, Destination: destination, Format: "session", Scope: "all"})
			if e != nil || n != 64 {
				t.Fatal("export", n, e)
			}
			doc, _, e := sessionfile.Read(destination)
			if e != nil || doc.Retention == nil || doc.Retention.Retained != 64 || len(doc.Transactions) != 64 {
				t.Fatal("portable retention", e)
			}
			var bytes int64
			walkErr := filepath.WalkDir(session.StorePath, func(_ string, d os.DirEntry, e error) error {
				if e != nil {
					return e
				}
				if !d.IsDir() {
					i, e := d.Info()
					if e != nil {
						return e
					}
					bytes += i.Size()
				}
				return nil
			})
			if walkErr != nil {
				t.Fatal(walkErr)
			}
			report.PeakStoreBytes = max(report.PeakStoreBytes, bytes)
			if e = os.Remove(destination); e != nil {
				t.Fatal(e)
			}
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
		report.Cycles = cycles
		report.Requests = total
		sample()
		// Keep the harness itself bounded over hours. Only this fixture's
		// finalized temporary session is removed after reopen/export checks.
		if duration > 0 {
			if err = os.RemoveAll(session.StorePath); err != nil {
				t.Fatal(err)
			}
		}
		if t.Failed() {
			return
		}
	}
	t.Logf("local proxy soak: elapsed=%s cycles=%d requests=%d; no native Windows/UI claim", time.Since(begin).Round(time.Millisecond), cycles, total)
}
