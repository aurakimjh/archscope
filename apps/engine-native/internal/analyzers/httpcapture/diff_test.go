package httpcapture

import (
	"encoding/json"
	"errors"
	"math/rand"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/capture/redact"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/httpcapture"
)

func TestURLTemplateVersionOneRulesAndQueryKeys(t *testing.T) {
	tests := []struct {
		path, query, want string
	}{
		{"/users/123/orders", "z=secret&a=one&a=two", "/users/{id}/orders?a&z"},
		{"/users/550e8400-e29b-41d4-a716-446655440000", "", "/users/{uuid}"},
		{"/asset/0123456789abcdef0123", "", "/asset/{hash}"},
		{"/token/AbCdEfGhIjKlMnOpQrStUv", "", "/token/{token}"},
		{"/owner/alice@example.com", "", "/owner/{email}"},
		{"/static/orders", "token=DO-NOT-KEEP", "/static/orders?token"},
	}
	for _, test := range tests {
		if got := templatePathAndQuery(test.path, test.query); got != test.want {
			t.Errorf("templatePathAndQuery(%q, %q)=%q, want %q", test.path, test.query, got, test.want)
		}
	}
}

func TestDiffSourceProjectionIsBoundedAndCrossDimensionTotalsReconcile(t *testing.T) {
	entries := []models.CaptureTransaction{
		diffEntry("one", "2026-07-30T01:00:00Z", "GET", "api", "/a/1", 200, 10),
		diffEntry("two", "2026-07-30T01:00:01Z", "GET", "api", "/b/2", 200, 20),
		diffEntry("three", "2026-07-30T01:00:02Z", "GET", "worker", "/c/3", 500, 30),
		diffEntry("four", "2026-07-30T01:00:03Z", "POST", "worker", "/d/4", 200, 40),
	}
	result := BuildParsed(parser.ParseResult{
		Format: "har", Dialect: "chrome", Entries: entries,
		Redaction: redact.Summary{Known: true}, TimelineAvailable: true,
	}, "bounded.har", Options{TopN: 1, DiffTemplateLimit: 2})
	projection := result.Metadata.Extra["http_capture_diff_source"].(DiffSourceProjection)
	if len(projection.Endpoints) != 3 {
		t.Fatalf("expected two kept templates plus {other}, got %+v", projection.Endpoints)
	}
	if projection.TemplatesFolded != 2 {
		t.Fatalf("templates_folded=%d, want 2", projection.TemplatesFolded)
	}
	if projection.EndpointTotal != len(entries) || projection.HostTotal != len(entries) {
		t.Fatalf("cross-dimension totals do not reconcile: %+v", projection)
	}
	if projection.ProcessAvailable || len(projection.Processes) != 0 {
		t.Fatalf("HAR pseudo-process dimension must be disabled: %+v", projection)
	}
	if codes := findingCodes(result); !codes["HTTP_DIFF_DIMENSIONS_FOLDED"] {
		t.Fatalf("bounded projection did not disclose folded dimensions: %+v", result.Metadata.Findings)
	}
	if got := result.Tables["transactions"].([]map[string]any); len(got) != 1 {
		t.Fatalf("fixture must prove diff projection is independent of bounded inline rows: %+v", got)
	}
}

