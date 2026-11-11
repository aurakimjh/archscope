package resultcontract

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/wirejson"
)

func semanticFixture(t testing.TB, n int) (*Registry, models.AnalysisResult) {
	t.Helper()
	registry, e := New(Descriptor{Type: "review.r8", Payload: Range{"0.1.0", "0.2.0"}, Envelope: Range{"1.0.0", "2.0.0"}, Providers: []Provider{{ID: "review:timeline", Version: "1.0.0", Kind: "timeline", Path: "/tables/events", Schema: "timeline-v1", Time: "time", Clock: "clock", Evidence: "evidence"}, {ID: "review:sli", Version: "1.0.0", Kind: "sli", Path: "/tables/sli", Schema: "sli-v1", Numerator: "n", Denominator: "d", Unit: "unit", Population: "population"}}})
	if e != nil {
		t.Fatal(e)
	}
	r := models.New("review.r8", "synthetic")
	events, sli := []map[string]any{}, []map[string]any{}
	ids := []string{}
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("source-%d", i)
		ids = append(ids, id)
		events = append(events, map[string]any{"time": "2026-11-12T00:00:00Z", "clock": id, "evidence": id})
		sli = append(sli, map[string]any{"n": json.Number("1"), "d": json.Number("3"), "unit": "ratio", "population": id})
	}
	r.Metadata.Extra["clock_ids"] = ids
	r.Metadata.Extra["evidence_ids"] = ids
	r.Metadata.Extra["population_ids"] = ids
	r.Tables["events"] = events
	r.Tables["sli"] = sli
	return registry, r
}
func TestR801LinearSemanticProvidersAtEightAndSixteenThousandRows(t *testing.T) {
	for _, n := range []int{8000, 16000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			registry, result := semanticFixture(t, n)
			started := time.Now()
			caps, e := registry.Capabilities(context.Background(), result)
			elapsed := time.Since(started)
			if e != nil {
				t.Fatal(e)
			}
			for _, id := range []string{"review:timeline", "review:sli"} {
				found := false
				for _, p := range caps.Providers {
					if p.ID == id {
						found = p.Availability == "available"
					}
				}
				if !found {
					t.Fatal("semantic provider unavailable", id)
				}
			}
			t.Logf("rows=%d timeline+sli Capabilities=%s", n, elapsed)
			// Leave CPU scheduling headroom for parallel hosted race suites. The
			// local acceptance target is <1s; the old 16k probe took ~75s.
			if elapsed > 3*time.Second {
				t.Fatalf("linear semantic validation exceeded CI headroom: %s", elapsed)
			}
			// A fresh validation must still refuse missing references and malformed rows.
			result.Tables["events"].([]map[string]any)[n-1]["evidence"] = "missing"
			result.Tables["sli"].([]map[string]any)[n-1]["population"] = "missing"
			caps, e = registry.Capabilities(context.Background(), result)
			if e != nil {
				t.Fatal(e)
			}
			for _, p := range caps.Providers {
				if (p.Kind == "timeline" || p.Kind == "sli") && p.Availability == "available" {
					t.Fatal("reference validation bypassed")
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if _, e = registry.Capabilities(ctx, result); !errors.Is(e, context.Canceled) {
				t.Fatal(e)
			}
		})
	}
}
func TestR801PreparedSnapshotIsPrivateIndexesAreReusedAndCancellationIsNotCached(t *testing.T) {
	registry, result := semanticFixture(t, 16000)
	body, _ := json.Marshal(result)
	p, e := registry.PrepareBytes(context.Background(), body, wirejson.LegacyResultLimits())
	if e != nil {
		t.Fatal(e)
	}
	first, e := p.Project(context.Background(), "review:timeline", 0, 25)
	if e != nil {
		t.Fatal(e)
	}
	index := p.indexes["/tables/events"]
	// DTO and caller-model mutation cannot affect a validated private snapshot.
	first.Columns[0] = "mutated"
	first.Rows[0][0].Text = "mutated"
	caps := p.Capabilities()
	caps.Providers[0].ID = "mutated"
	result.Tables["events"].([]map[string]any)[0]["clock"] = "mutated"
	for i := range body {
		body[i] = 'x'
	}
	last, e := p.Project(context.Background(), "review:timeline", 15975, 25)
	if e != nil || last.Total != 16000 || last.Next != -1 || last.Columns[0] == "mutated" || p.indexes["/tables/events"] != index {
		t.Fatal(last, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = p.Project(ctx, "review:sli", 0, 25); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if p.indexes["/tables/sli"] != nil {
		t.Fatal("canceled index cached")
	}
	// A queued lazy index remains cancelable even while another query owns its gate.
	p.gate <- struct{}{}
	done := make(chan error, 1)
	queued, cancelQueued := context.WithCancel(context.Background())
	go func() { _, e := p.Project(queued, "review:sli", 0, 25); done <- e }()
	cancelQueued()
	select {
	case e := <-done:
		if !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	case <-time.After(time.Second):
		t.Fatal("queued projection ignored cancellation")
	}
	<-p.gate
	if _, e = p.Project(context.Background(), "review:sli", 0, 25); e != nil {
		t.Fatal(e)
	}
}
func TestR803DisplayEscapesControlsAndPreservesOriginalData(t *testing.T) {
	registry, _ := New()
	r := models.New("unknown", "synthetic")
	raw := "abc\u202edef\x07\u0085\u061c\u2066\n\t"
	r.Summary[raw] = raw
	r.SourceFiles = []string{raw}
	r.Tables["rows"] = []map[string]any{{raw: raw}}
	body, _ := json.Marshal(r)
	p, e := registry.PrepareBytes(context.Background(), body, wirejson.LegacyResultLimits())
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []string{"generic:summary", "generic:/tables/rows"} {
		page, e := p.Project(context.Background(), id, 0, 25)
		if e != nil || !page.Escaped {
			t.Fatal(page, e)
		}
		for _, row := range page.Rows {
			for _, cell := range row {
				if !cell.Escaped || !strings.Contains(cell.Text, "⟦U+202E⟧") {
					t.Fatal(cell)
				}
				for _, c := range cell.Text {
					if unicode.IsControl(c) || c == 0x202e {
						t.Fatal("invisible display", cell)
					}
				}
			}
		}
	}
	sources, _ := p.Sources()
	if !sources[0].Escaped {
		t.Fatal(sources)
	}
	after, _ := json.Marshal(r)
	if string(after) != string(body) {
		t.Fatal("original changed")
	}
	long, truncated, escaped := DisplayText(strings.Repeat("\u202e", 2048))
	if !truncated || !escaped || len(long) > MaxCellBytes || strings.ContainsRune(long, 0x202e) {
		t.Fatal("escape expansion not bounded")
	}
}
func TestR804UnsupportedComparisonIsTyped(t *testing.T) {
	registry, _ := New(Descriptor{Type: "known", Payload: Range{"0.1.0", "0.2.0"}, Envelope: Range{"1.0.0", "2.0.0"}})
	base := models.New("known", "synthetic")
	target := models.New("unknown", "synthetic")
	for _, pair := range [][2]models.AnalysisResult{{base, target}, {target, base}} {
		delta, e := registry.Compare(pair[0], pair[1], "/summary/count")
		var refusal *ComparisonError
		if !errors.As(e, &refusal) || refusal.Code != ComparisonResultUnsupported || refusal.Message != "unknown_result_type" || delta.Value != nil || delta.Percent != nil {
			t.Fatal(delta, e)
		}
	}
}

type cancelOnChecks struct {
	context.Context
	cancel    context.CancelFunc
	remaining int
}

func (c *cancelOnChecks) Err() error {
	c.remaining--
	if c.remaining == 0 {
		c.cancel()
	}
	return c.Context.Err()
}
func TestR801MidValidationAndMidIndexCancellationLeaveNoPartialIndex(t *testing.T) {
	registry, result := semanticFixture(t, 8000)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	probe := &cancelOnChecks{parent, cancel, 3*8000 + 100}
	if _, e := registry.Capabilities(probe, result); !errors.Is(e, context.Canceled) {
		t.Fatal("mid-row validation ignored cancellation", e)
	}
	body, _ := json.Marshal(result)
	p, e := registry.PrepareBytes(context.Background(), body, wirejson.LegacyResultLimits())
	if e != nil {
		t.Fatal(e)
	}
	parent, cancel = context.WithCancel(context.Background())
	defer cancel()
	probe = &cancelOnChecks{parent, cancel, 100}
	if _, e = p.Project(probe, "review:timeline", 0, 25); !errors.Is(e, context.Canceled) {
		t.Fatal("mid-index cancellation ignored", e)
	}
	if p.indexes["/tables/events"] != nil {
		t.Fatal("partial collection index cached")
	}
	if _, e = p.Project(context.Background(), "review:timeline", 0, 25); e != nil {
		t.Fatal("retry after canceled index failed", e)
	}
	result.Metadata.Extra["evidence_ids"] = []any{"source-0", 17}
	caps, e := registry.Capabilities(context.Background(), result)
	if e != nil {
		t.Fatal(e)
	}
	for _, provider := range caps.Providers {
		if provider.Kind == "timeline" && provider.Availability == "available" {
			t.Fatal("malformed reference list partially accepted")
		}
	}
}
