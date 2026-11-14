package profiler

import (
	"strconv"
	"testing"
)

func TestDiffScaledProjectionDoesNotOverflowNativeInt(t *testing.T) {
	result := AnalyzeProfilerDiff(map[string]int{"app;worker": 3000}, map[string]int{"app;worker": 4000}, "before", "after", DiffOptions{})
	root := result.Charts.Flamegraph
	if root.Samples < 0 {
		t.Fatalf("scaled projection overflowed: %d", root.Samples)
	}
	meta := root.Metadata.(map[string]any)
	if meta["a"] != float64(3000) || meta["b"] != float64(4000) {
		t.Fatal("canonical diff weights changed", meta)
	}
	if meta["samples_clamped"] != (strconv.IntSize == 32) {
		t.Fatal("projection bound must be explicit", meta)
	}
	if meta["samples_scale"] != 1000000 {
		t.Fatal("projection scale must be explicit", meta)
	}
	if strconv.IntSize == 32 && root.Samples != int(^uint(0)>>1) {
		t.Fatal("32bit projection must stop at the representable bound")
	}
}
