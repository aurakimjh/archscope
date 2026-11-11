package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/analysis"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/catalog"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/ingestion"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
	c "github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
)

func evidenceBridge(t *testing.T, registry *c.Registry) *EngineService {
	t.Helper()
	cat, defaults, e := catalog.Builtin()
	if e != nil {
		t.Fatal(e)
	}
	if registry == nil {
		registry = defaults
	} else {
		cat, e = ingestion.NewCatalog(nil, nil, nil)
		if e != nil {
			t.Fatal(e)
		}
	}
	service, e := analysis.New(t.TempDir(), cat, registry)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(service.Close)
	return &EngineService{evidenceService: service}
}
func TestC4WailsJobBridgeAndBackendRevisionExport(t *testing.T) {
	svc := evidenceBridge(t, nil)
	body, _ := catalog.Contracts.ReadFile("contracts/server.log")
	path := filepath.Join(t.TempDir(), "server.log")
	os.WriteFile(path, body, 0600)
	job, e := svc.ImportEvidence(EvidenceImportRequest{Path: path, Importer: "tomcat", OptionsJSON: `{"top_n":3}`})
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	job, e = svc.evidenceService.Wait(ctx, job.ID, job.Generation)
	if e != nil || job.State != "completed" {
		t.Fatal(job, e)
	}
	raw, e := svc.GetEvidenceResultJSON(job.ResultHandle)
	if e != nil {
		t.Fatal(e)
	}
	var result models.AnalysisResult
	if e = json.Unmarshal([]byte(raw), &result); e != nil {
		t.Fatal(e)
	}
	future, _ := catalog.Contracts.ReadFile("contracts/wire-cases.json")
	result.Metadata.Extra["future"] = json.RawMessage(future)
	extended, _ := json.Marshal(result)
	source := filepath.Join(t.TempDir(), "extended.json")
	os.WriteFile(source, extended, 0600)
	view, e := svc.LoadEvidenceResult(source)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	text, e := svc.GetEvidenceResultJSON(view.Handle)
	if e != nil || !strings.Contains(text, "9007199254740993") {
		t.Fatal("backend precision loss", e)
	}
	// Simulate Wails' JSON string transport separately from Go persistence. No
	// actual native WebView execution is claimed by this bridge integration test.
	transport, _ := json.Marshal(struct {
		Text string `json:"text"`
	}{text})
	var received struct {
		Text string `json:"text"`
	}
	json.Unmarshal(transport, &received)
	if received.Text != text {
		t.Fatal("opaque Wails string changed")
	}
	for _, format := range []string{"json", "csv", "html", "report-pack"} {
		out := filepath.Join(t.TempDir(), format)
		if e = svc.ExportEvidenceResult(view.Handle, out, format); e != nil {
			t.Fatal(format, e)
		}
		data, e := os.ReadFile(out)
		if e != nil {
			t.Fatal(e)
		}
		if format == "html" && strings.Contains(string(data), "<script>alert(1)</script>") {
			t.Fatal("opaque metadata became executable HTML")
		}
		if format == "json" && string(data) != text {
			t.Fatal("export came from a JS projection")
		}
		if format == "report-pack" {
			z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
			if e != nil {
				t.Fatal(e)
			}
			found := false
			for _, f := range z.File {
				if strings.HasPrefix(f.Name, "sources/") {
					r, _ := f.Open()
					body, _ := io.ReadAll(r)
					r.Close()
					found = true
					if string(body) != text {
						t.Fatal("Report Pack source appendix lost opaque metadata")
					}
				}
			}
			if !found {
				t.Fatal("missing pack source")
			}
		}
	}
}

