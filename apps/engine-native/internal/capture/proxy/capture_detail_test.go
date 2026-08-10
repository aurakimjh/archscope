package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// startPlainProxy boots a loopback proxy for the plain-HTTP path and returns
// its address plus the channel finalized transactions arrive on.
func startPlainProxy(t *testing.T, cfg Config) (string, <-chan models.CaptureTransaction) {
	t.Helper()
	authority, err := NewAuthority(time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(authority.Close)
	cfg.ListenAddress = "127.0.0.1:0"
	cfg.Authority = authority
	server, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	captured := make(chan models.CaptureTransaction, 4)
	address, err := server.Start(context.Background(), capture.Config{}, func(tx models.CaptureTransaction) error {
		captured <- tx
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { stopServer(t, server) })
	return address, captured
}

// proxyGet drives one request through the proxy and drains the response so the
// receive phase and the response body recorder both see the full payload.
func proxyGet(t *testing.T, proxyAddress, target string, header http.Header) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, values := range header {
		for _, value := range values {
			request.Header.Add(name, value)
		}
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL(proxyAddress))}}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.ReadAll(response.Body)
	response.Body.Close()
}

func awaitTransaction(t *testing.T, captured <-chan models.CaptureTransaction) models.CaptureTransaction {
	t.Helper()
	select {
	case tx := <-captured:
		return tx
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for a finalized transaction")
		return models.CaptureTransaction{}
	}
}

// A finalized MITM transaction must carry a proxy-upstream phase breakdown, not
// just a total. Without it the analyst sees one number and cannot tell a slow
// DNS lookup from a slow origin.
func TestMITMRecordsUpstreamTimingPhases(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_, _ = io.WriteString(writer, "hello")
	}))
	defer origin.Close()

	address, captured := startPlainProxy(t, Config{})
	proxyGet(t, address, origin.URL+"/timed", nil)
	tx := awaitTransaction(t, captured)

	if tx.Timings.ProxyUpstream == nil {
		t.Fatal("proxyUpstream timings are absent; the waterfall has nothing to draw")
	}
	upstream := *tx.Timings.ProxyUpstream
	if upstream.Connect.State != models.TimingKnown {
		t.Fatalf("connect state=%q, want known on a fresh connection", upstream.Connect.State)
	}
	if upstream.Wait.State != models.TimingKnown || upstream.Wait.MS <= 0 {
		t.Fatalf("wait=%+v, want a positive TTFB", upstream.Wait)
	}
	if upstream.Receive.State != models.TimingKnown {
		t.Fatalf("receive state=%q, want known once the body was read", upstream.Receive.State)
	}
	// The observation point the phases belong to must travel with them.
	if tx.Timings.ProxyInternal == nil {
		t.Fatal("proxyInternal timings are absent; the proxy's own leg is unattributed")
	}
	if tx.Timings.ImportedHAR != nil {
		t.Fatal("a live capture must never claim imported-HAR timings")
	}
	if tx.Timings.ProxyInternal.DNS.State != models.TimingNotApplicable {
		t.Fatalf("proxyInternal dns=%q, want not_applicable at the proxy observation point",
			tx.Timings.ProxyInternal.DNS.State)
	}
}

// A reused connection did not resolve, dial, or handshake. Reporting 0 ms would
// let the waterfall imply the work happened instantly (INV-5).
func TestReusedConnectionMarksHandshakePhasesNotApplicable(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(writer, "ok")
	}))
	defer origin.Close()

	address, captured := startPlainProxy(t, Config{})
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL(address))}}
	for index := 0; index < 2; index++ {
		response, err := client.Get(origin.URL + "/keepalive")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.ReadAll(response.Body)
		response.Body.Close()
	}
	// Each proxied request builds its own upstream transport today, so the
	// assertion is on the model contract rather than on pooling: whenever the
	// transport reports reuse, the handshake phases must say not_applicable.
	for index := 0; index < 2; index++ {
		tx := awaitTransaction(t, captured)
		if tx.Timings.ProxyUpstream == nil {
			t.Fatal("proxyUpstream timings are absent")
		}
		if !tx.UsedExistingConnection {
			continue
		}
		upstream := *tx.Timings.ProxyUpstream
		for name, phase := range map[string]models.Duration{
			"dns": upstream.DNS, "connect": upstream.Connect, "tls": upstream.TLS,
		} {
			if phase.State != models.TimingNotApplicable {
				t.Fatalf("reused connection %s state=%q, want not_applicable", name, phase.State)
			}
		}
	}
}