func TestAnalyzeDiffReorderedEquivalentSessionsCompareEqual(t *testing.T) {
	entries := []models.CaptureTransaction{
		diffEntry("one", "2026-07-30T01:00:00Z", "GET", "api.example", "/users/1?ignored=true", 200, 20),
		diffEntry("two", "2026-07-30T01:00:01Z", "GET", "api.example", "/users/2", 200, 30),
		diffEntry("three", "2026-07-30T01:00:02Z", "POST", "api.example", "/orders/0123456789abcdef", 500, 80),
	}
	reordered := append([]models.CaptureTransaction(nil), entries...)
	rand.New(rand.NewSource(582)).Shuffle(len(reordered), func(i, j int) {
		reordered[i], reordered[j] = reordered[j], reordered[i]
	})
	before := resultPayload(t, BuildParsed(parser.ParseResult{
		Format: "har", Dialect: "chrome", Entries: entries,
		Redaction: redact.Summary{Known: true}, TimelineAvailable: true,
	}, "same.har", Options{}))
	after := resultPayload(t, BuildParsed(parser.ParseResult{
		Format: "har", Dialect: "chrome", Entries: reordered,
		Redaction: redact.Summary{Known: true}, TimelineAvailable: true,
	}, "same.har", Options{}))

	result, err := AnalyzeDiff(before, after, DiffOptions{})
	if err != nil {
		t.Fatalf("AnalyzeDiff: %v", err)
	}
	for _, name := range []string{"endpoints_changed", "endpoints_added", "endpoints_removed", "hosts_changed", "processes_changed"} {
		rows := result.Tables[name].([]map[string]any)
		if len(rows) != 0 {
			t.Fatalf("%s must be empty for reordered equivalent sessions: %+v", name, rows)
		}
	}
	if len(result.Metadata.Findings) != 0 {
		t.Fatalf("equivalent aligned sessions produced findings: %+v", result.Metadata.Findings)
	}
	delta := result.Summary["delta"].(map[string]any)
	if delta["count"] != 0 || delta["errors"] != 0 || delta["duration_p95_ms"] != float64(0) {
		t.Fatalf("equivalent session delta is non-zero: %+v", delta)
	}
}

func TestAnalyzeDiffEmitsExplicitRatesGradesAndBoundedFindings(t *testing.T) {
	beforeEntries := make([]models.CaptureTransaction, 0, 20)
	afterEntries := make([]models.CaptureTransaction, 0, 20)
	for i := 0; i < 20; i++ {
		beforeEntries = append(beforeEntries, diffEntry(
			"before-"+string(rune('a'+i)), "2026-07-30T01:00:00Z",
			"GET", "api", "/stable/1", 200, 40,
		))
		status := 200
		path := "/new/2"
		if i >= 10 {
			status = 500
		}
		afterEntries = append(afterEntries, diffEntry(
			"after-"+string(rune('a'+i)), "2026-07-30T01:00:00Z",
			"GET", "api", path, status, 200,
		))
	}
	before := resultPayload(t, BuildParsed(parser.ParseResult{
		Format: "har", Dialect: "generic", Entries: beforeEntries,
		Redaction: redact.Summary{Known: true}, TimelineAvailable: false,
	}, "before.har", Options{}))
	after := resultPayload(t, BuildParsed(parser.ParseResult{
		Format: "har", Dialect: "generic", Entries: afterEntries,
		Redaction: redact.Summary{Known: true}, TimelineAvailable: false,
	}, "after.har", Options{}))

	result, err := AnalyzeDiff(before, after, DiffOptions{TopN: 1})
	if err != nil {
		t.Fatalf("AnalyzeDiff: %v", err)
	}
	alignment := result.Summary["time_alignment"].(timeAlignment)
	if alignment.Grade != "none" || alignment.OverlayAllowed {
		t.Fatalf("degenerate sessions must suppress overlay: %+v", alignment)
	}
	afterMetrics := result.Summary["after"].(sideMetrics)
	if afterMetrics.ErrorRate.Numerator != 10 || afterMetrics.ErrorRate.Denominator != 20 {
		t.Fatalf("error rate omitted explicit numerator/denominator: %+v", afterMetrics.ErrorRate)
	}
	if afterMetrics.CountPerMinute != nil || afterMetrics.RateUnavailable != "timestamps_degenerate" {
		t.Fatalf("degenerate timestamps must omit per-minute normalization: %+v", afterMetrics)
	}
	if rows := result.Tables["endpoints_added"].([]map[string]any); len(rows) != 1 {
		t.Fatalf("top-N table bound not applied: %+v", rows)
	}
	codes := findingCodes(result)
	for _, code := range []string{
		"HTTP_DIFF_TRAFFIC_SHIFT", "HTTP_DIFF_ERROR_RATE_UP",
		"HTTP_DIFF_LATENCY_REGRESSION", "HTTP_DIFF_NEW_ERROR_ENDPOINT",
		"HTTP_DIFF_ALIGNMENT_LOW",
	} {
		if !codes[code] {
			t.Errorf("missing finding %s: %+v", code, result.Metadata.Findings)
		}
	}
	meta := result.Metadata.Extra["http_capture_diff"].(map[string]any)
	if meta["store_rescanned"] != false || meta["export_projection"] != "analysis_result_envelope" {
		t.Fatalf("store-free export contract missing: %+v", meta)
	}
	process := meta["process_dimension"].(map[string]any)
	if process["available"] != false || !strings.Contains(process["reason"].(string), "HAR pseudo-process") {
		t.Fatalf("HAR process dimension not explicitly disabled: %+v", process)
	}
}

