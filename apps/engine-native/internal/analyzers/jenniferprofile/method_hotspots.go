// [한글] method_hotspots.go — 메소드별 "순수 self-time" 추출.
//
// 문제
//
//	MSA 타임라인에서 메소드 elapsed 가 길어도 그 시간 대부분이 SQL·외부
//	호출·fetch 대기인 경우가 많아, 정작 어떤 메소드의 "자체 코드"가 느린지
//	보이지 않는다.
//
// 계산
//
//	Jennifer body 이벤트는 평탄(flat)하지만 StartOffset/EndOffset(또는
//	elapsed) 로 구간이 주어지므로 구간 포함관계로 호출 트리를 복원할 수
//	있다. 각 메소드 프레임 M 에 대해:
//
//	    self(M) = elapsed(M) − union(직접 자식 구간)
//
//	직접 자식에는 중첩 메소드/SQL/외부호출/fetch/connection 이 모두 포함되며
//	union 으로 합치므로 병렬(겹치는) 자식은 한 번만 차감된다. 자식 사이의
//	빈 구간은 메소드 자체 시간으로 남는다. 리프 SQL/외부호출 프레임은 메소드가
//	아니므로 랭킹 대상에서 제외되고, 그 시간은 부모 메소드에서 차감된다.
package jenniferprofile

