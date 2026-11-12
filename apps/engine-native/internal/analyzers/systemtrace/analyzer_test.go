package systemtrace_test

import (
	"context"
	"encoding/json"
	"fmt"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/systemtrace"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/profilemodel"
	"github.com/aurakimjh/archscope/apps/engine-native/internal/resultcontract"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/systemmodel"
	"math/big"
	"os"
	"strings"
	"testing"
)

func model(t *testing.T, name string) m.Set {
	t.Helper()
	raw, e := os.ReadFile("../../parsers/systemtrace/testdata/" + name)
	if e != nil {
		t.Fatal(e)
	}
	s, e := m.Decode(context.Background(), raw)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func TestSchedulerIntervalsSeparateRunnableSleepingAndIncomplete(t *testing.T) {
	ctx := context.Background()
	s := model(t, "system-evidence-set.json")
	rows, e := a.Intervals(ctx, s)
	if e != nil {
		t.Fatal(e)
	}
	totals := map[string][]string{}
	incomplete := 0
	for _, r := range rows {
		if r.Duration != nil {
			totals[r.Kind] = append(totals[r.Kind], *r.Duration)
		} else {
			incomplete++
		}
		if r.Start != nil && r.End != nil && r.Duration != nil && m.Sub(*r.End, *r.Start) != *r.Duration {
			t.Fatal("span arithmetic")
		}
	}
	for kind, want := range map[string]string{"off_cpu": "300,50", "on_cpu": "100,100,50", "runqueue": "150,50", "block_io": "100", "syscall": "0", "network": "100"} {
		if strings.Join(totals[kind], ",") != want {
			t.Fatal(kind, totals[kind], want)
		}
	}
	if incomplete == 0 {
		t.Fatal("open scheduler boundary lost")
	}
	for _, loss := range []*string{nil, m.Ptr("3")} {
		copy := s
		copy.Capture.Lost = loss
		rows, _ := a.Intervals(ctx, copy)
		for _, r := range rows {
			if r.Duration != nil && r.Kind != "block_io" && r.Kind != "syscall" && r.Kind != "network" {
				t.Fatal("lossy sequence derived", r)
			}
		}
	}
	copy := s
	copy.Capture.Clock = "unknown"
	rows, _ = a.Intervals(ctx, copy)
	for _, r := range rows {
		if r.Duration != nil {
			t.Fatal("unknown clock derived")
		}
	}
	copy = s
	copy.Records = append(append([]m.Record(nil), s.Records...), s.Records[2])
	copy.Records[len(copy.Records)-1].Line = 100
	copy.Records[len(copy.Records)-1].ID = m.RecordID(copy.SourceSHA256, copy.Records[len(copy.Records)-1])
	rows, _ = a.Intervals(ctx, copy)
	ambiguous := 0
	for _, r := range rows {
		if r.Status == "ambiguous_timestamp" {
			ambiguous++
		}
		if r.Start != nil && *r.Start == "200" && r.Duration != nil {
			t.Fatal("duplicate timestamp arbitrarily paired")
		}
	}
	if ambiguous != 2 {
		t.Fatal(ambiguous)
	}
}
func TestSampleAndPeriodProfilesNeverShareUnits(t *testing.T) {
	ctx := context.Background()
	s := model(t, "perf-set.json")
	for metric, want := range map[string]string{"samples/cpu-clock:HG": "3", "period/cpu-clock:HG": "6000000"} {
		projection, e := a.Project(ctx, s, metric, nil)
		if e != nil {
			t.Fatal(e)
		}
		ids := []string{}
		for _, v := range projection.Profiles {
			ids = append(ids, v.ID)
			for _, sample := range v.Samples {
				if sample.Interval != nil {
					t.Fatal("sampling period invented sample interval")
				}
			}
		}
		tree, e := p.BuildTree(ctx, projection, p.Selection{ProfileIDs: ids, MetricID: metric})
		if e != nil || tree.Root.Inclusive != want {
			t.Fatal(tree, e)
		}
		if len(projection.Frames) != 4 {
			t.Fatal("different unknown addresses merged", len(projection.Frames))
		}
	}
	s = model(t, "system-evidence-set.json")
	projection, e := a.Project(ctx, s, "period/cycles:u", nil)
	if e != nil {
		t.Fatal(e)
	}
	if projection.Profiles[0].Samples[0].Weight != "9007199254740993" || projection.Metrics[0].Unit != "cycles" {
		t.Fatal("PMU period converted to time")
	}
	projection, e = a.Project(ctx, s, "off_cpu", nil)
	if e != nil || projection.Metrics[0].Kind != "duration" {
		t.Fatal(e)
	}
	s.Capture.Lost = nil
	if _, e = a.Project(ctx, s, "off_cpu", nil); e == nil {
		t.Fatal("unknown loss off-CPU qualified")
	}
	hist := model(t, "histogram-set.json")
	if _, e = a.Project(ctx, hist, "histogram", nil); e == nil {
		t.Fatal("histogram became timeline")
	}
}
func TestResultValidationRefusesTamperedValuesAndDecoratesAvailability(t *testing.T) {
	ctx := context.Background()
	s := model(t, "system-evidence-set.json")
	r, e := a.Build(ctx, s, "synthetic")
	if e != nil {
		t.Fatal(e)
	}
	registry, e := resultcontract.New(a.Descriptor())
	if e != nil {
		t.Fatal(e)
	}
	if verdict := registry.Check(r, "compute"); !verdict.Allowed {
		t.Fatal(verdict)
	}
	if e = a.Validate(r); e != nil {
		t.Fatal(e)
	}
	r.Summary["sample_records"] = "2"
	if registry.Check(r, "compute").Allowed || a.Validate(r) == nil {
		t.Fatal("tampered summary accepted")
	}
	r, _ = a.Build(ctx, s, "synthetic")
	raw, _ := json.Marshal(r)
	var stored map[string]any
	json.Unmarshal(raw, &stored)
	tables := stored["tables"].(map[string]any)
	tables["intervals"].([]any)[0].(map[string]any)["duration_ns"] = "999"
	raw, _ = json.Marshal(stored)
	changed, e := resultcontract.Decode(raw)
	if e != nil {
		t.Fatal(e)
	}
	if a.Validate(changed) == nil {
		t.Fatal("tampered interval accepted")
	}
}

func TestHistogramPagesKeepFullScaleAndNoTemporalProjection(t *testing.T) {
	s := model(t, "histogram-set.json")
	if len(a.Projections(s)) != 0 {
		t.Fatal("aggregate-only input advertised event profiles")
	}
	page, e := a.Histogram(context.Background(), s, "immutable", s.Records[0].ID, 0, 1)
	if e != nil {
		t.Fatal(e)
	}
	if page.RetainedCount != "2" || page.Rows[0].Count != "1" || page.Rows[0].Width != 1 || page.Next != 1 {
		t.Fatal(page)
	}
	zero, e := a.Histogram(context.Background(), s, "immutable", s.Records[0].ID, 1, 1)
	if e != nil || zero.Rows[0].Count != "0" || zero.Rows[0].Width != 0 {
		t.Fatal(zero, e)
	}
	if _, e = a.Histogram(context.Background(), s, "immutable", "invented", 0, 25); e == nil {
		t.Fatal("invented histogram reference")
	}
	if _, e = a.Histogram(context.Background(), s, "immutable", s.Records[0].ID, -1, 25); e == nil {
		t.Fatal("negative page")
	}
}

func TestR1504IgnoredTIDsRetainRawEvidenceButExcludeCosts(t *testing.T) {
	s := model(t, "system-evidence-set.json")
	ctx := context.Background()
	tid := s.Capture.Entities[0].TID
	id := s.Capture.Entities[0].ID
	s.Capture.IgnoredTIDs = []string{tid}
	if e := s.Validate(ctx); e != nil {
		t.Fatal(e)
	}
	before := len(s.Records)
	rows, e := a.Intervals(ctx, s)
	if e != nil {
		t.Fatal(e)
	}
	for _, row := range rows {
		if row.Entity == id {
			t.Fatal("ignored interval leaked")
		}
	}
	result, e := a.Build(ctx, s, "ignored")
	if e != nil {
		t.Fatal(e)
	}
	if result.Summary["record_count"] != fmt.Sprint(before) {
		t.Fatal("raw records dropped")
	}
	s.Capture.IgnoredTIDs = []string{"999999"}
	if s.Validate(ctx) == nil {
		t.Fatal("undeclared ignored lifetime accepted")
	}
}

func TestR1505OffCPUStateSelectionsRemainSeparate(t *testing.T) {
	s := model(t, "system-evidence-set.json")
	total := func(metric string) *big.Int {
		t.Helper()
		view, e := a.Project(context.Background(), s, metric, nil)
		if e != nil {
			t.Fatal(e)
		}
		sum := new(big.Int)
		for _, profile := range view.Profiles {
			for _, sample := range profile.Samples {
				n, ok := new(big.Int).SetString(sample.Weight, 10)
				if !ok {
					t.Fatal(sample.Weight)
				}
				sum.Add(sum, n)
			}
		}
		if !strings.Contains(strings.Join(view.Quality.ConversionLoss, " "), "unattributed endpoint cost") {
			t.Fatal("stackless scope missing")
		}
		return sum
	}
	sleeping, runnable, all := total("off_cpu/sleeping"), total("off_cpu/runnable"), total("off_cpu")
	if sleeping.Sign() <= 0 || runnable.Sign() <= 0 || new(big.Int).Add(sleeping, runnable).Cmp(all) != 0 {
		t.Fatal(sleeping, runnable, all)
	}
	if _, e := a.Project(context.Background(), s, "off_cpu/unknown", nil); e == nil {
		t.Fatal("unknown state selector accepted")
	}
}