func TestW10LegacyProfilerThroughOpaqueBridgeAndReportPack(t *testing.T) {
	svc := evidenceBridge(t, nil)
	r := profiler.AnalyzeCollapsedStacks(map[string]int{"root;leaf": 3}, "synthetic.collapsed", profiler.ParserDiagnostics{}, profiler.Options{IntervalMS: 100, TopN: 3})
	future, _ := catalog.Contracts.ReadFile("contracts/wire-cases.json")
	r.Metadata.Extra = map[string]json.RawMessage{"future": future}
	body, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	path := filepath.Join(t.TempDir(), "legacy.json")
	os.WriteFile(path, body, 0600)
	view, e := svc.LoadEvidenceResult(path)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	text, e := svc.GetEvidenceResultJSON(view.Handle)
	if e != nil || text != string(body) {
		t.Fatal("legacy opaque bridge changed bytes", e)
	}
	for _, format := range []string{"json", "report-pack"} {
		out, e := svc.evidenceService.Export(view.Handle, format)
		if e != nil || len(out) == 0 {
			t.Fatal(format, e)
		}
	}
}
func TestW07NullZeroAbsentThroughBridgeCSVHTMLAndExactDiff(t *testing.T) {
	schema := c.ValueSchema{Nullable: true, Encodings: []string{"json_number", "exact_integer_string", "decimal_string"}, Kinds: []string{"count"}, Units: []string{"count"}, Aggregations: []string{"sum"}}
	registry, e := c.New(c.Descriptor{Type: "wire_fixture", Payload: c.Range{Min: "1.0.0", Before: "1.1.0"}, Envelope: c.Range{Min: "1.0.0", Before: "2.0.0"}, Extensions: map[string]c.Range{"measurement": {Min: "1.0.0", Before: "1.1.0"}}, Scalars: map[string]c.ValueSchema{"/summary/value": schema}})
	if e != nil {
		t.Fatal(e)
	}
	svc := evidenceBridge(t, registry)
	for _, state := range []string{"observed", "unavailable", "absent"} {
		r := models.New("wire_fixture", "fixture")
		r.Metadata.SchemaVersion = "1.0.0"
		m := c.Measurement{Kind: "count", Unit: "count", Scope: "fixture", Aggregation: "sum", Encoding: "json_number", Availability: state}
		if state == "observed" {
			r.Summary["value"] = 0
		} else if state == "unavailable" {
			r.Summary["value"] = nil
			m.Reason = "not_measured"
		}
		if state != "absent" {
			r.Metadata.Extra["extensions"] = map[string]string{"measurement": "1.0.0"}
			r.Metadata.Extra["measurement"] = map[string]c.Measurement{"/summary/value": m}
		}
		raw, _ := json.Marshal(r)
		path := filepath.Join(t.TempDir(), state+".json")
		os.WriteFile(path, raw, 0600)
		view, e := svc.LoadEvidenceResult(path)
		if e != nil || !view.Compatibility.Allowed {
			t.Fatal(view, e)
		}
		scalar, e := svc.GetEvidenceScalar(view.Handle, "/summary/value")
		if e != nil {
			t.Fatal(e)
		}
		if state == "observed" && (scalar.Value == nil || *scalar.Value != "0") {
			t.Fatal(scalar)
		}
		if state != "observed" && scalar.Value != nil {
			t.Fatal("missing became zero", scalar)
		}
		if state == "absent" && scalar.Present {
			t.Fatal(scalar)
		}
		delta, e := svc.CompareEvidenceScalar(view.Handle, view.Handle, "/summary/value")
		if e != nil {
			t.Fatal(e)
		}
		if state != "observed" && delta.Value != nil {
			t.Fatal("unknown delta became zero")
		}
		for _, format := range []string{"csv", "html"} {
			body, e := svc.evidenceService.Export(view.Handle, format)
			if e != nil {
				t.Fatal(e)
			}
			if state == "unavailable" {
				if !strings.Contains(string(body), "not_measured") {
					t.Fatal("availability reason missing", format)
				}
				if format == "html" && !strings.Contains(string(body), "—") {
					t.Fatal("HTML missing unavailable mark")
				}
				if format == "csv" && strings.Contains(string(body), "value,0") {
					t.Fatal("CSV fabricated zero")
				}
			}
		}
	}
}

func TestReviewR01DeepWailsLoadEvidenceResultBridge(t *testing.T) {
	svc := evidenceBridge(t, nil)
	input := filepath.Join(t.TempDir(), "deep.collapsed")
	frames := make([]string, 256)
	for i := range frames {
		frames[i] = fmt.Sprintf("synthetic.Frame%d", i)
	}
	os.WriteFile(input, []byte(strings.Join(frames, ";")+" 3\n"), 0600)
	result, e := profiler.AnalyzeCollapsedFile(input, profiler.Options{})
	if e != nil {
		t.Fatal(e)
	}
	raw, e := json.Marshal(result)
	if e != nil {
		t.Fatal(e)
	}
	saved := filepath.Join(t.TempDir(), "deep.json")
	os.WriteFile(saved, raw, 0600)
	view, e := svc.LoadEvidenceResult(saved)
	if e != nil || !view.Compatibility.Allowed {
		t.Fatal(view, e)
	}
	again, e := svc.GetEvidenceResultJSON(view.Handle)
	if e != nil || again != string(raw) {
		t.Fatal("deep bridge roundtrip failed", e)
	}
	for _, format := range []string{"json", "html", "csv", "report-pack"} {
		if body, e := svc.evidenceService.Export(view.Handle, format); e != nil || len(body) == 0 {
			t.Fatal(format, e)
		}
	}
}
