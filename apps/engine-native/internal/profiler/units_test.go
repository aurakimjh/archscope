package profiler

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/pprof/profile"
)

func TestProfileCounterDurationContract(t *testing.T) {
	for _, tc := range []struct {
		kind, unit string
		weight     int
		seconds    *float64
	}{
		{"lock", "nanoseconds", 1000000, floatPtr(0.001)},
		{"lock", "microseconds", 1000, floatPtr(0.001)},
		{"lock", "milliseconds", 1, floatPtr(0.001)},
		{"lock", "seconds", 1, floatPtr(1)},
		{"lock", "samples", 5, nil},
		{"lock", "", 5, nil},
		{"wall", "unknown", 5, nil},
		{"wall", "bytes", 5, nil},
		{"wall", "samples", 5, floatPtr(0.5)},
		{"cpu", "", 5, floatPtr(0.5)},
	} {
		t.Run(tc.kind+"_"+tc.unit, func(t *testing.T) {
			o := Options{ProfileKind: tc.kind, ValueUnit: tc.unit, IntervalMS: 100, TopN: 10}
			r := AnalyzeCollapsedStacks(map[string]int{"Root;work": tc.weight}, "synthetic", ParserDiagnostics{}, o)
			check := func(got *float64) {
				t.Helper()
				if tc.seconds == nil {
					if got != nil {
						t.Fatalf("invented duration %v", *got)
					}
				} else if got == nil || math.Abs(*got-*tc.seconds) > 1e-12 {
					t.Fatalf("duration %v want %v", got, tc.seconds)
				}
			}
			check(r.Summary.EstimatedSeconds)
			for _, row := range r.Series.TopStacks {
				check(row.EstimatedSeconds)
			}
			for _, row := range r.Series.ExecutionBreakdown {
				check(row.EstimatedSeconds)
			}
			for _, row := range r.Series.TimelineAnalysis {
				check(row.EstimatedSeconds)
			}
			st := BuildDrilldownStagesWithOptions(r.Charts.Flamegraph, []DrilldownFilter{{Pattern: "work", FilterType: "include_text"}}, o, r.Summary.TotalSamples)
			for _, stage := range st {
				check(stage.Metrics["estimated_seconds"].(*float64))
			}
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if tc.seconds == nil && !strings.Contains(string(b), `"estimated_seconds":null`) {
				t.Fatal("unavailable duration is not null")
			}
			if tc.seconds == nil && r.Summary.DurationUnavailableReason == "" {
				t.Fatal("missing unavailable reason")
			}
		})
	}
}

func TestPprofKeepsWeightUnitsWithoutIntervalMultiplication(t *testing.T) {
	for _, tc := range []struct {
		unit, wireUnit string
		durationColumn bool
	}{
		{"nanoseconds", "nanoseconds", false}, {"samples", "count", false}, {"unknown", "unknown", false},
	} {
		path := filepath.Join(t.TempDir(), "profile.pb.gz")
		if err := ExportToPprofWithOptions(map[string]int{"A;B": 1000000}, path, Options{ProfileKind: "lock", ValueUnit: tc.unit, IntervalMS: 100}); err != nil {
			t.Fatal(err)
		}
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		p, err := profile.Parse(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if len(p.SampleType) != 1 || p.SampleType[0].Unit != tc.wireUnit || p.Sample[0].Value[0] != 1000000 || p.Period != 0 || (p.PeriodType != nil && p.PeriodType.Type != "") {
			t.Fatalf("incorrect units: %+v", p)
		}
	}
	path := filepath.Join(t.TempDir(), "cpu.pb.gz")
	if err := ExportToPprofWithOptions(map[string]int{"A": 5}, path, Options{ProfileKind: "cpu", IntervalMS: 100}); err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(path)
	defer f.Close()
	p, err := profile.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.SampleType) != 2 || p.Sample[0].Value[1] != 500 {
		t.Fatal("CPU sample interval compatibility lost")
	}
}
