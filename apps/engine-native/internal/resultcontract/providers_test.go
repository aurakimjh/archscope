package resultcontract

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func TestProvidersFallbackExactBoundsAndUntrustedClaims(t *testing.T) {
	registry, _ := New()
	r := models.New("unknown", "synthetic")
	r.Summary = map[string]any{"exact": json.Number("9007199254740993"), "zero": 0, "null": nil, "text": strings.Repeat("한", 3000)}
	r.Metadata.Extra["capabilities"] = []string{"timeline", "compare"}
	r.Tables["a/b~c"] = []any{map[string]any{"number": json.Number("9007199254740993"), "value": nil}, map[string]any{"number": json.Number("0")}}
	r.Charts["unsafe"] = "<script>fetch('https://invalid')</script>"
	caps, e := registry.Capabilities(context.Background(), r)
	if e != nil || caps.Compatibility.Allowed {
		t.Fatal(caps, e)
	}
	for _, p := range caps.Providers {
		if p.Kind == "compare" || p.Kind == "timeline" {
			t.Fatal("imported string granted capability")
		}
	}
	page, e := registry.Project(context.Background(), r, "generic:summary", 0, 100)
	if e != nil {
		t.Fatal(e)
	}
	found := map[string]Cell{}
	for _, row := range page.Rows {
		found[row[0].Text] = row[1]
	}
	if found["exact"].Text != "9007199254740993" || !found["null"].Null || found["zero"].Text != "0" || !found["text"].Truncated || len(found["text"].Text) > MaxCellBytes {
		t.Fatal(found)
	}
	table, e := registry.Project(context.Background(), r, "generic:/tables/a~1b~0c", 0, 1)
	if e != nil || table.Next != 1 || table.Rows[0][0].Text != "9007199254740993" {
		t.Fatal(table, e)
	}
	table, e = registry.Project(context.Background(), r, "generic:/tables/a~1b~0c", 1, 1)
	if e != nil || table.Next != -1 || table.Rows[0][1].Availability != "absent" {
		t.Fatal(table, e)
	}
	if _, e = registry.Project(context.Background(), r, "generic:charts", 0, 1); e == nil {
		t.Fatal("unknown chart executed")
	}
	for _, bounds := range [][2]int{{-1, 1}, {0, 0}, {0, 101}, {99, 1}} {
		if _, e = registry.Project(context.Background(), r, "generic:summary", bounds[0], bounds[1]); e == nil {
			t.Fatal("invalid bounds", bounds)
		}
	}
	// Aggregate byte budget and column/cell limits hold simultaneously.
	rows := []map[string]any{}
	for i := 0; i < 100; i++ {
		row := map[string]any{}
		for j := 0; j < 40; j++ {
			row[string(rune('a'+j))] = strings.Repeat("x", 3000)
		}
		rows = append(rows, row)
	}
	r.Tables["wide"] = rows
	page, e = registry.Project(context.Background(), r, "generic:/tables/wide", 0, 100)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := json.Marshal(page)
	if len(body) > MaxPageBytes || len(page.Columns) > MaxPageColumns || !page.Truncated || page.Next < 1 {
		t.Fatal(len(body), page.Next)
	}
	// Unsupported known revisions retain the same read-only fallback.
	r.Type = "known"
	registry, _ = New(Descriptor{Type: "known", Payload: Range{"1.0.0", "2.0.0"}, Envelope: Range{"1.0.0", "2.0.0"}})
	if caps, e = registry.Capabilities(context.Background(), r); e != nil || caps.Compatibility.Reason != "payload_requires_original_reanalysis" {
		t.Fatal(caps, e)
	}
}
func TestProvidersSemanticPayloadValidationAndCancellation(t *testing.T) {
	timeline := Provider{ID: "test:timeline", Version: "1.0.0", Kind: "timeline", Path: "/tables/events", Schema: "timeline-v1", Time: "time", Clock: "clock", Evidence: "evidence"}
	sli := Provider{ID: "test:sli", Version: "1.0.0", Kind: "sli", Path: "/tables/sli", Schema: "sli-v1", Numerator: "n", Denominator: "d", Unit: "unit", Population: "population"}
	descriptor := Descriptor{Type: "synthetic", Payload: Range{"0.1.0", "0.2.0"}, Envelope: Range{"1.0.0", "2.0.0"}, Providers: []Provider{timeline, sli}}
	registry, e := New(descriptor)
	if e != nil {
		t.Fatal(e)
	}
	// Registry freezes declarations, including paths/column roles.
	descriptor.Providers[0].Time = "wrong"
	r := models.New("synthetic", "fixture")
	r.Metadata.Extra["clock_ids"] = []string{"clock-1"}
	r.Metadata.Extra["evidence_ids"] = []string{"source-1"}
	r.Metadata.Extra["population_ids"] = []string{"requests"}
	r.Tables["events"] = []map[string]any{{"time": "2026-11-11T00:00:00Z", "clock": "clock-1", "evidence": "source-1"}}
	r.Tables["sli"] = []map[string]any{{"n": 1, "d": 3, "unit": "ratio", "population": "requests"}}
	available := func(id string) bool {
		t.Helper()
		caps, e := registry.Capabilities(context.Background(), r)
		if e != nil {
			t.Fatal(e)
		}
		for _, c := range caps.Providers {
			if c.ID == id {
				return c.Availability == "available"
			}
		}
		t.Fatal("missing provider")
		return false
	}
	if !available(timeline.ID) || !available(sli.ID) {
		t.Fatal("valid semantic provider refused")
	}
	r.Tables["events"].([]map[string]any)[0]["clock"] = "forged"
	r.Tables["sli"].([]map[string]any)[0]["d"] = 0
	if available(timeline.ID) || available(sli.ID) {
		t.Fatal("malformed semantic payload accepted")
	}
	if _, e = registry.Project(context.Background(), r, timeline.ID, 0, 10); e == nil {
		t.Fatal("invalid provider queried")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = registry.Capabilities(ctx, r); e != context.Canceled {
		t.Fatal(e)
	}
	for _, bad := range []Provider{{ID: "bad", Version: "1.0.0", Kind: "timeline", Path: "/tables/events", Schema: "timeline-v1"}, {ID: "generic:forged", Version: "1.0.0", Kind: "detail", Path: "/tables/events", Schema: "object-rows-v1"}} {
		descriptor.Providers = []Provider{bad}
		if _, e = New(descriptor); e == nil {
			t.Fatal("invalid registration")
		}
	}
}

func TestProviderOversizedFirstRowRefusesWithoutPagingLoop(t *testing.T) {
	registry, _ := New()
	r := models.New("unknown", "synthetic")
	row := map[string]any{}
	for i := 0; i < 32; i++ {
		row[string(rune('a'+i))] = strings.Repeat("\x01", MaxCellBytes)
	}
	r.Tables["escaped"] = []map[string]any{row}
	if _, e := registry.Project(context.Background(), r, "generic:/tables/escaped", 0, 25); e == nil {
		t.Fatal("oversized row must refuse rather than return nonadvancing next offset")
	}
}
