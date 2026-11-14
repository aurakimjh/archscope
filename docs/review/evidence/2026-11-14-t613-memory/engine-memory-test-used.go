package main

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
)

// Opt-in real-proxy lifecycle measurement. GC affects only this test process,
// never the production desktop/WebView. This is not GUI or universal leak proof.
func TestCaptureLifecyclePostGCMemory(t *testing.T) {
	value := os.Getenv("ARCHSCOPE_CAPTURE_MEMORY_CYCLES")
	if value == "" {
		t.Skip("set ARCHSCOPE_CAPTURE_MEMORY_CYCLES=24 for post-GC lifecycle measurement")
	}
	cycles, err := strconv.Atoi(value)
	if err != nil || cycles < 24 || cycles > 200 {
		t.Fatal("memory cycles must be 24..200")
	}
	type point struct {
		Cycle       int    `json:"cycle"`
		HeapBytes   uint64 `json:"heapBytes"`
		HeapObjects uint64 `json:"heapObjects"`
		Goroutines  int    `json:"goroutines"`
	}
	report := struct {
		Schema         int     `json:"schemaVersion"`
		Scope          string  `json:"scope"`
		OS             string  `json:"os"`
		Arch           string  `json:"arch"`
		Passed         bool    `json:"passed"`
		Requests       int     `json:"requests"`
		WarmupCycles   int     `json:"warmupCycles"`
		Samples        []point `json:"samples"`
		HeapDelta      int64   `json:"heapMedianDelta"`
		ObjectDelta    int64   `json:"objectMedianDelta"`
		GoroutineDelta int64   `json:"goroutineMedianDelta"`
		ElapsedSeconds float64 `json:"elapsedSeconds"`
	}{Schema: 1, Scope: "Common engine real loopback proxy; post-GC retained heap and goroutines; no WebView/driver/body-tier claim", OS: runtime.GOOS, Arch: runtime.GOARCH, WarmupCycles: 8}
	begin := time.Now()
	t.Cleanup(func() {
		report.Passed = !t.Failed()
		report.ElapsedSeconds = time.Since(begin).Seconds()
		if destination := os.Getenv("ARCHSCOPE_CAPTURE_MEMORY_REPORT"); destination != "" {
			raw, e := json.MarshalIndent(report, "", "  ")
			if e == nil {
				e = os.WriteFile(destination, append(raw, '\n'), 0600)
			}
			if e != nil {
				t.Error(e)
			}
		}
	})
	root := t.TempDir()
	service := newCaptureServiceForPlatform(root, certstore.New(memoryTrustBackend{}, nil), "windows")
	defer service.shutdownAutomation()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "memory fixture")
	}))
	defer origin.Close()
	for cycle := 1; cycle <= cycles; cycle++ {
		runMemoryLifecycle(t, service, root, origin.URL)
		report.Requests += 512
		runtime.GC()
		runtime.GC() // Allow finalizers and the following collection to finish.
		time.Sleep(50 * time.Millisecond)
		var memory runtime.MemStats
		runtime.ReadMemStats(&memory)
		report.Samples = append(report.Samples, point{cycle, memory.HeapAlloc, memory.HeapObjects, runtime.NumGoroutine()})
	}
	measured := report.Samples[report.WarmupCycles:]
	median := func(points []point, field func(point) int64) int64 {
		values := make([]int64, len(points))
		for i, p := range points {
			values[i] = field(p)
		}
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		return (values[len(values)/2-1] + values[len(values)/2]) / 2
	}
	first, last := measured[:4], measured[len(measured)-4:]
	report.HeapDelta = median(last, func(p point) int64 { return int64(p.HeapBytes) }) - median(first, func(p point) int64 { return int64(p.HeapBytes) })
	report.ObjectDelta = median(last, func(p point) int64 { return int64(p.HeapObjects) }) - median(first, func(p point) int64 { return int64(p.HeapObjects) })
	report.GoroutineDelta = median(last, func(p point) int64 { return int64(p.Goroutines) }) - median(first, func(p point) int64 { return int64(p.Goroutines) })
	if report.HeapDelta > 8<<20 || report.ObjectDelta > 5000 || report.GoroutineDelta > 8 {
		t.Fatalf("post-warmup retained growth exceeds fixed screening budgets: heap=%d objects=%d goroutines=%d", report.HeapDelta, report.ObjectDelta, report.GoroutineDelta)
	}
}

func runMemoryLifecycle(t *testing.T, service *CaptureService, root, origin string) {
	t.Helper()
	session, err := service.StartCapture(capture.Config{ListenAddress: "127.0.0.1:0", RetentionRows: 64, LiveWindow: 64, RetainUnattributedMetadata: true})
	if err != nil {
		t.Fatal(err)
	}
	proxy, _ := url.Parse("http://" + session.ListenAddress)
	transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for batch := 0; batch < 2; batch++ {
		for n := 0; n < 256; n++ {
			response, e := client.Get(origin + "/memory?sequence=" + strconv.Itoa(n))
			if e != nil {
				t.Fatal(e)
			}
			_, e = io.Copy(io.Discard, response.Body)
			response.Body.Close()
			if e != nil || response.StatusCode != 200 {
				t.Fatal("synthetic response", e)
			}
		}
		if batch == 0 {
			if _, err = service.ClearCapture(string(session.ID)); err != nil {
				t.Fatal(err)
			}
		}
	}
	stopped, err := service.StopCapture(string(session.ID))
	if err != nil || stopped.State != capture.StateFinalized {
		t.Fatal("finalize", err)
	}
	stats, err := service.GetCaptureStats(string(session.ID))
	if err != nil || stats.Captured != 256 || stats.Persisted != 256 || stats.Retained != 64 || stats.Dropped != 0 {
		t.Fatal("final counters", stats, err)
	}
	listener, err := net.Listen("tcp4", session.ListenAddress)
	if err != nil {
		t.Fatal("listener leaked after stop", err)
	}
	listener.Close()
	// Remove only the just-finalized store under this test's verified temp root.
	rel, err := filepath.Rel(root, session.StorePath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		t.Fatal("test store escaped owned root")
	}
	if err = os.RemoveAll(session.StorePath); err != nil {
		t.Fatal(err)
	}
}
