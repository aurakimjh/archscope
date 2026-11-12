package gputrace_test

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/gputrace"
	e "github.com/aurakimjh/archscope/apps/engine-native/internal/exporters/gputrace"
	g "github.com/aurakimjh/archscope/apps/engine-native/internal/gputracemodel"
	"strings"
	"testing"
)

var ctx = context.Background()

func model() g.Set {
	s := g.Empty()
	src := g.Source{SHA: strings.Repeat("a", 64), Format: "fixture", Producer: "fixture", Version: "1", Schema: "fixture", Capture: "capture", Host: "host", Boot: "boot", Namespace: "packed", Run: "run", Clock: "source-clock", DeviceMap: map[string]string{"0": "gpu"}}
	src.ID = g.SourceID(src)
	s.Sources = append(s.Sources, src)
	return s
}
func add(s *g.Set, kind, thread, stream, from, to, corr string) g.Event {
	track := g.Track{Source: s.Sources[0].ID, Kind: kind, Process: "1", Thread: thread}
	if kind == "gpu_kernel" {
		track.Device = "gpu"
		track.DeviceIndex = "0"
		track.Context = "context"
		track.Stream = stream
	}
	track.ID = g.TrackID(track)
	known := false
	for _, t := range s.Tracks {
		known = known || t.ID == track.ID
	}
	if !known {
		s.Tracks = append(s.Tracks, track)
	}
	v := g.Event{Track: track.ID, Kind: kind, Name: "<script>private&name</script>", From: from, To: to, Correlation: corr, Position: fmt.Sprint(len(s.Events))}
	v.ID = g.EventID(v)
	s.Events = append(s.Events, v)
	return v
}
func TestExactCPUChildUnionGPUOverlapAndScopedCorrelations(t *testing.T) {
	s := model()
	outer := add(&s, "cpu", "thread", "", "0", "100", "")
	child := add(&s, "cpu", "thread", "", "10", "30", "")
	add(&s, "cpu", "thread", "", "15", "20", "")
	add(&s, "cpu", "thread", "", "30", "40", "")
	api := add(&s, "cuda_api", "thread", "", "5", "8", "7")
	add(&s, "gpu_kernel", "", "1", "10", "30", "7")
	add(&s, "gpu_kernel", "", "2", "20", "40", "7")
	if err := s.Validate(ctx); err != nil {
		t.Fatal(err)
	}
	p, err := a.Project(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Busy) != 1 || p.Busy[0].KernelSum != "40" || p.Busy[0].Busy != "30" || p.Busy[0].Extent != "30" {
		t.Fatal(p.Busy)
	}
	for _, v := range p.Events {
		if v.ID == outer.ID && (v.Self == nil || *v.Self != "70") {
			t.Fatal(v)
		}
		if v.ID == child.ID && (v.Self == nil || *v.Self != "15" || v.Parent != outer.ID) {
			t.Fatal(v)
		}
	}
	if len(p.Relations) != 2 {
		t.Fatal(p.Relations)
	}
	for _, r := range p.Relations {
		if r.From != api.ID || r.Status != "explicit_correlation" {
			t.Fatal(r)
		}
	}
	add(&s, "cuda_api", "other-thread", "", "6", "9", "7")
	p, err = a.Project(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range p.Relations {
		if r.Status != "ambiguous_launch" || r.From != "" {
			t.Fatal("ambiguous launch fabricated", r)
		}
	}
}
func TestCrossingCPUSelfUnavailableAndExactUint64SVG(t *testing.T) {
	s := model()
	add(&s, "cpu", "t", "", "9007199254740993", "9007199254741003", "")
	add(&s, "cpu", "t", "", "9007199254740998", "9007199254741008", "")
	p, err := a.Project(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range p.Events {
		if v.Self != nil || v.SelfStatus != "ambiguous_crossing_or_equal_intervals" {
			t.Fatal(v)
		}
	}
	svg, err := e.SVG(ctx, s, p, strings.Repeat("a", 64), a.Query{Kind: "events", Source: s.Sources[0].ID})
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	_ = doc
	d := xml.NewDecoder(strings.NewReader(string(svg)))
	for {
		_, err = d.Token()
		if err != nil {
			if err.Error() != "EOF" {
				t.Fatal(err)
			}
			break
		}
	}
	if strings.Contains(string(svg), "<script>") || !strings.Contains(string(svg), "9007199254740993") {
		t.Fatal("escaped/exact SVG")
	}
	if _, err = e.SVG(ctx, s, p, strings.Repeat("a", 64), a.Query{Kind: "events"}); err == nil {
		t.Fatal("independent clocks aligned")
	}
}
func TestQueriesCursorIntegritySavedProjectionAndScale(t *testing.T) {
	s := model()
	for i := 0; i < 20000; i++ {
		add(&s, "cpu", "t", "", fmt.Sprint(i*2), fmt.Sprint(i*2+1), "")
	}
	if err := s.Validate(ctx); err != nil {
		t.Fatal(err)
	}
	p, err := a.Project(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	sha := strings.Repeat("b", 64)
	q := a.Query{Kind: "events", Source: s.Sources[0].ID, Limit: 25}
	page, err := a.QuerySet(ctx, s, p, sha, q)
	if err != nil || page.Total != 20000 || len(page.Rows) != 25 || page.Next == "" {
		t.Fatal(page.Total, err)
	}
	q.Cursor = page.Next
	q.Kind = "tracks"
	if _, err = a.QuerySet(ctx, s, p, sha, q); err == nil {
		t.Fatal("cursor selector crossed")
	}
	q.Kind = "events"
	if _, err = a.QuerySet(ctx, s, p, strings.Repeat("c", 64), q); err == nil {
		t.Fatal("cursor revision crossed")
	}
	if _, err = e.SVG(ctx, s, p, sha, a.Query{Kind: "events", Source: s.Sources[0].ID}); err == nil {
		t.Fatal("SVG silently truncated")
	}
	result, err := a.Build(ctx, s, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Validate(result); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(result)
	t.Logf("20k model/result bytes=%d", len(raw))
	if len(raw) > 32<<20 {
		t.Fatal("unexpected scale inflation")
	}
	result.Summary["event_count"] = "1"
	if a.Validate(result) == nil {
		t.Fatal("forged projection")
	}
	s.Events = append(s.Events, make([]g.Event, g.MaxEvents)...)
	if s.Validate(ctx) == nil {
		t.Fatal("record limit")
	}
}
func TestRankClockIsolationAndFlowAmbiguity(t *testing.T) {
	one := model()
	ev := add(&one, "cpu", "t", "", "1", "2", "")
	two := model()
	rank := "1"
	world := "2"
	two.Sources[0].Rank = &rank
	two.Sources[0].WorldSize = &world
	two.Sources[0].Backend = "gloo"
	two.Sources[0].Clock = "other-clock"
	two.Sources[0].ID = g.SourceID(two.Sources[0])
	add(&two, "cpu", "t", "", "1", "2", "")
	merged, err := g.Merge(ctx, []g.Set{one, two})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := a.Project(ctx, merged)
	page, err := a.QuerySet(ctx, merged, p, strings.Repeat("a", 64), a.Query{Kind: "events", Rank: &rank, Limit: 25})
	if err != nil || len(page.Rows) != 1 || strings.Contains(string(page.Rows[0]), ev.ID) {
		t.Fatal(page, err)
	}
	lo, hi := "0", "10"
	if _, err = a.QuerySet(ctx, merged, p, strings.Repeat("a", 64), a.Query{Kind: "events", From: &lo, To: &hi, Limit: 25}); err == nil {
		t.Fatal("cross-clock time filter")
	}
	if _, err = g.Merge(ctx, []g.Set{one, one}); err == nil {
		t.Fatal("duplicate source implicitly merged")
	}
	s := model()
	for i, phase := range []string{"s", "f", "f"} {
		v := add(&s, "flow", fmt.Sprint(i), "", "1", "1", "")
		s.Events[len(s.Events)-1].Flow = "source-flow"
		s.Events[len(s.Events)-1].Phase = phase
		s.Events[len(s.Events)-1].ID = g.EventID(s.Events[len(s.Events)-1])
		_ = v
	}
	if err = s.Validate(ctx); err != nil {
		t.Fatal(err)
	}
	projection, _ := a.Project(ctx, s)
	if len(projection.Relations) != 1 || projection.Relations[0].Status != "orphan_or_ambiguous_flow" {
		t.Fatal(projection.Relations)
	}
}

func TestIntegerBudgetsBeforeArithmetic(t *testing.T) {
	if g.Uint(strings.Repeat("9", 1<<20)) {
		t.Fatal("oversized uint accepted")
	}
	s := model()
	offset := strings.Repeat("9", 1<<20)
	uncertainty := "0"
	s.Sources[0].ClockGroup = "group"
	s.Sources[0].Offset = &offset
	s.Sources[0].Uncertainty = &uncertainty
	s.Sources[0].ID = g.SourceID(s.Sources[0])
	if s.Validate(ctx) == nil {
		t.Fatal("oversized offset")
	}
}
