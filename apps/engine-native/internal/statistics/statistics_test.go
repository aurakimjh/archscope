package statistics

import (
	"math"
	"testing"
)

func TestAverageEmpty(t *testing.T) {
	if got := Average(nil); got != 0 {
		t.Fatalf("Average(nil) = %v, want 0", got)
	}
}

func TestAverage(t *testing.T) {
	if got := Average([]float64{1, 2, 3, 4}); got != 2.5 {
		t.Fatalf("Average = %v, want 2.5", got)
	}
}

func TestPercentileMatchesPythonLinearInterpolation(t *testing.T) {
	// Python's `percentile([1..10], 50)` returns 5.5; the formula is
	// `(n-1) * percent/100` rank with linear interpolation between
	// the two surrounding ordered values.
	values := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	if got := Percentile(values, 50); got != 5.5 {
		t.Fatalf("p50 = %v, want 5.5", got)
	}
	if got := Percentile(values, 90); got != 9.1 {
		t.Fatalf("p90 = %v, want 9.1", got)
	}
	if got := Percentile(values, 0); got != 1 {
		t.Fatalf("p0 = %v, want 1", got)
	}
	if got := Percentile(values, 100); got != 10 {
		t.Fatalf("p100 = %v, want 10", got)
	}
}

func TestPercentileSingleValue(t *testing.T) {
	if got := Percentile([]float64{42}, 99); got != 42 {
		t.Fatalf("single-value percentile = %v, want 42", got)
	}
}

func TestPercentileEmpty(t *testing.T) {
	if got := Percentile(nil, 50); got != 0 {
		t.Fatalf("empty percentile = %v, want 0", got)
	}
}

func TestBoundedPercentileRejectsBadSeed(t *testing.T) {
	if _, err := NewBoundedPercentile(100, 0); err == nil {
		t.Fatalf("expected error for seed=0")
	}
	if _, err := NewBoundedPercentile(0, 1); err == nil {
		t.Fatalf("expected error for maxSamples=0")
	}
}

func TestBoundedPercentileFullReservoir(t *testing.T) {
	bp, err := NewBoundedPercentile(1000, 42)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	for i := 1; i <= 1000; i++ {
		bp.Add(float64(i))
	}
	if bp.SampleSize() != 1000 {
		t.Fatalf("sample size = %d, want 1000", bp.SampleSize())
	}
	if bp.Count() != 1000 {
		t.Fatalf("count = %d, want 1000", bp.Count())
	}
	// Full reservoir is the deterministic input itself; p50 is exact.
	if got := bp.Percentile(50); got != 500.5 {
		t.Fatalf("p50 = %v, want 500.5", got)
	}
}

func TestBoundedPercentileReservoirIsRepresentative(t *testing.T) {
	// 100k samples through a 1k-cap reservoir; the sampled p50 should
	// stay within ~5% of the true median (50000.5).
	bp, err := NewBoundedPercentile(1_000, 42)
	if err != nil {
		t.Fatalf("init: %v", err)
	}
	for i := 1; i <= 100_000; i++ {
		bp.Add(float64(i))
	}
	if bp.Count() != 100_000 {
		t.Fatalf("count = %d, want 100k", bp.Count())
	}
	if bp.SampleSize() != 1_000 {
		t.Fatalf("sample size = %d, want 1k", bp.SampleSize())
	}
	got := bp.Percentile(50)
	if math.Abs(got-50_000.5) > 5_000 {
		t.Fatalf("p50 = %v, expected ~50000 (±5000)", got)
	}
}
