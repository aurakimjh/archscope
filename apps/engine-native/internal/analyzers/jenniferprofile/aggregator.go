// [한글] aggregator.go — 본문 이벤트 → §12 비용 ledger.
//
// 핵심 불변
//   분류기(parser 의 event_classifier) 가 이미 각 이벤트를 정확히
//   한 카테고리(SQL/CHECK_QUERY/2PC/FETCH/EXTERNAL_CALL/...) 로
//   배정했습니다. 따라서 합산은 단순 카테고리별 합으로 충분 — 카테고리
//   간 중복 시간이 발생하지 않습니다(double-counting 방지).
//
// 처리 규칙
//   • SQL 계열(EXECUTE/UPDATE/QUERY) → SQLExecuteCum + count.
//   • TWO_PC_* 6종 → TwoPCCum 한 통계로 합쳐짐(start/end/prepare/
//     commit/rollback/unknown 모두 같은 트랜잭션 phase).
//   • FETCH 의 행 수: §2.7 "최종 또는 최대 누적 fetch 건수" — last
//     value wins, 단조 증가 가정 하에 max 로 처리(누락에 안전).
//   • CONNECTION_ACQUIRE: §11.9 dedup 규칙 — startTime+elapsed 가
//     같으면 한 건. 같은 trace 가 여러 번 보고되는 경우를 한 번으로.
//
// 누적 vs Wall-clock
//   여기서는 "누적(cumulative)" 만 계산. 외부호출 병렬도(wall-clock)
//   는 msa_parallelism.go 에서 별도 처리.
package jenniferprofile

import (
	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// AggregateBody walks the body events and produces the §12 cost
// ledger. Critical correctness rule: SQL_EXECUTE / CHECK_QUERY /
// TWO_PC / FETCH / EXTERNAL_CALL never overlap because the classifier
// already categorised each event into exactly one slot. We just sum
// elapsed-ms per category here.
func AggregateBody(p *models.JenniferTransactionProfile) models.JenniferBodyMetrics {
	m := models.JenniferBodyMetrics{}

	// Connection-acquire events get deduped per §11.9: same
	// startTime + same elapsedMs counts as one. Map key is
	// `start|elapsed` so identical stack-derived rows fold.
	connSeen := map[string]struct{}{}

	for _, ev := range p.Body.Events {
		switch ev.EventType {
		case models.JenniferEventSQLExecute, models.JenniferEventSQLUpdate, models.JenniferEventSQLQuery:
			if ev.ElapsedMs != nil {
				m.SQLExecuteCumMs += *ev.ElapsedMs
			}
			m.SQLExecuteCount++
		case models.JenniferEventCheckQuery:
			if ev.ElapsedMs != nil {
				m.CheckQueryCumMs += *ev.ElapsedMs
			}
			m.CheckQueryCount++
		case models.JenniferEventTwoPCStart, models.JenniferEventTwoPCEnd,
			models.JenniferEventTwoPCPrepare, models.JenniferEventTwoPCCommit,
			models.JenniferEventTwoPCRollback, models.JenniferEventTwoPCUnknown:
			if ev.ElapsedMs != nil {
				m.TwoPCCumMs += *ev.ElapsedMs
			}
			m.TwoPCCount++
		case models.JenniferEventFetch:
			if ev.ElapsedMs != nil {
				m.FetchCumMs += *ev.ElapsedMs
			}
			m.FetchCount++
			// Fetch total rows uses the cumulative count column
			// (last value seen wins; if missing, fall back to
			// current). §2.7 says "최종 또는 최대 누적 fetch 건수".
			if ev.CumulativeFetchRows != nil {
				if *ev.CumulativeFetchRows > m.FetchTotalRows {
					m.FetchTotalRows = *ev.CumulativeFetchRows
				}
			} else if ev.CurrentFetchRows != nil {
				m.FetchTotalRows += *ev.CurrentFetchRows
			}
		case models.JenniferEventExternalCall:
			if ev.ElapsedMs != nil {
				m.ExternalCallCumMs += *ev.ElapsedMs
			}
			m.ExternalCallCount++
		case models.JenniferEventConnAcquire:
			elapsed := 0
			if ev.ElapsedMs != nil {
				elapsed = *ev.ElapsedMs
			}
			key := ev.EventStart + "|" + intKey(elapsed)
			if _, dup := connSeen[key]; dup {
				continue
			}
			connSeen[key] = struct{}{}
			m.ConnectionAcquireCumMs += elapsed
			m.ConnectionAcquireCount++
		case models.JenniferEventNetworkPrep:
			if ev.ElapsedMs != nil {
				m.NetworkPrepMethodCumMs += *ev.ElapsedMs
			}
			m.NetworkPrepMethodCount++
		}
	}
	// Network preparation = wrapper-method elapsed minus the
	// EXTERNAL_CALL elapsed we already counted above. This is the
	// "marshalling + DNS + TLS handshake + thread queueing" remainder
	// the user asked for. Clamped to 0 because the trailing-elapsed
	// regex can occasionally pull a longer value off the EXTERNAL_CALL
	// row than the wrapper method reports (different code paths).
	prep := m.NetworkPrepMethodCumMs - m.ExternalCallCumMs
	if prep < 0 {
		prep = 0
	}
	m.NetworkPrepCumMs = prep
	return m
}

// validateHeaderVsBody emits §12.2 warnings when the header
// pre-aggregated totals diverge from the body sums beyond tolerance.
// Severity is "warning" — Jennifer's own SQL_TIME definition varies
// across product versions so we don't block the analysis.
func validateHeaderVsBody(p *models.JenniferTransactionProfile, m models.JenniferBodyMetrics, tolerance int) {
	addWarn := func(code, msg string) {
		p.Warnings = append(p.Warnings, models.JenniferProfileIssue{Code: code, Message: msg})
	}

	if p.Header.ExternalCallMs != nil {
		if abs(m.ExternalCallCumMs-*p.Header.ExternalCallMs) > tolerance {
			addWarn("EXTERNAL_CALL_TIME_MISMATCH",
				"header EXTERNAL_CALL_TIME="+intKey(*p.Header.ExternalCallMs)+
					" but body sum="+intKey(m.ExternalCallCumMs))
		}
	}
	if p.Header.FetchTimeMs != nil {
		if abs(m.FetchCumMs-*p.Header.FetchTimeMs) > tolerance {
			addWarn("FETCH_TIME_MISMATCH",
				"header FETCH_TIME="+intKey(*p.Header.FetchTimeMs)+
					" but body sum="+intKey(m.FetchCumMs))
		}
	}
	if p.Header.SQLCount != nil {
		bodySQLCount := m.SQLExecuteCount + m.CheckQueryCount + m.TwoPCCount
		if abs(bodySQLCount-*p.Header.SQLCount) > 0 {
			addWarn("SQL_COUNT_MISMATCH",
				"header SQL_COUNT="+intKey(*p.Header.SQLCount)+
					" but body count="+intKey(bodySQLCount))
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// intKey is a tiny strconv.Itoa wrapper that keeps callers terse.
func intKey(v int) string {
	// Local helper avoids dragging strconv into the analyzer package
	// just for log strings; we only need it for warning messages.
	if v == 0 {
		return "0"
	}
	negative := v < 0
	if negative {
		v = -v
	}
	digits := []byte{}
	for v > 0 {
		digits = append([]byte{byte('0' + v%10)}, digits...)
		v /= 10
	}
	if negative {
		return "-" + string(digits)
	}
	return string(digits)
}
