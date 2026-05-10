// [한글] msa_stats.go — §19-§21 Timeline Signature 통계.
//
// Timeline Signature 의 의미
//
//	GUID 는 한 트랜잭션 인스턴스를 식별하는 키입니다. 그러나 같은
//	비즈니스 호출 패턴(예: order 생성 → user 조회 → inventory 차감)
//	은 인스턴스마다 GUID 가 다르더라도 "구조" 는 같습니다.
//	Signature 는 그 "구조" 를 GUID 와 무관하게 고정 — 같은 시그니처를
//	가진 인스턴스 N개를 한 데 모아 분포 통계(p50/p90/p95/p99) 를
//	낼 수 있게 함.
//
// canonical signature 정의 (§19)
//   - 시간 순서 무시(병렬 호출에서도 안정된 시그니처가 되도록).
//   - caller_application → callee_application 엣지 문자열을 사전순
//     정렬.
//   - 동일 엣지가 여러 번이면 occurrence_index 로 disambiguate
//     (예: A→B(1), A→B(2)).
//   - 시그니처 hash = SHA-256(canonical 문자열) — 짧고 비교 빠름.
//   - SignatureVersion ("v1") 으로 알고리즘 버전 stamp — 알고리즘이
//     바뀌면 옛 시그니처와 섞이지 않도록.
//
// 통계
//
//	같은 signature_hash 를 가진 GUID 그룹들의 메트릭 분포:
//	  external_call_elapsed / callee_response_time / raw_network_gap /
//	  adjusted_network_gap → count/min/avg/p50/p90/p95/p99/max/stddev.
//	엣지별 통계도 같은 방식으로 산출(occurrence_index 로 동일 엣지
//	반복 호출 분리).
//
// 백분위 계산
//
//	sample_count 가 작은(예: <10) signature 도 의미를 가질 수 있으므로
//	linear interpolation 표준 알고리즘을 그대로 사용. 큰 sample 에서는
//	sort 후 선형 보간.
package jenniferprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// SignatureVersion stamps the signature algorithm. Bump when
// canonicalSignature() changes shape so old vs new aren't compared.
const SignatureVersion = "v1"

// computeTimelineSignature builds the §19 fingerprint of one GUID
// group's call structure. Time-order is deliberately ignored
// (parallelism-safe per §19.3); we instead sort caller→callee edge
// strings lexicographically and tag duplicates with an occurrence
// index.
func computeTimelineSignature(group models.JenniferGuidGroup) models.JenniferTimelineSignature {
	rootApp := group.RootApplication
	edges := make([]edgeKeyTuple, 0, len(group.CallGraph))
	occurrence := map[string]int{}
	for _, e := range group.CallGraph {
		base := fmt.Sprintf("%s->%s", e.CallerApplication, e.CalleeApplication)
		occurrence[base]++
		edges = append(edges, edgeKeyTuple{
			caller:    e.CallerApplication,
			callee:    e.CalleeApplication,
			occurence: occurrence[base],
		})
	}
	sort.SliceStable(edges, func(i, j int) bool {
		if edges[i].caller != edges[j].caller {
			return edges[i].caller < edges[j].caller
		}
		if edges[i].callee != edges[j].callee {
			return edges[i].callee < edges[j].callee
		}
		return edges[i].occurence < edges[j].occurence
	})

	var b strings.Builder
	b.WriteString(rootApp)
	b.WriteByte('\n')
	for _, e := range edges {
		fmt.Fprintf(&b, "%s->%s#%d\n", e.caller, e.callee, e.occurence)
	}
	canonical := b.String()
	sum := sha256.Sum256([]byte(canonical))

	return models.JenniferTimelineSignature{
		Version:         SignatureVersion,
		Hash:            hex.EncodeToString(sum[:]),
		Canonical:       canonical,
		RootApplication: rootApp,
		EdgeCount:       len(edges),
	}
}

// edgeKeyTuple is the matcher's view of one edge during signature
// computation — used for sorting + occurrence tagging.
type edgeKeyTuple struct {
	caller    string
	callee    string
	occurence int
}