func TestAnalyzeDiffLiveProcessDimensionAndDurationOnlyGrade(t *testing.T) {
	process := &models.ProcessInstance{Key: models.ProcessKey{PID: 42, StartTime: "x"}, Name: "Example.EXE", Attribution: "confirmed"}
	beforeEntries := []models.CaptureTransaction{
		diffEntry("one", "", "GET", "api", "/a", 200, 10),
	}
	beforeEntries[0].Process = process
	afterEntries := []models.CaptureTransaction{
		diffEntry("two", "", "GET", "api", "/a", 200, 20),
		diffEntry("three", "", "GET", "api", "/b", 200, 30),
	}
	for index := range afterEntries {
		afterEntries[index].Process = process
	}
	before := resultPayload(t, BuildLive(beforeEntries, "capture://before", "canonical-v1", redact.Summary{Known: true}, Options{}))
	after := resultPayload(t, BuildLive(afterEntries, "capture://after", "canonical-v1", redact.Summary{Known: true}, Options{}))

	result, err := AnalyzeDiff(before, after, DiffOptions{})
	if err != nil {
		t.Fatalf("AnalyzeDiff: %v", err)
	}
	alignment := result.Summary["time_alignment"].(timeAlignment)
	if alignment.Grade != "duration_only" || !alignment.OverlayAllowed {
		t.Fatalf("relative durations should produce duration_only: %+v", alignment)
	}
	if rows := result.Tables["processes_changed"].([]map[string]any); len(rows) != 1 || rows[0]["key"] != "example.exe" {
		t.Fatalf("live real-process dimension missing: %+v", rows)
	}
}

func TestAnalyzeDiffRejectsMissingOrMismatchedProjectionVersions(t *testing.T) {
	if _, err := AnalyzeDiff(map[string]any{"type": ResultType}, map[string]any{"type": ResultType}, DiffOptions{}); !errors.Is(err, errDiffSourceMissing) {
		t.Fatalf("missing projection error=%v", err)
	}
	result := resultPayload(t, BuildParsed(parser.ParseResult{
		Format: "har", Entries: []models.CaptureTransaction{diffEntry("one", "", "GET", "api", "/", 200, 1)},
		Redaction: redact.Summary{Known: true}, TimelineAvailable: true,
	}, "one.har", Options{}))
	other := clonePayload(t, result)
	metadata := other["metadata"].(map[string]any)
	projection := metadata["http_capture_diff_source"].(map[string]any)
	projection["url_template_version"] = float64(99)
	if _, err := AnalyzeDiff(result, other, DiffOptions{}); err == nil || !strings.Contains(err.Error(), "version mismatch") {
		t.Fatalf("expected explicit template-version error, got %v", err)
	}
}

func diffEntry(id, startedAt, method, host, path string, status int, durationMS float64) models.CaptureTransaction {
	endedAt := ""
	if startedAt != "" {
		endedAt = startedAt
	}
	return models.CaptureTransaction{
		ID: id, StartedAt: startedAt, EndedAt: endedAt,
		Method: method, Host: host, Path: path, StatusCode: status,
		State: models.TxComplete, TotalMS: durationMS,
		Request:  models.HTTPMessage{BodySize: 10, TransferSize: -1},
		Response: models.HTTPMessage{BodySize: 20, TransferSize: 30},
	}
}

func resultPayload(t *testing.T, result models.AnalysisResult) map[string]any {
	t.Helper()
	body, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func clonePayload(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	var cloned map[string]any
	if err := json.Unmarshal(body, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func findingCodes(result models.AnalysisResult) map[string]bool {
	codes := map[string]bool{}
	for _, finding := range result.Metadata.Findings {
		codes[finding["code"].(string)] = true
	}
	return codes
}
