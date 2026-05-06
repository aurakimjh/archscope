package profiler

import (
	"path/filepath"
	"testing"
)

func BenchmarkAnalyzeCollapsedSampleWall(b *testing.B) {
	path := filepath.Join("..", "..", "..", "..", "examples", "profiler", "sample-wall.collapsed")
	options := Options{IntervalMS: 100, TopN: 20, ProfileKind: "wall"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := AnalyzeCollapsedFile(path, options); err != nil {
			b.Fatalf("analyze: %v", err)
		}
	}
}

func BenchmarkAnalyzeJenniferSample(b *testing.B) {
	path := filepath.Join("..", "..", "..", "..", "examples", "profiler", "sample-jennifer-flame.csv")
	options := Options{IntervalMS: 100, TopN: 20, ProfileKind: "wall"}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := AnalyzeJenniferFile(path, options); err != nil {
			b.Fatalf("analyze: %v", err)
		}
	}
}

func BenchmarkParseSvgBrendan(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseSvgFlamegraphText(brendanSVG)
	}
}

func BenchmarkParseHtmlAsyncProfiler(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ParseHtmlProfilerText(asyncProfilerHTML, nil)
	}
}

func BenchmarkBuildDiffFlameTree(b *testing.B) {
	baseline := map[string]int{
		"a;b;c": 100,
		"a;b;d": 60,
		"a;e":   40,
		"main;loop;tick": 220,
	}
	target := map[string]int{
		"a;b;c": 80,
		"a;b;d": 90,
		"a;e":   30,
		"main;loop;tick": 250,
		"main;new":      10,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = BuildDiffFlameTree(baseline, target, DiffOptions{Normalize: true})
	}
}

func BenchmarkExportToPprof(b *testing.B) {
	stacks := map[string]int{
		"a;b;c":   100,
		"a;b;d":   50,
		"a;e":     20,
		"main;f":  300,
	}
	dir := b.TempDir()
	out := filepath.Join(dir, "profile.pb.gz")
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := ExportToPprof(stacks, out, "samples", "count", 100); err != nil {
			b.Fatalf("export: %v", err)
		}
	}
}
