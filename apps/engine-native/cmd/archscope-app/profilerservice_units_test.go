package main

import (
	"os"
	"path/filepath"
	"testing"

	p "github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
	"github.com/google/pprof/profile"
)

func TestProfilerServiceUnitFlowsThroughDrilldownAndExport(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "lock.collapsed")
	if err := os.WriteFile(input, []byte("A;Monitor 1000000\n"), 0600); err != nil {
		t.Fatal(err)
	}
	req := AnalyzeRequest{Path: input, Format: "collapsed", ProfileKind: "lock", ValueUnit: "nanoseconds", IntervalMS: 100, ProgressLogDir: dir}
	s := &ProfilerService{}
	stages, err := s.Drilldown(DrilldownRequest{AnalyzeRequest: req, Filters: []p.DrilldownFilter{{Pattern: "Monitor", FilterType: "include_text"}}})
	if err != nil {
		t.Fatal(err)
	}
	seconds := stages[1].Metrics["estimated_seconds"].(*float64)
	if seconds == nil || *seconds != 0.001 {
		t.Fatal("service dropped input unit")
	}
	out, err := s.ExportPprof(ExportPprofRequest{AnalyzeRequest: req, OutputPath: filepath.Join(dir, "lock.pb.gz")})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(out.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	parsed, err := profile.Parse(f)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed.SampleType) != 1 || parsed.SampleType[0].Unit != "nanoseconds" {
		t.Fatal("service export lost source units")
	}
}
