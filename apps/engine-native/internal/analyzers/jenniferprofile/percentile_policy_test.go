package jenniferprofile

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/statistics"
	"math"
	"testing"
)

func TestDisclosedLegacyPercentileExample(t *testing.T) {
	values := []float64{1, 2, 3, 100}
	if got := nearestRank(values, 95); math.Abs(got-100) > 1e-10 {
		t.Fatal(got)
	}
	if statistics.PercentilePolicy("jennifer_profile")["version"] != "1.0.0" {
		t.Fatal("missing policy")
	}
}
