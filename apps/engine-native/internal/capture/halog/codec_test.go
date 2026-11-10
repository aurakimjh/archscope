package halog

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestReferenceV764NativeFixture(t *testing.T) {
	b, err := os.ReadFile("testdata/reference-v764.halog")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	txs, err := f.Transactions()
	if err != nil || len(txs) != 1 {
		t.Fatal(err, len(txs))
	}
	x := txs[0]
	if x.Method != "POST" || x.URL != "http://127.0.0.1:8080/fixture?a=1&a=2" || x.StatusCode != 200 || x.State != models.TxComplete || x.TotalMS != 125 || !x.Starred {
		t.Fatalf("incorrect native metadata: %+v", x)
	}
	if x.Request.BodyPreview != "request 한글" || x.Response.BodyPreview != `{"marker":"response","message":"한글"}` || x.Request.BodyStorage != "inline" || x.Response.BodyStorage != "inline" {
		t.Fatal("reference payload mapping failed")
	}
	if x.StartedAt != time.Date(2026, 11, 10, 0, 0, 0, 0, time.Local).Format(time.RFC3339Nano) || x.Process.Name != "fixture.exe" || x.Process.Key.PID != 0 || x.Process.Attribution != "foreign_tool" || x.CaptureMode != "halog_import" {
		t.Fatal("time/process provenance mapping failed")
	}
	if header(x.Request.Headers, "X-Duplicate") != "one" || len(x.Request.Headers) != 3 || len(x.Response.Headers) != 3 || x.Request.Headers[2].Value != "two" || x.Response.Headers[2].Value != "blue" {
		t.Fatal("ordered duplicate headers lost")
	}
	encoded, err := f.Encode()
	if err != nil || !bytes.Equal(encoded, b) {
		t.Fatal("native wire round trip changed reference bytes", err)
	}
}

func TestReferenceV764StateMatrix(t *testing.T) {
	b, err := os.ReadFile("testdata/reference-v764-states.halog")
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := f.Transactions()
	if err != nil || len(rows) != 16 {
		t.Fatal("native state matrix invalid", err)
	}
	for i, row := range rows {
		// Original COM IsComplete ignores TimeEnd and SR_Result. Stage 4
		// closes a request; a completion tick also closes stage 1 requests.
		closed := i&8 != 0 || i&1 != 0
		if (row.State == models.TxComplete || row.State == models.TxFailed) != closed {
			t.Fatalf("native completion differs from reference case %d", i)
		}
		wantMS := float64(0)
		if i&1 != 0 {
			wantMS = 125
		}
		if row.TotalMS != wantMS {
			t.Fatalf("native timing differs from reference case %d", i)
		}
	}
}

func TestCanonicalNativeRoundTripAndRefusals(t *testing.T) {
	rows := []models.CaptureTransaction{{ID: "fixture", Method: "POST", URL: "https://example.test/a?x=1&x=2", HTTPVersion: "HTTP/1.1", StatusCode: 302, StatusText: "Found", StartedAt: "2026-11-10T00:00:00Z", EndedAt: "2026-11-10T00:00:00.125Z", TotalMS: 125, State: models.TxComplete, Starred: true, Request: models.HTTPMessage{BodyStorage: "inline", ContentType: "text/plain", BodyPreview: "한글"}, Response: models.HTTPMessage{BodyStorage: "inline", ContentType: "application/json", BodyPreview: `{"x":1}`}, RedirectURL: "https://example.test/b"}}
	b, err := EncodeTransactions(rows)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.Transactions()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].TotalMS != 125 || got[0].Request.BodyPreview != "한글" || got[0].Response.BodyPreview != `{"x":1}` || got[0].RedirectURL != rows[0].RedirectURL || !got[0].Starred {
		t.Fatal("canonical native round trip failed")
	}
	rows[0].StatusCode = 0
	rows[0].StatusText = ""
	rows[0].State = models.TxFailed
	failed, err := EncodeTransactions(rows)
	if err != nil {
		t.Fatal(err)
	}
	failedFile, err := Decode(failed)
	if err != nil {
		t.Fatal(err)
	}
	failedRows, err := failedFile.Transactions()
	if err != nil || failedRows[0].Response.BodyPreview != rows[0].Response.BodyPreview || failedRows[0].State != models.TxFailed {
		t.Fatal("unknown-status failure lost retained response", err)
	}
	rows[0].Request.Headers = []models.HeaderField{{Name: "X-Test", Value: "a\r\nInjected: yes"}}
	if _, err = EncodeTransactions(rows); err == nil {
		t.Fatal("header injection accepted")
	}
	rows[0].Request.Headers = nil
	rows[0].Comment = string([]byte{0xff})
	if _, err = EncodeTransactions(rows); err == nil {
		t.Fatal("ambiguous ANSI metadata silently corrupted")
	}
	for _, bad := range [][]byte{[]byte(`{"type":"archscope-http-session"}`), append(append([]byte{}, b...), 0), b[:len(b)-1]} {
		if _, err := Decode(bad); err == nil {
			t.Fatal("invalid native file accepted")
		}
	}
	mutated := bytes.Replace(b, []byte(requestClass), []byte(strings.Repeat("x", len(requestClass))), 1)
	if _, err = Decode(mutated); err == nil {
		t.Fatal("arbitrary native class accepted")
	}
}

