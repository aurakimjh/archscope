package main

import (
	"os"
	"path/filepath"
	"testing"

	p "github.com/aurakimjh/archscope/apps/engine-native/internal/profiler"
)

func TestDrilldownFiltersCanonicalEvidenceBeforePruning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "profile.collapsed")
	if err := os.WriteFile(path, []byte("a;hot 100\na;rare 10\n"), 0600); err != nil {
		t.Fatal(err)
	}
	service := &ProfilerService{}
	for _, limit := range []int{1, 3, 100} {
		stages, err := service.Drilldown(DrilldownRequest{
			AnalyzeRequest: AnalyzeRequest{Path: path, Format: "collapsed", IntervalMS: 100, TopN: 10, MaxFlamegraphNodes: limit, ProgressLogDir: t.TempDir()},
			Filters:        []p.DrilldownFilter{{Pattern: "rare", FilterType: "include_text", MatchMode: "anywhere"}},
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(stages) != 2 || stages[1].Flamegraph.Samples != 10 {
			t.Fatalf("limit %d: rare samples lost: %+v", limit, stages)
		}
		var nodes func(p.FlameNode) int
		nodes = func(n p.FlameNode) int {
			sum := 1
			for _, c := range n.Children {
				sum += nodes(c)
			}
			return sum
		}
		for _, stage := range stages {
			if nodes(stage.Flamegraph) > limit {
				t.Fatalf("limit %d exceeded", limit)
			}
		}
	}
}