// Bodies stay omitted in every startable session while SEC-10 is pending. This
// must not regress into "captured because the dormant plumbing exists".
func TestBodiesStayOmittedWithoutOptIn(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(writer, `{"secretish":"payload"}`)
	}))
	defer origin.Close()

	address, captured := startPlainProxy(t, Config{})
	proxyGet(t, address, origin.URL+"/json", nil)
	tx := awaitTransaction(t, captured)

	if tx.Response.BodyStorage != BodyStorageOmitted {
		t.Fatalf("response bodyStorage=%q, want %q by default", tx.Response.BodyStorage, BodyStorageOmitted)
	}
	if tx.Response.BodyPreview != "" {
		t.Fatalf("response bodyPreview=%q, want empty by default", tx.Response.BodyPreview)
	}
	// Byte accounting must still work; omission is about retention, not counting.
	if tx.Response.BodySize <= 0 {
		t.Fatalf("response bodySize=%d, want the transferred byte count", tx.Response.BodySize)
	}
}

func TestDormantBodyCoreCapturesDecodedBodies(t *testing.T) {
	payload := `{"order":{"id":42}}`
	recorder := newBodyRecorder(io.NopCloser(strings.NewReader(payload)), true, 0)
	if _, err := io.Copy(io.Discard, recorder); err != nil {
		t.Fatal(err)
	}
	message := models.HTTPMessage{}
	recorder.apply(&message, "identity")

	if message.BodyStorage != BodyStorageInline {
		t.Fatalf("bodyStorage=%q, want %q", message.BodyStorage, BodyStorageInline)
	}
	if message.BodyPreview != payload {
		t.Fatalf("bodyPreview=%q, want %q", message.BodyPreview, payload)
	}
	if recorder.buffer.Len() != 0 {
		t.Fatalf("unredacted recorder buffer retained %d bytes after apply", recorder.buffer.Len())
	}
}

// A body over the per-message cap must be reported as truncated rather than
// presented as if it were the whole payload.
func TestDormantBodyCoreTruncatesOversizedBodies(t *testing.T) {
	payload := strings.Repeat("A", 4096)
	recorder := newBodyRecorder(io.NopCloser(strings.NewReader(payload)), true, 512)
	if _, err := io.Copy(io.Discard, recorder); err != nil {
		t.Fatal(err)
	}
	message := models.HTTPMessage{}
	recorder.apply(&message, "identity")

	if message.BodyStorage != BodyStorageTruncated {
		t.Fatalf("bodyStorage=%q, want %q", message.BodyStorage, BodyStorageTruncated)
	}
	if len(message.BodyPreview) != 512 {
		t.Fatalf("preview length=%d, want the 512-byte cap", len(message.BodyPreview))
	}
	if message.BodySize != int64(len(payload)) {
		t.Fatalf("bodySize=%d, want the full transferred size %d", message.BodySize, len(payload))
	}
}

func TestDormantBodyCoreDecodesGzipBodies(t *testing.T) {
	payload := `{"compressed":true}`
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err := io.WriteString(gzipWriter, payload); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	recorder := newBodyRecorder(io.NopCloser(bytes.NewReader(compressed.Bytes())), true, 0)
	if _, err := io.Copy(io.Discard, recorder); err != nil {
		t.Fatal(err)
	}
	message := models.HTTPMessage{}
	recorder.apply(&message, "gzip")

	if message.BodyEncoding != "gzip" {
		t.Fatalf("bodyEncoding=%q, want gzip", message.BodyEncoding)
	}
	if message.BodyPreview != payload {
		t.Fatalf("bodyPreview=%q, want the decoded payload %q", message.BodyPreview, payload)
	}
}

func TestCompressedBodyExpansionUsesThePerMessageCap(t *testing.T) {
	payload := strings.Repeat("Z", 4096)
	var compressed bytes.Buffer
	gzipWriter := gzip.NewWriter(&compressed)
	if _, err := io.WriteString(gzipWriter, payload); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}

	recorder := newBodyRecorder(io.NopCloser(bytes.NewReader(compressed.Bytes())), true, 512)
	if _, err := io.Copy(io.Discard, recorder); err != nil {
		t.Fatal(err)
	}
	message := models.HTTPMessage{}
	recorder.apply(&message, "gzip")

	if message.BodyStorage != BodyStorageTruncated || len(message.BodyPreview) != 512 {
		t.Fatalf("expanded body storage=%q preview=%d, want truncated at 512", message.BodyStorage, len(message.BodyPreview))
	}
}

// The status cell renders StatusCode and StatusText side by side, so StatusText
// must be the reason phrase alone.
func TestStatusTextExcludesTheNumericCode(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNotFound)
	}))
	defer origin.Close()

	address, captured := startPlainProxy(t, Config{})
	proxyGet(t, address, origin.URL+"/missing", nil)
	tx := awaitTransaction(t, captured)

	if tx.StatusCode != http.StatusNotFound {
		t.Fatalf("statusCode=%d, want 404", tx.StatusCode)
	}
	if tx.StatusText != "Not Found" {
		t.Fatalf("statusText=%q, want the reason phrase alone", tx.StatusText)
	}
}
