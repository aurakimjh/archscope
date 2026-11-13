package databaselog

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/statistics"
	"math"
	"testing"
)

func TestDisclosedLegacyPercentileExample(t *testing.T) {
	values := []float64{1, 2, 3, 100}
	if got := percentile(values, 95); math.Abs(got-3) > 1e-10 {
		t.Fatal(got)
	}
	if statistics.PercentilePolicy("database_slow_query")["version"] != "1.0.0" {
		t.Fatal("missing policy")
	}
}