import (
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// DefaultMethodHotspotLimit bounds how many ranked methods one scope
// (profile or group) contributes — generous enough that group roll-ups
// stay accurate while keeping tables bounded.
const DefaultMethodHotspotLimit = 50

// isMethodFrameEvent reports whether an event is a "method frame" we
// rank. NETWORK_PREP_METHOD is intentionally not ranked here: once a
// user classifies a wrapper as network prep, its remainder belongs to
// the Network Prep ledger instead of the general slow-method list.
func isMethodFrameEvent(t models.JenniferEventType) bool {
	return t == models.JenniferEventMethod
}

// isStructuralEvent marks profile boundary rows that are not real work
// frames and must be excluded from the containment tree.
func isStructuralEvent(t models.JenniferEventType) bool {
	return t == models.JenniferEventStart ||
		t == models.JenniferEventEnd ||
		t == models.JenniferEventTotal
}

// nestedCostCategory buckets a nested child for the per-method
// breakdown bar. SQL/cursor/transaction work folds into "sql";
// EXTERNAL_CALL into "external"; child methods into "method"; the rest
// (connection-acquire, socket, unknown, info) into "other".
func nestedCostCategory(t models.JenniferEventType) string {
	switch t {
	case models.JenniferEventMethod, models.JenniferEventNetworkPrep:
		return "method"
	case models.JenniferEventSQLExecute, models.JenniferEventSQLUpdate, models.JenniferEventSQLQuery,
		models.JenniferEventCheckQuery, models.JenniferEventFetch,
		models.JenniferEventTwoPCStart, models.JenniferEventTwoPCEnd, models.JenniferEventTwoPCPrepare,
		models.JenniferEventTwoPCCommit, models.JenniferEventTwoPCRollback, models.JenniferEventTwoPCUnknown:
		return "sql"
	case models.JenniferEventExternalCall:
		return "external"
	default:
		return "other"
	}
}

// normalizeMethodSignature collapses whitespace so the same method
// aggregates regardless of incidental formatting in the raw row.
func normalizeMethodSignature(raw string) string {
	s := strings.Join(strings.Fields(raw), " ")
	if s == "" {
		return "(anonymous method)"
	}
	return s
}

// frameNode pairs an event index with its interval for the sweep.
type frameNode struct {
	idx int
	iv  interval
}

// containmentParents returns parentIdx-by-eventIdx for every
// non-structural event that carries a usable interval. Parent = the
// tightest enclosing event. Sweeping the intervals in (start asc, end
// desc, idx asc) order with a stack reconstructs the call tree even
// when sibling intervals overlap (parallel external calls).
func containmentParents(events []models.JenniferProfileEvent) map[int]int {
	nodes := make([]frameNode, 0, len(events))
	for i := range events {
		if isStructuralEvent(events[i].EventType) {
			continue
		}
		iv, ok := eventOffsetInterval(events[i])
		if !ok {
			continue
		}
		nodes = append(nodes, frameNode{idx: i, iv: iv})
	}
	sort.SliceStable(nodes, func(a, b int) bool {
		na, nb := nodes[a], nodes[b]
		if na.iv.start != nb.iv.start {
			return na.iv.start < nb.iv.start
		}
		if na.iv.end != nb.iv.end {
			return na.iv.end > nb.iv.end // outer (longer) first
		}
		return na.idx < nb.idx
	})

	parents := make(map[int]int, len(nodes))
	stack := make([]frameNode, 0, 16)
	for _, n := range nodes {
		for len(stack) > 0 {
			top := stack[len(stack)-1]
			if intervalContains(top.iv, n.iv) {
				break
			}
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			parents[n.idx] = stack[len(stack)-1].idx
		} else {
			parents[n.idx] = -1
		}
		stack = append(stack, n)
	}
	return parents
}

// methodHotspotAgg accumulates one method signature's occurrences.
type methodHotspotAgg struct {
	method      string
	self        int
	total       int
	calls       int
	maxSelf     int
	childMethod int
	sql         int
	external    int
	other       int
}

// MethodHotspots computes per-method pure self-time for one profile,
// aggregated by normalized signature and ranked self-time descending.
// limit <= 0 falls back to DefaultMethodHotspotLimit.
func MethodHotspots(p models.JenniferTransactionProfile, limit int) []models.JenniferMethodHotspot {
	return methodHotspots(p, limit, nil)
}

// MethodHotspotsWithCustomRules applies the same slow-method ranking,
// but excludes method events already carved out by user-defined
// method rules. Excluded events still participate in the containment
// tree, so an enclosing method's self-time keeps subtracting them.
func MethodHotspotsWithCustomRules(
	p models.JenniferTransactionProfile,
	limit int,
	rules []models.JenniferCustomAnalysisRule,
) []models.JenniferMethodHotspot {
	return methodHotspots(p, limit, customRuleMethodHotspotExclusions(p.Body.Events, rules))
}

func methodHotspots(
	p models.JenniferTransactionProfile,
	limit int,
	excludedIndexes map[int]struct{},
) []models.JenniferMethodHotspot {
	events := p.Body.Events
	if len(events) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = DefaultMethodHotspotLimit
	}

	parents := containmentParents(events)
	childrenByParent := make(map[int][]int, len(parents))
	for childIdx, parentIdx := range parents {
		if parentIdx >= 0 {
			childrenByParent[parentIdx] = append(childrenByParent[parentIdx], childIdx)
		}
	}

	aggs := map[string]*methodHotspotAgg{}
	for i := range events {
		ev := events[i]
		if !isMethodFrameEvent(ev.EventType) || ev.ElapsedMs == nil {
			continue
		}
		if _, excluded := excludedIndexes[i]; excluded {
			continue
		}
		inclusive := *ev.ElapsedMs
		if inclusive < 0 {
			inclusive = 0
		}

		var allChild, catMethod, catSQL, catExternal, catOther []interval
		for _, c := range childrenByParent[i] {
			iv, ok := eventOffsetInterval(events[c])
			if !ok {
				continue
			}
			allChild = append(allChild, iv)
			switch nestedCostCategory(events[c].EventType) {
			case "method":
				catMethod = append(catMethod, iv)
			case "sql":
				catSQL = append(catSQL, iv)
			case "external":
				catExternal = append(catExternal, iv)
			default:
				catOther = append(catOther, iv)
			}
		}
		self := inclusive - unionDuration(allChild)
		if self < 0 {
			self = 0
		}

		key := normalizeMethodSignature(ev.RawMessage)
		a := aggs[key]
		if a == nil {
			a = &methodHotspotAgg{method: key}
			aggs[key] = a
		}
		a.self += self
		a.total += inclusive
		a.calls++
		if self > a.maxSelf {
			a.maxSelf = self
		}
		a.childMethod += unionDuration(catMethod)
		a.sql += unionDuration(catSQL)
		a.external += unionDuration(catExternal)
		a.other += unionDuration(catOther)
	}

	out := make([]models.JenniferMethodHotspot, 0, len(aggs))
	for _, a := range aggs {
		if a.self <= 0 {
			continue
		}
		h := models.JenniferMethodHotspot{
			Method:         a.method,
			Application:    p.Header.Application,
			TXID:           p.Header.TXID,
			GUID:           p.Header.GUID,
			SelfTimeMs:     a.self,
			TotalElapsedMs: a.total,
			Calls:          a.calls,
			MaxSelfMs:      a.maxSelf,
			ChildMethodMs:  a.childMethod,
			SqlMs:          a.sql,
			ExternalMs:     a.external,
			OtherMs:        a.other,
		}
		if a.calls > 0 {
			h.AvgSelfMs = float64(a.self) / float64(a.calls)
		}
		if a.total > 0 {
			h.SelfRatio = float64(a.self) / float64(a.total)
		}
		out = append(out, h)
	}
	sortMethodHotspots(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func customRuleMethodHotspotExclusions(
	events []models.JenniferProfileEvent,
	rules []models.JenniferCustomAnalysisRule,
) map[int]struct{} {
	normalized := normalizeCustomRules(rules)
	if len(normalized) == 0 {
		return nil
	}
	excluded := map[int]struct{}{}
	for _, rule := range normalized {
		if rule.Source != customRuleSourceMethod {
			continue
		}
		for _, match := range customRuleMethodMatches(events, rule.Patterns) {
			if match.index >= 0 && match.index < len(events) &&
				isMethodFrameEvent(events[match.index].EventType) {
				excluded[match.index] = struct{}{}
			}
		}
	}
	if len(excluded) == 0 {
		return nil
	}
	return excluded
}

// RollUpMethodHotspots folds per-profile hotspots into a group-level
// ranking keyed by Application+Method, so the MSA timeline can show the
// slowest pure methods across the whole GUID group.
func RollUpMethodHotspots(hotspots []models.JenniferMethodHotspot, guid string, limit int) []models.JenniferMethodHotspot {
	if len(hotspots) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = DefaultMethodHotspotLimit
	}
	type key struct{ app, method string }
	aggs := map[key]*models.JenniferMethodHotspot{}
	for i := range hotspots {
		h := hotspots[i]
		k := key{app: h.Application, method: h.Method}
		a := aggs[k]
		if a == nil {
			a = &models.JenniferMethodHotspot{Method: h.Method, Application: h.Application, GUID: guid}
			aggs[k] = a
		}
		a.SelfTimeMs += h.SelfTimeMs
		a.TotalElapsedMs += h.TotalElapsedMs
		a.Calls += h.Calls
		if h.MaxSelfMs > a.MaxSelfMs {
			a.MaxSelfMs = h.MaxSelfMs
		}
		a.ChildMethodMs += h.ChildMethodMs
		a.SqlMs += h.SqlMs
		a.ExternalMs += h.ExternalMs
		a.OtherMs += h.OtherMs
	}

	out := make([]models.JenniferMethodHotspot, 0, len(aggs))
	for _, a := range aggs {
		if a.Calls > 0 {
			a.AvgSelfMs = float64(a.SelfTimeMs) / float64(a.Calls)
		}
		if a.TotalElapsedMs > 0 {
			a.SelfRatio = float64(a.SelfTimeMs) / float64(a.TotalElapsedMs)
		}
		out = append(out, *a)
	}
	sortMethodHotspots(out)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// sortMethodHotspots orders by self-time desc, then inclusive desc,
// then application+method for a stable, deterministic ranking.
func sortMethodHotspots(hs []models.JenniferMethodHotspot) {
	sort.SliceStable(hs, func(i, j int) bool {
		if hs[i].SelfTimeMs != hs[j].SelfTimeMs {
			return hs[i].SelfTimeMs > hs[j].SelfTimeMs
		}
		if hs[i].TotalElapsedMs != hs[j].TotalElapsedMs {
			return hs[i].TotalElapsedMs > hs[j].TotalElapsedMs
		}
		if hs[i].Application != hs[j].Application {
			return hs[i].Application < hs[j].Application
		}
		return hs[i].Method < hs[j].Method
	})
}