// aggregateSignatureStats walks every GUID group, computes its
// signature, then folds groups sharing a signature into a single
// JenniferSignatureStats entry with per-metric distribution stats
// (§21.2) and per-edge stats (§21.3).
func aggregateSignatureStats(groups []models.JenniferGuidGroup) []models.JenniferSignatureStats {
	type bucket struct {
		signature models.JenniferTimelineSignature
		groups    []models.JenniferGuidGroup
	}
	buckets := map[string]*bucket{}
	order := []string{}

	for _, g := range groups {
		sig := computeTimelineSignature(g)
		b, ok := buckets[sig.Hash]
		if !ok {
			b = &bucket{signature: sig}
			buckets[sig.Hash] = b
			order = append(order, sig.Hash)
		}
		b.groups = append(b.groups, g)
	}

	out := make([]models.JenniferSignatureStats, 0, len(order))
	for _, h := range order {
		b := buckets[h]
		stats := models.JenniferSignatureStats{
			Signature:   b.signature,
			SampleCount: len(b.groups),
			Metrics:     map[string]models.JenniferMetricStats{},
		}
		for _, g := range b.groups {
			stats.GUIDs = append(stats.GUIDs, g.GUID)
		}
		stats.Metrics = computeGroupMetricStats(b.groups)
		stats.Edges = computeEdgeStats(b.groups)
		out = append(out, stats)
	}

	// Stable, count-desc ordering so the renderer surfaces the
	// busiest signature first.
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].SampleCount != out[j].SampleCount {
			return out[i].SampleCount > out[j].SampleCount
		}
		return out[i].Signature.Hash < out[j].Signature.Hash
	})
	return out
}

// computeGroupMetricStats applies the §21.2 metric list. Each metric
// pulls one float per GUID group; we then run nearest-rank
// percentile (§22) over the resulting slice.
func computeGroupMetricStats(groups []models.JenniferGuidGroup) map[string]models.JenniferMetricStats {
	pull := func(extract func(models.JenniferGuidMetrics, models.JenniferGuidGroup) (float64, bool)) []float64 {
		values := make([]float64, 0, len(groups))
		for _, g := range groups {
			if v, ok := extract(g.Metrics, g); ok {
				values = append(values, v)
			}
		}
		return values
	}

	metrics := map[string]models.JenniferMetricStats{}
	addMetric := func(name string, values []float64) {
		metrics[name] = computeMetricStats(values)
	}

	addMetric("root_response_time_ms", pull(func(_ models.JenniferGuidMetrics, g models.JenniferGuidGroup) (float64, bool) {
		if g.RootResponseTimeMs == nil {
			return 0, false
		}
		return float64(*g.RootResponseTimeMs), true
	}))
	addMetric("total_external_call_cumulative_ms", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.TotalExternalCallCumulativeMs), true
	}))
	addMetric("total_network_gap_cumulative_ms", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.TotalNetworkGapCumulativeMs), true
	}))
	addMetric("total_unprofiled_external_call_ms", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.TotalUnprofiledExternalCallMs), true
	}))
	addMetric("total_sql_execute_ms", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.TotalSqlExecuteMs), true
	}))
	addMetric("total_check_query_ms", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.TotalCheckQueryMs), true
	}))
	addMetric("total_two_pc_ms", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.TotalTwoPcMs), true
	}))
	addMetric("total_fetch_ms", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.TotalFetchMs), true
	}))
	addMetric("total_fetch_rows", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.TotalFetchRows), true
	}))
	addMetric("total_connection_acquire_ms", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.TotalConnectionAcquireMs), true
	}))
	addMetric("matched_external_call_count", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.MatchedExternalCallCount), true
	}))
	addMetric("unmatched_external_call_count", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.UnmatchedExternalCallCount), true
	}))
	// MVP4 parallelism columns. Wall-clock + ratio + max concurrency
	// are first-class members of the §21.5 distribution table.
	addMetric("total_external_call_wall_time_ms", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.TotalExternalCallWallTimeMs), true
	}))
	addMetric("max_external_call_concurrency", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return float64(m.MaxExternalCallConcurrency), true
	}))
	addMetric("external_call_parallelism_ratio", pull(func(m models.JenniferGuidMetrics, _ models.JenniferGuidGroup) (float64, bool) {
		return m.ExternalCallParallelismRatio, true
	}))
	return metrics
}

