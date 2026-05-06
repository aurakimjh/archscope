package html

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// TestWriteRendersAnalysisResult mirrors Python
// `test_html_report_exporter_renders_analysis_result`: build an
// analysis-result payload similar to access-log-result.json and verify
// the rendered HTML contains the expected structural anchors.
func TestWriteRendersAnalysisResult(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "nested", "report.html")

	payload := map[string]any{
		"type":         "access_log",
		"source_files": []any{"sample.log"},
		"created_at":   "2026-04-29T10:00:00Z",
		"summary": map[string]any{
			"total_requests":  6,
			"avg_response_ms": 648.67,
			"error_rate":      33.33,
		},
		"series": map[string]any{},
		"tables": map[string]any{
			"top_urls_by_avg_response_time": []any{
				map[string]any{
					"uri":        "/api/orders",
					"avg_response_ms": 1820.5,
					"count":      2,
				},
				map[string]any{
					"uri":        "/api/users",
					"avg_response_ms": 200.0,
					"count":      4,
				},
			},
		},
		"charts": map[string]any{},
		"metadata": map[string]any{
			"parser":         "nginx_combined_with_response_time",
			"schema_version": "0.1.0",
			"findings": []any{
				map[string]any{
					"severity": "warning",
					"code":     "high_response_time",
					"message":  "URI /api/orders averaging >1s",
				},
			},
		},
	}

	if err := Write(out, payload); err != nil {
		t.Fatalf("Write: %v", err)
	}

	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	html := string(body)

	mustContain(t, html, "<!doctype html>")
	mustContain(t, html, "ArchScope Analysis Report - access_log")
	mustContain(t, html, "Summary")
	mustContain(t, html, "top_urls_by_avg_response_time")
	mustContain(t, html, "Findings")
	mustContain(t, html, "high_response_time")
	mustContain(t, html, "<style>")
	// CSS should be inlined (single-file portability).
	mustContain(t, html, ".flame-bar")
}

// TestWriteRendersDebugLog mirrors Python
// `test_html_report_exporter_renders_parser_debug_log`.
func TestWriteRendersDebugLog(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "debug.html")

	payload := map[string]any{
		"environment": map[string]any{"archscope_version": "0.1.0"},
		"context": map[string]any{
			"analyzer_type":     "access_log",
			"parser":            "nginx",
			"source_file_name":  "access.log",
			"encoding_detected": "utf-8",
		},
		"redaction": map[string]any{"enabled": true},
		"summary":   map[string]any{"verdict": "PARTIAL_SUCCESS", "skipped": 1},
		"errors_by_type": map[string]any{
			"INVALID_NUMBER": map[string]any{
				"count":          1,
				"description":    "bad response time",
				"failed_pattern": "ACCESS_LOG",
				"samples": []any{
					map[string]any{"line_number": 2, "message": "bad"},
				},
			},
		},
		"exceptions": []any{},
		"hints":      []any{"Check response-time suffix."},
	}

	if err := Write(out, payload); err != nil {
		t.Fatalf("Write: %v", err)
	}

	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	html := string(body)

	mustContain(t, html, "ArchScope Parser Debug Report - access_log")
	mustContain(t, html, "INVALID_NUMBER")
	mustContain(t, html, "Check response-time suffix.")
	mustContain(t, html, "<ul>")
	mustContain(t, html, "Parse Errors")
}

func TestWriteAcceptsAnalysisResultStruct(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "result.html")

	r := models.New("access_log", "nginx_combined_with_response_time")
	r.SourceFiles = []string{"access.log"}
	r.Summary = map[string]any{"total_requests": 1}
	r.Tables = map[string]any{
		"sample_records": []map[string]any{
			{"uri": "/api/orders", "status": 200},
		},
	}
	r.AddFinding("warning", "high_latency", "slow", nil)

	if err := Write(out, r); err != nil {
		t.Fatalf("Write: %v", err)
	}

	body, _ := os.ReadFile(out)
	html := string(body)
	mustContain(t, html, "ArchScope Analysis Report - access_log")
	mustContain(t, html, "/api/orders")
	mustContain(t, html, "high_latency")
	mustContain(t, html, "sample_records")
}

