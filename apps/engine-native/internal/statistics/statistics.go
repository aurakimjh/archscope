// Package statistics ports archscope_engine.common.statistics —
// the Average, Percentile, and BoundedPercentile helpers that the
// access-log / GC-log / profiler analyzers all consume.
//
// Rather than depending on a 3rd-party stats library, we keep the
// surface tiny so the cross-engine parity job (T-244 / T-390) can
// compare numbers byte-for-byte against the Python implementation.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] statistics 패키지 — 분석기가 공유하는 작은 통계 헬퍼.
//
// 3개 핵심 도구
//   1) Average(values []float64) float64
//        빈 입력은 0 반환 (NaN/panic 안 함). Python 측 동치.
//
//   2) Percentile(values []float64, p float64) float64
//        sort 후 linear interpolation 으로 p-percentile 계산.
//        시간복잡도 O(n log n) — 작은 표본 (수백 ~ 수천) 에 적합.
//
//   3) BoundedPercentile (Reservoir sampling)
//        대용량 입력에서 메모리 상한을 두고 percentile 추정.
//        • capacity (e.g. 10_000) + seed.
//        • Add(v) 마다 reservoir 가 가득 찼으면 무작위 교체.
//        • Percentile(p) 가 호출될 때 reservoir 만 sort 해 추정.
//        • capacity 미만 입력에서는 sort 결과가 정확 (Python 과 동일).
//
// T-049 정책 (bounded percentile)
//   대용량 access log / GC log 에서 unbounded array 가 메모리를 폭주
//   시키는 문제를 reservoir 로 해결. 정확도-메모리 trade-off 가
//   PARSER_DESIGN.md 에 문서화되어 있음.
//
// RNG 차이
//   Python 은 Mersenne Twister, Go 는 PCG (math/rand/v2) 를 사용.
//   reservoir 가 가득 차지 않으면 결과가 byte 단위로 일치, 가득 찬
//   이후의 추정값은 두 RNG 차이로 약간 다를 수 있음 (parity gate 도
//   같은 seed 로 동일 입력에서만 비교).
package statistics

import (
	"errors"
	"math/rand/v2"
	"sort"
)

// Average mirrors Python's `average(values)` — returns 0 on empty
// input rather than raising / NaN, which is what the Python analyzers
// rely on for cleanly empty buckets.
func Average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// Percentile mirrors Python's linear-interpolation percentile.
// `percent` is in 0..100. Empty input returns 0.
func Percentile(values []float64, percent float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	return percentileFromSorted(ordered, percent)
}

func percentileFromSorted(ordered []float64, percent float64) float64 {
	if len(ordered) == 1 {
		return ordered[0]
	}
	rank := float64(len(ordered)-1) * (percent / 100)
	lower := int(rank)
	upper := lower + 1
	if upper >= len(ordered) {
		upper = len(ordered) - 1
	}
	weight := rank - float64(lower)
	return ordered[lower]*(1-weight) + ordered[upper]*weight
}

// BoundedPercentile mirrors Python's reservoir-sampling
// `BoundedPercentile`. Use when the input stream might be unbounded
// (e.g. millions of access-log latencies) — keeps a fixed-size sample
// while the reservoir fill stays uniform across the full stream.
type BoundedPercentile struct {
	maxSamples  int
	count       int
	samples     []float64
	rng         *rand.Rand
	sortedCache []float64
}

// NewBoundedPercentile constructs a sampler. `maxSamples` and `seed`
// must both be positive — same constraint as Python's `__post_init__`.
func NewBoundedPercentile(maxSamples int, seed uint64) (*BoundedPercentile, error) {
	if maxSamples <= 0 {
		return nil, errors.New("max_samples must be a positive integer")
	}
	if seed == 0 {
		return nil, errors.New("seed must be a positive integer")
	}
	src := rand.NewPCG(seed, seed^0x9E3779B97F4A7C15)
	return &BoundedPercentile{
		maxSamples: maxSamples,
		samples:    make([]float64, 0, maxSamples),
		rng:        rand.New(src),
	}, nil
}

// Add ingests a value. While the reservoir is filling, every sample
// is kept; once full, each new sample displaces a random existing
// slot with probability `maxSamples/count` so the reservoir remains
// a uniform sample of the full stream.
func (b *BoundedPercentile) Add(value float64) {
	b.count++
	b.sortedCache = nil
	if len(b.samples) < b.maxSamples {
		b.samples = append(b.samples, value)
		return
	}
	idx := b.rng.IntN(b.count)
	if idx < b.maxSamples {
		b.samples[idx] = value
	}
}

// Percentile returns the requested percentile from the reservoir.
// 0 on empty.
func (b *BoundedPercentile) Percentile(percent float64) float64 {
	if len(b.samples) == 0 {
		return 0
	}
	if b.sortedCache == nil {
		b.sortedCache = append([]float64(nil), b.samples...)
		sort.Float64s(b.sortedCache)
	}
	return percentileFromSorted(b.sortedCache, percent)
}

// SampleSize is the current reservoir occupancy (capped at
// maxSamples).
func (b *BoundedPercentile) SampleSize() int {
	return len(b.samples)
}

// Count is the total number of values ingested (uncapped). Lets
// callers compute approximation confidence ratios.
func (b *BoundedPercentile) Count() int {
	return b.count
}
