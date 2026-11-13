package statistics

import (
	"bytes"
	"math"
	"os"
	"testing"
)

func TestPolicyMirrorAndIndependentExamples(t *testing.T) {
	p := PercentilePolicy("access_log")
	if p["capacity"] != float64(10000) || p["algorithm"] != "reservoir_then_linear_interpolation" {
		t.Fatal(p)
	}
	p["capacity"] = 0
	if PercentilePolicy("access_log")["capacity"] != float64(10000) {
		t.Fatal("shared mutable policy")
	}
	b, e := NewBoundedPercentile(10000, 12345)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []float64{1, 2, 3, 100} {
		b.Add(v)
	}
	if math.Abs(b.Percentile(95)-85.45) > 1e-10 || math.Abs(Percentile([]float64{1, 2, 3, 100}, 95)-85.45) > 1e-10 {
		t.Fatal("linear example")
	}
	if shared, e := os.ReadFile("../../../../../projects-assets/diagnostics/rules/legacy-percentile-policies/percentile-policies.json"); e == nil && !bytes.Equal(shared, percentilePolicies) {
		t.Fatal("compiled mirror drift")
	}
}