func TestChunkLengths(t *testing.T) {
	for _, n := range []int{0, 1, 239, 240, 255, 256, 395, 495, 496, 2799, 2800, 65535, 65536, 100000} {
		w := writer{}
		w.intN(n)
		r := reader{data: w.Bytes()}
		if got := r.intN(); got != uint64(n) || r.err != nil || r.pos != w.Len() {
			t.Fatalf("chunk length %d: got %d, %v", n, got, r.err)
		}
	}
}

func TestNativePropertyTypesAndCodepages(t *testing.T) {
	b, _ := os.ReadFile("testdata/reference-v764.halog")
	reference, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := reference.Transactions()
	if err != nil {
		t.Fatal(err)
	}
	rows[0].Comment = "검증용 한글 주석"
	rows[0].Process.Name = "한글-fixture.exe"
	b, err = EncodeTransactionsCodePage(rows, 949)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.Transactions()
	if err != nil || got[0].Comment[:len(rows[0].Comment)] != rows[0].Comment || got[0].Process.Name != rows[0].Process.Name {
		t.Fatal("CP949 metadata damaged", err)
	}
	if got[0].Timings.ImportedHAR == nil || got[0].Timings.ImportedHAR.Send.MS != rows[0].Timings.ImportedHAR.Send.MS || got[0].Timings.ImportedHAR.Wait.MS != rows[0].Timings.ImportedHAR.Wait.MS || got[0].Timings.ImportedHAR.Receive.MS != rows[0].Timings.ImportedHAR.Receive.MS {
		t.Fatal("retained native timing phases lost")
	}
	// The original loader rejected an int32 Starred field despite accepting the
	// same numeric value as a byte. Check against types from its saved fixture.
	for _, pair := range [][2]*object{{reference.entries[0].object, f.entries[0].object}, {reference.entries[0].object.extra[0].object, f.entries[0].object.extra[0].object}, {reference.entries[0].object.extra[2].object, f.entries[0].object.extra[2].object}} {
		if len(pair[0].fields) != len(pair[1].fields) {
			t.Fatal("native field schema changed")
		}
		for i, old := range pair[0].fields {
			v := pair[1].fields[i]
			if v.name != old.name {
				t.Fatal("native field order changed")
			}
			if (old.value.tag == 0 || old.value.tag == 1) && (v.value.tag == 0 || v.value.tag == 1) {
				continue
			}
			if (old.value.tag == 11 || old.value.tag == 12) && (v.value.tag == 11 || v.value.tag == 12) {
				continue
			}
			if old.value.tag != v.value.tag {
				t.Fatalf("native type changed for %s: %d to %d", v.name, old.value.tag, v.value.tag)
			}
		}
	}
	if _, err = EncodeTransactionsCodePage(rows, 1252); err == nil {
		t.Fatal("unrepresentable metadata silently substituted")
	}
	if _, err = EncodeTransactionsCodePage(rows, 932); err == nil {
		t.Fatal("unsupported codepage accepted")
	}
}

