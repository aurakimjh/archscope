package traceimport

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/statistics"
	"math"
	"testing"
)

func TestDisclosedLegacyPercentileExample(t *testing.T) {
	values := []float64{1, 2, 3, 100}
	if got := percentile(values, .95); math.Abs(got-85.45) > 1e-10 {
		t.Fatal(got)
	}
	if statistics.PercentilePolicy("trace_import")["version"] != "1.0.0" {
		t.Fatal("missing policy")
	}
}
