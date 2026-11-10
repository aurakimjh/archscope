package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/certstore"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/sessionfile"
)

func TestCapturePhysicalRetentionCountersAnalysisAndPortableExport(t *testing.T) {
	s := newCaptureServiceForPlatform(t.TempDir(), certstore.New(memoryTrustBackend{}, nil), "windows")
	defer s.shutdownAutomation()
	started, e := s.StartCapture(capture.Config{RetentionRows: 2, ProcessScope: "all_sessions", ListenAddress: "127.0.0.1:0", RetainUnattributedMetadata: true})
	if e != nil {
		t.Fatal(e)
	}
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, "synthetic") }))
	defer origin.Close()
	proxy, _ := url.Parse("http://" + started.ListenAddress)
	transport := &http.Transport{Proxy: http.ProxyURL(proxy)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport}
	for i := 0; i < 4; i++ {
		response, e := client.Get(origin.URL)
		if e != nil {
			t.Fatal(e)
		}
		io.Copy(io.Discard, response.Body)
		response.Body.Close()
	}
	if _, e = s.StopCapture(string(started.ID)); e != nil {
		t.Fatal(e)
	}
	meta, e := s.manager.Manifest(started.ID)
	if e != nil || meta.Counters.Persisted != 4 || meta.Counters.Rotated != 2 || meta.RetainedRecords != 2 {
		t.Fatal(meta, e)
	}
	result, e := s.AnalyzeCaptureSession(CaptureAnalyzeRequest{SessionID: string(started.ID)})
	if e != nil || result.Metadata.Extra["retention"] == nil {
		t.Fatal("analysis hid retention", e)
	}
	path := filepath.Join(t.TempDir(), "retained.httpsession")
	count, e := s.ExportCaptureFile(CaptureExportRequest{Source: CaptureFileSource{SessionID: string(started.ID)}, Destination: path, Format: "session", Scope: "all"})
	if e != nil || count != 2 {
		t.Fatal(count, e)
	}
	doc, _, e := sessionfile.Read(path)
	if e != nil || len(doc.Transactions) != 2 || doc.Retention == nil || doc.Retention.Evicted != 2 {
		t.Fatal(doc.Retention, e)
	}
}