func TestRequestBodyStoredOnlyInFullStream(t *testing.T) {
	b, _ := os.ReadFile("testdata/reference-v764.halog")
	f, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	o := f.entries[0].object
	for i := range o.fields {
		if o.fields[i].name == "ProcessName" {
			o.fields[i].value = str(`C:\fixture\fixture.exe[1234]`)
		}
	}
	o.extra[3] = memory(nil)
	o.extra[4] = memory([]byte("POST / HTTP/1.1\r\nContent-Length: 6\r\n\r\nstream"))
	rows, err := f.Transactions()
	if err != nil || rows[0].Request.BodyPreview != "stream" || rows[0].Process.Name != "fixture.exe" || rows[0].Process.Key.PID != 0 {
		t.Fatal("full-stream body/process label mapping failed", err)
	}
}

func TestPendingResponseRemainsIncomplete(t *testing.T) {
	for _, status := range []int{0, 403} {
		rows := []models.CaptureTransaction{{ID: "pending", Method: "POST", URL: "http://example.test/", State: models.TxReceiving, StatusCode: status, Request: models.HTTPMessage{BodyStorage: "inline"}, Response: models.HTTPMessage{BodyStorage: "inline"}}}
		if status > 0 {
			rows[0].Response.BodyPreview = "partial response"
		}
		b, err := EncodeTransactionsCodePage(rows, 1252)
		if err != nil {
			t.Fatal(err)
		}
		f, err := Decode(b)
		if err != nil {
			t.Fatal(err)
		}
		got, err := f.Transactions()
		if err != nil || got[0].State == models.TxComplete || got[0].StatusCode != status || got[0].Response.BodyPreview != rows[0].Response.BodyPreview || got[0].EndedAt != "" {
			t.Fatal("pending response was lost or finalized", err)
		}
	}
}

func TestPrivateReferenceSample(t *testing.T) {
	path := os.Getenv("ARCHSCOPE_HALOG_SOURCE")
	if path == "" {
		t.Skip("private local reference sample")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	f, err := Decode(b)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := f.Encode()
	if err != nil || !bytes.Equal(b, encoded) {
		t.Fatal("private native wire round trip differs", err)
	}
	rows, err := f.Transactions()
	if err != nil {
		t.Fatal(err)
	}
	if out := os.Getenv("ARCHSCOPE_HALOG_ROUNDTRIP"); out != "" {
		a, _ := filepath.Abs(path)
		z, _ := filepath.Abs(out)
		if strings.EqualFold(a, z) {
			t.Fatal("test output must differ from input")
		}
		if err = os.WriteFile(out, encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if out := os.Getenv("ARCHSCOPE_HALOG_PROJECTION"); out != "" {
		// Private, opt-in oracle input: compare decoded values with COM without
		// printing URLs, process labels or bodies in test logs/evidence.
		hash := func(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }
		projection := make([]map[string]any, 0, len(rows))
		for _, row := range rows {
			process := ""
			if row.Process != nil {
				process = row.Process.Name
			}
			projection = append(projection, map[string]any{"method": row.Method, "urlHash": hash(row.URL), "processHash": hash(process), "status": row.StatusCode, "state": row.State, "duration": row.TotalMS, "requestStorage": row.Request.BodyStorage, "responseStorage": row.Response.BodyStorage, "requestHash": hash(row.Request.BodyPreview), "responseHash": hash(row.Response.BodyPreview)})
		}
		data, err := json.Marshal(projection)
		if err != nil {
			t.Fatal(err)
		}
		a, _ := filepath.Abs(path)
		z, _ := filepath.Abs(out)
		if strings.EqualFold(a, z) {
			t.Fatal("projection must not overwrite source")
		}
		if err = os.WriteFile(out, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("native rows=%d bytes=%d SHA256=%x; contents retained locally", len(rows), len(b), sha256.Sum256(b))
}

func FuzzDecode(f *testing.F) {
	b, _ := os.ReadFile("testdata/reference-v764.halog")
	f.Add(b)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		if decoded, err := Decode(b); err == nil {
			_, _ = decoded.Transactions()
		}
	})
}