func TestWriteCreatesParentDirs(t *testing.T) {
	tmp := t.TempDir()
	out := filepath.Join(tmp, "deep", "nested", "missing", "dir", "report.html")
	if err := Write(out, map[string]any{"type": "custom"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(out); err != nil {
		t.Fatalf("output not created: %v", err)
	}
}

// TestEscapesUntrustedStrings: the renderer must not let raw HTML in
// payload values reach the output (XSS guard).
func TestEscapesUntrustedStrings(t *testing.T) {
	body, err := Marshal(map[string]any{
		"type":         "<script>alert(1)</script>",
		"source_files": []any{"<img src=x onerror=alert(1)>"},
		"summary":      map[string]any{"label": "a & b"},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	html := string(body)
	if strings.Contains(html, "<script>alert(1)</script>") {
		t.Errorf("raw <script> reached output: %q", html)
	}
	if strings.Contains(html, "<img src=x onerror=alert(1)>") {
		t.Errorf("raw <img onerror> reached output: %q", html)
	}
	if !strings.Contains(html, "&lt;script&gt;") && !strings.Contains(html, "&lt;script&#") {
		t.Errorf("expected escaped <script>; got %q", html[:min(2000, len(html))])
	}
	if !strings.Contains(html, "&amp;") {
		t.Errorf("expected & to be escaped: %q", html)
	}
}

// TestMarshalIsDeterministic: two consecutive marshals should produce
// the same bytes — important for snapshot review and report-diff.
func TestMarshalIsDeterministic(t *testing.T) {
	payload := map[string]any{
		"type":         "access_log",
		"source_files": []any{"a.log", "b.log"},
		"summary":      map[string]any{"a": 1, "b": 2, "c": 3},
		"series":       map[string]any{"x": []any{}, "y": []any{}, "z": []any{}},
		"tables":       map[string]any{},
		"charts":       map[string]any{},
		"metadata":     map[string]any{"parser": "p", "schema_version": "0.1.0", "findings": []any{}},
	}
	first, err := Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for i := 0; i < 4; i++ {
		next, err := Marshal(payload)
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		if string(first) != string(next) {
			t.Fatalf("non-deterministic output on iteration %d", i)
		}
	}
}

func TestRendersFlamegraph(t *testing.T) {
	payload := map[string]any{
		"type":         "profiler",
		"source_files": []any{"sample.jfr"},
		"summary":      map[string]any{},
		"series":       map[string]any{},
		"tables":       map[string]any{},
		"charts": map[string]any{
			"flamegraph": map[string]any{
				"name":    "root",
				"samples": 100,
				"children": []any{
					map[string]any{"name": "main", "samples": 80, "children": []any{}},
					map[string]any{"name": "init", "samples": 20, "children": []any{}},
				},
			},
		},
		"metadata": map[string]any{"parser": "jfr", "schema_version": "0.1.0", "findings": []any{}},
	}
	body, err := Marshal(payload)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	html := string(body)
	mustContain(t, html, `class="flamegraph"`)
	mustContain(t, html, `class="flame-bar"`)
	mustContain(t, html, "main")
	mustContain(t, html, "init")
}

func TestTitleOverride(t *testing.T) {
	body, err := MarshalWithOptions(map[string]any{"type": "access_log"}, Options{Title: "Custom Title"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	html := string(body)
	mustContain(t, html, "<title>Custom Title</title>")
	if strings.Contains(html, "ArchScope Analysis Report") {
		t.Errorf("default title leaked when override given: %q", html)
	}
}

func TestSourcePathRendered(t *testing.T) {
	body, err := MarshalWithOptions(map[string]any{"type": "access_log"}, Options{SourcePath: "/tmp/in.json"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	mustContain(t, string(body), "/tmp/in.json")
}

func TestRendersUTF8Verbatim(t *testing.T) {
	body, err := Marshal(map[string]any{
		"type":         "access_log",
		"source_files": []any{"한글.log"},
		"summary":      map[string]any{"label": "메시지"},
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(body), "한글.log") {
		t.Errorf("non-ASCII source file not preserved")
	}
	if !strings.Contains(string(body), "메시지") {
		t.Errorf("non-ASCII summary value not preserved")
	}
}

// TestOutputIsValidUTF8AndStandalone: the rendered file must be a
// complete HTML document with <!doctype>, <html>, <head>, <body>, and
// no external <link> or <script src=...> references (single-file
// portability).
func TestOutputIsStandalone(t *testing.T) {
	body, err := Marshal(map[string]any{"type": "access_log"})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	html := string(body)
	mustContain(t, html, "<!doctype html>")
	mustContain(t, html, "<html lang=\"en\">")
	mustContain(t, html, "</html>")
	if strings.Contains(html, "<link ") {
		t.Errorf("unexpected <link>; CSS should be inlined")
	}
	if strings.Contains(html, "<script src=") {
		t.Errorf("unexpected external <script src>; report must be standalone")
	}
}

// TestAcceptsJSONBytesEquivalentInput: the Python exporter is invoked
// with JSON read from a file. We accept any JSON-marshalable Go input
// — verify decoding a raw JSON document into a map matches direct
// map input.
func TestAcceptsJSONDecodedInput(t *testing.T) {
	raw := []byte(`{"type":"access_log","summary":{"total_requests":5}}`)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	body, err := Marshal(decoded)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	mustContain(t, string(body), "ArchScope Analysis Report - access_log")
	mustContain(t, string(body), "total_requests")
}

func mustContain(t *testing.T, body, needle string) {
	t.Helper()
	if !strings.Contains(body, needle) {
		// Truncate so failure isn't unreadable.
		preview := body
		if len(preview) > 4000 {
			preview = preview[:4000] + "...[truncated]"
		}
		t.Errorf("expected output to contain %q; got %s", needle, preview)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