// computeEdgeStats walks every matched edge in every group of the
// bucket, keys by (callerApp, calleeApp, occurrenceIndex), and
// summarises the four edge metrics (elapsed, calleeRT, raw gap,
// adjusted gap) per §21.3.
func computeEdgeStats(groups []models.JenniferGuidGroup) []models.JenniferEdgeStats {
	type edgeBucket struct {
		caller   string
		callee   string
		occ      int
		elapsed  []float64
		calleeRT []float64
		rawGap   []float64
		adjGap   []float64
	}
	buckets := map[string]*edgeBucket{}
	order := []string{}

	for _, g := range groups {
		// Per-group occurrence counter so duplicate edges inside
		// one GUID get #1, #2, #3 indices.
		occ := map[string]int{}
		for _, e := range g.Edges {
			if e.MatchStatus != models.JenniferMatchOK {
				continue
			}
			base := e.CallerApplication + "->" + e.CalleeApplication
			occ[base]++
			key := fmt.Sprintf("%s#%d", base, occ[base])
			b, ok := buckets[key]
			if !ok {
				b = &edgeBucket{caller: e.CallerApplication, callee: e.CalleeApplication, occ: occ[base]}
				buckets[key] = b
				order = append(order, key)
			}
			b.elapsed = append(b.elapsed, float64(e.ExternalCallElapsedMs))
			if e.CalleeResponseTimeMs != nil {
				b.calleeRT = append(b.calleeRT, float64(*e.CalleeResponseTimeMs))
			}
			if e.RawNetworkGapMs != nil {
				b.rawGap = append(b.rawGap, float64(*e.RawNetworkGapMs))
			}
			if e.AdjustedNetworkGapMs != nil {
				b.adjGap = append(b.adjGap, float64(*e.AdjustedNetworkGapMs))
			}
		}
	}

	out := make([]models.JenniferEdgeStats, 0, len(order))
	for _, key := range order {
		b := buckets[key]
		out = append(out, models.JenniferEdgeStats{
			CallerApplication:        b.caller,
			CalleeApplication:        b.callee,
			OccurrenceIndex:          b.occ,
			ExternalCallElapsedStats: computeMetricStats(b.elapsed),
			CalleeResponseTimeStats:  computeMetricStats(b.calleeRT),
			RawNetworkGapStats:       computeMetricStats(b.rawGap),
			AdjustedNetworkGapStats:  computeMetricStats(b.adjGap),
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].CallerApplication != out[j].CallerApplication {
			return out[i].CallerApplication < out[j].CallerApplication
		}
		if out[i].CalleeApplication != out[j].CalleeApplication {
			return out[i].CalleeApplication < out[j].CalleeApplication
		}
		return out[i].OccurrenceIndex < out[j].OccurrenceIndex
	})
	return out
}

// computeMetricStats produces the §21.2 distribution shape: count,
// min, avg, p50/p90/p95/p99, max, stddev. Empty input → zero stats
// (Count=0). Percentiles use nearest-rank per §22:
//
//	p_k = sortedValues[ceil(k * n / 100) - 1]
func computeMetricStats(values []float64) models.JenniferMetricStats {
	if len(values) == 0 {
		return models.JenniferMetricStats{}
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)

	var sum float64
	for _, v := range sorted {
		sum += v
	}
	avg := sum / float64(len(sorted))

	var sqSum float64
	for _, v := range sorted {
		d := v - avg
		sqSum += d * d
	}
	stddev := 0.0
	if len(sorted) > 0 {
		stddev = math.Sqrt(sqSum / float64(len(sorted)))
	}

	return models.JenniferMetricStats{
		Count:  len(sorted),
		Min:    sorted[0],
		Avg:    avg,
		P50:    nearestRank(sorted, 50),
		P90:    nearestRank(sorted, 90),
		P95:    nearestRank(sorted, 95),
		P99:    nearestRank(sorted, 99),
		Max:    sorted[len(sorted)-1],
		Stddev: stddev,
	}
}

// nearestRank implements §22's MVP percentile rule:
//
//	p_k = sortedValues[ceil(k * n / 100) - 1]
//
// Empty input returns 0 (won't happen in practice — caller checks).
func nearestRank(sorted []float64, percent float64) float64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	idx := int(math.Ceil(percent*float64(n)/100)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}
