// ─────────────────────────────────────────────────────────────────────
// [한글] breakdown — flame tree 를 카테고리별 표로 정리.
//
// 책임/목적
//   profiler 결과 화면의 "execution breakdown" / "component breakdown"
//   표를 만든다. flame tree 를 그대로 보여주는 대신, 분류기(classify.go)
//   가 매긴 카테고리(running / waiting / IO 등)별로 leaf 들을 그룹핑하여
//   "어디서 시간을 쓰고 있는가" 를 한눈에 보여준다.
//
// 알고리즘 흐름
//   1. iterLeafPaths(root) 로 leaf 별 exclusive samples 추출.
//   2. classifyFrames 로 leaf 의 PrimaryCategory(executive label, wait
//      reason 포함) 결정.
//   3. 카테고리별로 samples 누적 + 같은 카테고리 내 method/stack 분포
//      누적(top-N 후보 산출용).
//   4. 카테고리를 samples DESC 로 정렬한 뒤, ExecutionBreakdownRow 배열
//      생성 — 추정 시간(samples * intervalMS / 1000), originalTotal 대비
//      비율, 부모 stage 대비 비율, elapsed 대비 비율을 함께 채움.
//
// 주요 함수
//   - buildExecutionBreakdown : 실행 카테고리 표
//   - componentBreakdown      : 컴포넌트(stack 분류기 결과) 표
//
// 파이썬 측 parity
//   원본: archscope_engine.profiler.breakdown. samples 정렬, top-N cap,
//   ratio round 위치까지 byte 단위 동등성 유지.
// ─────────────────────────────────────────────────────────────────────

package profiler

// [한글] buildExecutionBreakdown — flame tree → execution breakdown 표.
// originalTotal: 전체 root samples (총 비율 계산용).
// parentTotal:   현재 stage 의 samples (drilldown 의 부모 비율 계산용).
// 각 leaf 의 PrimaryCategory 로 누적 후 samples DESC 정렬, top-N 자른 표 반환.
func buildExecutionBreakdown(root FlameNode, options Options, originalTotal int, parentTotal int) []ExecutionBreakdownRow {
	byCategory := map[string]int{}
	methods := map[string]map[string]int{}
	stacks := map[string]map[string]int{}
	classifications := map[string]stackClassification{}

	for _, leaf := range iterLeafPaths(root) {
		classification := classifyFrames(leaf.Path)
		key := classification.PrimaryCategory
		byCategory[key] += leaf.Samples
		if methods[key] == nil {
			methods[key] = map[string]int{}
			stacks[key] = map[string]int{}
			classifications[key] = classification
		}
		increment(methods[key], methodName(leaf.Path), leaf.Samples)
		increment(stacks[key], joinPath(leaf.Path), leaf.Samples)
	}

	rows := []ExecutionBreakdownRow{}
	intervalSeconds := options.IntervalMS / 1000
	for _, key := range sortedSegmentsBySamples(byCategory) {
		samples := byCategory[key]
		estimated := round(float64(samples)*intervalSeconds, 3)
		classification := classifications[key]
		rows = append(rows, ExecutionBreakdownRow{
			Category:         key,
			ExecutiveLabel:   classification.Label,
			PrimaryCategory:  key,
			WaitReason:       classification.WaitReason,
			Samples:          samples,
			EstimatedSeconds: estimated,
			TotalRatio:       ratio(samples, originalTotal, 4),
			ParentStageRatio: ratio(samples, parentTotal, 4),
			ElapsedRatio:     elapsedRatio(estimated, options.ElapsedSec, 4),
			TopMethods:       topCounter(methods[key], options.TopN),
			TopStacks:        topCounter(stacks[key], options.TopN),
		})
	}
	return rows
}

// [한글] componentBreakdown — collapsed stack 들을 컴포넌트(분류기 라벨)
// 단위로 합산. classifyStack 이 stack key 전체를 보고 컴포넌트(예: "JDBC",
// "HTTP Client", "JVM Internal") 를 결정하면 그 단위로 samples 합산해 표를 만든다.
func componentBreakdown(stacks map[string]int) []ComponentBreakdownRow {
	counter := map[string]int{}
	for stack, samples := range stacks {
		counter[classifyStack(stack)] += samples
	}
	top := topCounter(counter, 0)
	out := make([]ComponentBreakdownRow, 0, len(top))
	for _, item := range top {
		out = append(out, ComponentBreakdownRow{Component: item.Name, Samples: item.Samples})
	}
	return out
}
