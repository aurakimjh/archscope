package lighthouse

import (
	"testing"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/lighthouse"
)

func TestBuildEmitsBoundedBrowserAuditEvidence(t *testing.T) {
	poor := 0.4
	good := 0.95
	lcp := 3200.0
	report := parser.Report{
		LighthouseVersion: "13.0.1", RequestedURL: "https://example.test/", FinalURL: "https://example.test/",
		Categories: []parser.Category{{ID: "performance", Title: "Performance", Score: &poor}},
		Audits: []parser.Audit{
			{ID: "largest-contentful-paint", Title: "LCP", Score: &poor, NumericValue: &lcp, NumericUnit: "millisecond"},
			{ID: "uses-text-compression", Title: "Compression", Score: &good},
		},
		Resources: []parser.Resource{
			{URL: "https://example.test/large.js", ResourceType: "Script", TransferBytes: 9000},
			{URL: "https://example.test/small.css", ResourceType: "Stylesheet", TransferBytes: 1000},
		},
	}
	result := Build(report, "report.json", Options{TopN: 1})
	if result.Type != ResultType || result.Metadata.Parser != ParserName {
		t.Fatalf("unexpected envelope: %+v", result)
	}
	if result.Summary["performance_score_pct"] != 40.0 || result.Summary["largest_contentful_paint_ms"] != 3200.0 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	rows := result.Tables["network_requests"].([]map[string]any)
	if len(rows) != 1 || rows[0]["transfer_size_bytes"] != int64(9000) {
		t.Fatalf("unexpected bounded rows: %+v", rows)
	}
	if len(result.Metadata.Findings) != 2 {
		t.Fatalf("unexpected findings: %+v", result.Metadata.Findings)
	}
}

func TestBuildUsesReportScoresWithoutRecomputingThresholds(t *testing.T) {
	good := 1.0
	metric := 99999.0
	report := parser.Report{
		Categories: []parser.Category{{ID: "performance", Score: &good}},
		Audits:     []parser.Audit{{ID: "largest-contentful-paint", Score: &good, NumericValue: &metric}},
	}
	result := Build(report, "report.json", Options{})
	if len(result.Metadata.Findings) != 0 {
		t.Fatalf("report score should remain authoritative: %+v", result.Metadata.Findings)
	}
}
