package runtimeevidence_test

import (
	"context"
	"encoding/json"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/profile"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/runtimeevidence"
	r "github.com/aurakimjh/archscope/apps/engine-native/internal/runtimemodel"
	"os"
	"strings"
	"testing"
)

func model(t *testing.T) r.Set {
	t.Helper()
	raw, e := os.ReadFile("../../catalog/contracts/runtime-evidence-set.json")
	if e != nil {
		t.Fatal(e)
	}
	s, e := r.Decode(context.Background(), raw)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func ptr(v string) *string { return &v }
func TestCPULifetimeClockAndNativeEncoding(t *testing.T) {
	s := model(t)
	var cpu, stack r.Observation
	for _, v := range s.Observations {
		if v.Kind == "cpu" {
			cpu = v
		}
		if v.Kind == "thread_snapshot" {
			stack = v
		}
	}
	if out := r.Correlate(s, cpu, stack); out.Status != "qualified_context" || out.NativeID != "500" {
		t.Fatal(out)
	}
	for _, c := range []struct {
		name   string
		change func(*r.Set, *r.Observation, *r.Observation)
		want   string
	}{{"future-process", func(s *r.Set, c, b *r.Observation) {
		for i := range s.Processes {
			s.Processes[i].Start = ptr("4000000000")
		}
	}, "incompatible"}, {"lifetime-average", func(s *r.Set, c, b *r.Observation) { c.Basis = "lifetime_average" }, "candidate"}, {"outside-window", func(s *r.Set, c, b *r.Observation) { b.Time = ptr("4000000000") }, "incompatible"}, {"unknown-clock", func(s *r.Set, c, b *r.Observation) {
		for i := range s.Sources {
			if s.Sources[i].ID == c.Ref.Source {
				s.Sources[i].Clock = "unknown"
			}
		}
	}, "candidate"}, {"host-missing", func(s *r.Set, c, b *r.Observation) {
		for i := range s.Processes {
			if s.Processes[i].ID == "jvm-process" {
				s.Processes[i].Host = ""
			}
		}
		for i := range s.Processes {
			s.Processes[i].Host = ""
		}
	}, "candidate"}, {"tid-reuse", func(s *r.Set, c, b *r.Observation) {
		for i := range s.Entities {
			if s.Entities[i].ID == b.Entity {
				s.Entities[i].Start = ptr("1")
			}
		}
	}, "incompatible"}, {"thread-lifetime-missing", func(s *r.Set, c, b *r.Observation) {
		for i := range s.Entities {
			if s.Entities[i].ID == b.Entity {
				s.Entities[i].Start = nil
			}
		}
	}, "candidate"}, {"native-mismatch", func(s *r.Set, c, b *r.Observation) {
		for i := range s.Entities {
			if s.Entities[i].ID == b.Entity {
				s.Entities[i].NativeID = "0x1f5"
			}
		}
	}, "incompatible"}, {"task-not-thread", func(s *r.Set, c, b *r.Observation) { b.Kind = "task_snapshot" }, "candidate"}} {
		t.Run(c.name, func(t *testing.T) {
			raw, _ := json.Marshal(s)
			var changed r.Set
			json.Unmarshal(raw, &changed)
			x, y := cpu, stack
			c.change(&changed, &x, &y)
			if out := r.Correlate(changed, x, y); out.Status != c.want {
				t.Fatal(out)
			}
		})
	}
}
func TestPauseUnionConcurrentAndUnknownClock(t *testing.T) {
	s := model(t)
	base := r.Observation{}
	for _, v := range s.Observations {
		if v.Kind == "gc" {
			base = v
			break
		}
	}
	if base.Entity == "" {
		t.Fatal("fixture gc missing")
	}
	s.Observations = nil
	for i, v := range [][2]string{{"10", "20"}, {"15", "30"}, {"30", "40"}, {"12", "50"}} {
		x := base
		x.From = ptr(v[0])
		x.To = ptr(v[1])
		x.Metric = "pause_ns"
		if i == 3 {
			x.Metric = "concurrent_ns"
		}
		s.Observations = append(s.Observations, x)
	}
	rows, e := a.GCCoverages(context.Background(), s)
	if e != nil || len(rows) != 1 || *rows[0].Union != "30" || *rows[0].Concurrent != "38" {
		t.Fatal(rows, e)
	}
	for i := range s.Sources {
		s.Sources[i].Clock = "unknown"
	}
	rows, e = a.GCCoverages(context.Background(), s)
	if e != nil || rows[0].Union != nil || rows[0].Concurrent != nil {
		t.Fatal(rows, e)
	}
}
func TestSavedProjectionAllocationAndPaging(t *testing.T) {
	ctx := context.Background()
	s := model(t)
	result, e := a.Build(ctx, s, "sanitized")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Validate(result); e != nil {
		t.Fatal(e)
	}
	result.Summary["entity_count"] = "999"
	if a.Validate(result) == nil {
		t.Fatal("forged summary")
	}
	digest := strings.Repeat("a", 64)
	page, e := a.QuerySet(ctx, s, digest, a.Query{Kind: "observations", Limit: 1})
	if e != nil || len(page.Rows) != 1 || page.Next == "" {
		t.Fatal(page, e)
	}
	second, e := a.QuerySet(ctx, s, digest, a.Query{Kind: "observations", Limit: 1, Cursor: page.Next})
	if e != nil || string(second.Rows[0]) == string(page.Rows[0]) {
		t.Fatal(second, e)
	}
	if _, e = a.QuerySet(ctx, s, strings.Repeat("b", 64), a.Query{Kind: "observations", Limit: 1, Cursor: page.Next}); e == nil {
		t.Fatal("stale cursor")
	}
	if _, e = a.QuerySet(ctx, s, digest, a.Query{Kind: "cpu_link", Limit: 1}); e == nil {
		t.Fatal("crossproduct allowed")
	}
	var alloc r.Observation
	for _, v := range s.Observations {
		if v.Kind == "allocation" && v.Metric == "live_bytes" {
			alloc = v
			break
		}
	}
	if alloc.ID == "" {
		t.Fatal("allocation missing")
	}
	for _, metric := range []string{"live_bytes", "live_blocks"} {
		set, selection, e := a.AllocationProfile(ctx, s, alloc.Entity, alloc.Ref.Source, metric)
		if e != nil {
			t.Fatal(e)
		}
		if set.Metrics[0].Kind == "sample_count" {
			t.Fatal("allocation converted to CPU samples")
		}
		if _, e = p.BuildSet(ctx, set, selection, "snapshot"); e != nil {
			t.Fatal(e)
		}
	}
	x := alloc
	x.Time = ptr("999999999999999999")
	s.Observations = append(s.Observations, x)
	if _, _, e = a.AllocationProfile(ctx, s, alloc.Entity, alloc.Ref.Source, "live_bytes"); e == nil {
		t.Fatal("cross-capture sum")
	}
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = a.QuerySet(ctx, s, digest, a.Query{Kind: "frames", Limit: 25}); e == nil {
		t.Fatal("cancel ignored")
	}
}
func TestScaleBudgetsAndMergeConflicts(t *testing.T) {
	s := model(t)
	base := s.Observations[0]
	s.Observations = nil
	for i := 0; i < 20000; i++ {
		v := base
		v.Ref.Position = string(rune('a'+i)) + "synthetic"
		v.ID = r.ObservationID(v)
		s.Observations = append(s.Observations, v)
	}
	if _, e := a.Build(context.Background(), s, "20k"); e != nil {
		t.Fatal(e)
	}
	if _, e := r.Merge(context.Background(), []r.Set{s, s}); e != nil {
		t.Fatal(e)
	}
	s.Observations = append(s.Observations, make([]r.Observation, r.MaxObservations)...)
	if s.Validate(context.Background()) == nil {
		t.Fatal("budget not enforced")
	}
	s = model(t)
	other := model(t)
	other.Processes[0].Runtime = "conflicting"
	if _, e := r.Merge(context.Background(), []r.Set{s, other}); e == nil {
		t.Fatal("conflict winner chosen")
	}
}

func TestPlanCyclesAndTaskCPUAreRefused(t *testing.T) {
	s := model(t)
	s.Plans[0].Parent = s.Plans[0].ID
	if e := s.Validate(context.Background()); e == nil {
		t.Fatal("plan cycle")
	}
	s = model(t)
	for i, v := range s.Observations {
		if v.Kind == "cpu" {
			for _, entity := range s.Entities {
				if entity.Kind == "async_task" {
					s.Observations[i].Entity = entity.ID
					s.Observations[i].ID = r.ObservationID(s.Observations[i])
					break
				}
			}
			break
		}
	}
	if e := s.Validate(context.Background()); e == nil {
		t.Fatal("task CPU attributed")
	}
}
