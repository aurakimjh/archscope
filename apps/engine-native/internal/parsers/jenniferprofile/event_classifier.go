// [한글] event_classifier.go — 이벤트 메시지 → JenniferEventType 분류.
//
// 분류 우선순위 (높은 순)
//  1. PROFILE_START / END / TOTAL  : 본문 경계.
//  2. EXTERNAL_CALL                : `EXTERNAL_CALL [proto] client (url=...)`
//     의 url 까지 추출.
//  3. EXTERNAL_CALL_INFO           : INFO 헤더만(시간 측정 X).
//  4. FETCH                        : `FETCH [N/M] [T ms]` — 행 수 + 시간.
//  5. TWO_PC_*                     : XA START/END/PREPARE/COMMIT/ROLLBACK
//     6종.
//  6. CHECK_QUERY                  : DB 쿼리 검사 phase.
//  7. SQL_EXECUTE_QUERY/UPDATE/...  : 실제 SQL 실행 (READ/WRITE 구분).
//  8. CONNECTION_ACQUIRE           : getConnection / PoolingDataSource.*
//     여러 패턴.
//  9. SOCKET                        : `SOCKET_OSTREAM` / `SOCEKT_ISTREAM`
//     (오타 SOCEKT 도 허용 — Jennifer
//     실제 출력에 존재).
//  10. METHOD                       : 일반 메서드 호출.
//  11. UNKNOWN                      : 위 어느 것에도 매칭 안 됨.
//
// 이렇게 우선순위 기반으로 분류해야 EXTERNAL_CALL 안에 SQL 같은 키워드
// 가 섞여도 EXTERNAL_CALL 로 정확히 라벨링됨 (예: SQL_EXECUTE 가 url
// 안 path 에 등장하는 케이스).
//
// 추출 부가 필드
//
//	EXTERNAL_CALL : protocol, client, url 분리 보관.
//	FETCH        : current/cumulative rows + elapsed ms.
//
// 비고: SOCEKT 오타
//
//	Jennifer 의 실제 export 에 `SOCEKT_OSTREAM` 오타가 등장 — 정정하지
//	말고 원본 그대로 인식해야 누락 없이 분류됨.
package jenniferprofile

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Event-message regexes per §11.
var (
	// Real-world Jennifer exports emit either EXTERNAL_CALL or
	// EXTERNAL-CALL depending on tracer version, so we accept both
	// here (and in the prefix check below). Counting EXTERNAL-CALL
	// is what unbroke the count=0 / ms=0 case the user reported on
	// the transaction profiles table.
	externalCallRE = regexp.MustCompile(
		`^EXTERNAL[_-]CALL\s+\[([^\]]+)\]\s+(\S+)\s+\(\s*url\s*=\s*([^,)]+)`,
	)
	// Note: the trailing `[NNN ms]` is stripped by extractTrailingElapsed
	// before this regex runs, so we only match the rows-portion here.
	fetchRE = regexp.MustCompile(
		`^FETCH\s*\[\s*([0-9,]+)\s*/\s*([0-9,]+)\s*\](?:\s*\[\s*([0-9,]+)\s*ms\s*\])?`,
	)
	socketRE             = regexp.MustCompile(`^(?:SOCKET|SOCEKT)_(?:OSTREAM|ISTREAM)\b`)
	externalCallInfoRE   = regexp.MustCompile(`_INFO\s*\(\s*url\s*=`)
	connectionAcquireREs = []*regexp.Regexp{
		regexp.MustCompile(`getConnection\s*\(`),
		regexp.MustCompile(`PoolingDataSource\.getConnection`),
		regexp.MustCompile(`GenericObjectPool\.borrowObject`),
		regexp.MustCompile(`LinkedBlockingDeque\.takeFirst`),
	}
	// `xa_start_new`, `xa_end_new` etc. are common in real exports;
	// we treat trailing `_word` as part of the same XA call. Word
	// boundary on the left only — leading-underscore identifiers
	// are not a concern in Jennifer SQL bodies.
	twoPCStartRE    = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])xa_start`)
	twoPCEndRE      = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])xa_end`)
	twoPCPrepareRE  = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])xa_prepare`)
	twoPCCommitRE   = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])xa_commit`)
	twoPCRollbackRE = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])xa_rollback`)
	// twoPCAnyRE picks up two flavours seen in real exports:
	//   - lowercase `xa_*` calls (`xa_start_new`, `xa_recover`, …)
	//   - the `JAVA_XA` family that wraps XA driver calls in
	//     application code; bytea/SQL bodies render the marker as
	//     `JAVA_XA` (uppercase) on its own. We accept either so the
	//     2PC ledger doesn't undercount when the tracing library
	//     normalises to the uppercase form.
	twoPCAnyRE  = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])xa_[a-z]\w*`)
	twoPCJavaRE = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])java_xa\b`)
)

// checkQueryNormalized are the canonical Check Query SQL bodies per §11.3.
var checkQueryNormalized = map[string]struct{}{
	"select 1 from dual":             {},
	"select 1":                       {},
	"select 1 from sysibm.sysdummy1": {},
	"select 1 from rdb$database":     {},
}

// defaultNetworkPrepPatterns is the built-in marker for HTTP-client
// wrapper methods. User-supplied options extend this list; they do
// not replace the built-ins because the UI treats Network Prep
// patterns as additive analysis options.
// Matched on the message text (case-insensitive substring).
var defaultNetworkPrepPatterns = []string{
	"integrationutil.sendtoservice",
	"sendtoservice(",
}

// NetworkPrepPatternsWithDefaults normalizes the built-in Network Prep
// markers plus caller-supplied additions into lower-case substring
// patterns. Callers that need to mirror classifier decisions, such as
// method residual ranking, use this to keep default/additive semantics
// identical.
func NetworkPrepPatternsWithDefaults(patterns []string) []string {
	raw := append([]string{}, defaultNetworkPrepPatterns...)
	raw = append(raw, patterns...)
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, p := range raw {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

// classifyEvents walks the body events in priority order (§11.1) and
// stamps `EventType`. SQL events are inspected against their
// detail-lines first so `select 1 from dual` becomes CHECK_QUERY
// rather than SQL_EXECUTE_GENERIC.
func classifyEvents(profile *models.JenniferTransactionProfile) {
	classifyEventsWithOptions(profile, Options{})
}

// classifyEventsWithOptions is the parser-options-aware variant. The
// user-supplied NetworkPrepPatterns and EventCategoryPatterns extend
// the built-in classifier; built-in matches still win for well-known
// forms (EXTERNAL_CALL, FETCH, etc) so a malformed user pattern can't
// silently break matched-edge correlation.
func classifyEventsWithOptions(profile *models.JenniferTransactionProfile, opts Options) {
	prepLower := NetworkPrepPatternsWithDefaults(opts.NetworkPrepPatterns)
	customRules := normalizeEventPatterns(opts.EventCategoryPatterns)

	for i := range profile.Body.Events {
		ev := &profile.Body.Events[i]
		ev.EventType = classifyOne(ev)
		// Built-in matches don't get re-classified. Only METHOD /
		// UNKNOWN events are eligible for the user-supplied patterns
		// so a typo in EventCategoryPatterns can't corrupt
		// EXTERNAL_CALL / FETCH bookkeeping.
		if ev.EventType == models.JenniferEventMethod || ev.EventType == models.JenniferEventUnknown {
			lowerMsg := strings.ToLower(ev.RawMessage)
			matchedPrep := false
			for _, p := range prepLower {
				if strings.Contains(lowerMsg, p) {
					ev.EventType = models.JenniferEventNetworkPrep
					matchedPrep = true
					break
				}
			}
			if !matchedPrep {
				for _, rule := range customRules {
					if matchAnyLower(lowerMsg, rule.patterns) {
						ev.EventType = rule.target
						break
					}
				}
			}
		}
		if ev.EventType == models.JenniferEventExternalCall {
			fillExternalCall(ev)
		}
		if ev.EventType == models.JenniferEventFetch {
			fillFetch(ev)
		}
	}
}

// validateNetworkPrep enforces the user-stated invariant after event
// offsets are available: a NETWORK_PREP_METHOD wrapper must contain
// at least one EXTERNAL_CALL. Older traces often put the wrapper
// immediately above the call, but one wrapper method can also contain
// several EXTERNAL_CALL rows before it returns.
func validateNetworkPrep(profile *models.JenniferTransactionProfile) {
	events := profile.Body.Events
	for i := range events {
		ev := &events[i]
		if ev.EventType != models.JenniferEventNetworkPrep {
			continue
		}
		callIdxs := containedExternalCallIndexes(events, i)
		if len(callIdxs) == 0 {
			nextIdx := nextRealEventIndex(events, i)
			if nextIdx >= 0 && events[nextIdx].EventType == models.JenniferEventExternalCall {
				callIdxs = append(callIdxs, nextIdx)
			}
		}
		if len(callIdxs) == 0 {
			ev.EventType = models.JenniferEventMethod
			profile.Warnings = append(profile.Warnings, models.JenniferProfileIssue{
				Code:    "NETWORK_PREP_NOT_FOLLOWED_BY_EXTERNAL_CALL",
				Message: "NETWORK_PREP wrapper method demoted: no enclosed EXTERNAL_CALL was found",
			})
			continue
		}
		if ev.ElapsedMs != nil {
			wrapperMs := *ev.ElapsedMs
			callMs := 0
			for _, idx := range callIdxs {
				if events[idx].ElapsedMs != nil {
					callMs += *events[idx].ElapsedMs
				}
			}
			diff := wrapperMs - callMs
			if diff < 0 {
				diff = -diff
			}
			// Threshold: 500 ms absolute or 50% relative — whichever
			// is larger — to avoid noise on tiny calls.
			rel := 0.0
			base := wrapperMs
			if callMs > base {
				base = callMs
			}
			if base > 0 {
				rel = float64(diff) / float64(base)
			}
			if diff > 500 && rel > 0.5 {
				profile.Warnings = append(profile.Warnings, models.JenniferProfileIssue{
					Code: "NETWORK_PREP_ELAPSED_MISMATCH",
					Message: "NETWORK_PREP wrapper elapsed differs from enclosed EXTERNAL_CALL sum by " +
						intToString(diff) + " ms (wrapper=" + intToString(wrapperMs) + " calls=" + intToString(callMs) + ")",
				})
			}
		}
	}
}

func containedExternalCallIndexes(events []models.JenniferProfileEvent, prepIdx int) []int {
	if prepIdx < 0 || prepIdx >= len(events) {
		return nil
	}
	prep := events[prepIdx]
	if prep.StartOffsetMs == nil || prep.EndOffsetMs == nil {
		return nil
	}
	out := []int{}
	for i := prepIdx + 1; i < len(events); i++ {
		ev := events[i]
		if ev.EventType != models.JenniferEventExternalCall || ev.StartOffsetMs == nil {
			continue
		}
		if *ev.StartOffsetMs >= *prep.StartOffsetMs && *ev.StartOffsetMs <= *prep.EndOffsetMs {
			out = append(out, i)
		}
	}
	return out
}

func nextRealEventIndex(events []models.JenniferProfileEvent, idx int) int {
	for j := idx + 1; j < len(events); j++ {
		t := events[j].EventType
		if t == models.JenniferEventStart || t == models.JenniferEventEnd || t == models.JenniferEventTotal {
			continue
		}
		return j
	}
	return -1
}

// intToString is a tiny strconv shim to keep validateNetworkPrep
// independent of the rest of the file's helpers.
func intToString(v int) string {
	if v == 0 {
		return "0"
	}
	negative := false
	if v < 0 {
		negative = true
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

// eventCategoryRule binds a target event-type to a list of
// case-insensitive substrings.
type eventCategoryRule struct {
	target   models.JenniferEventType
	patterns []string
}

func normalizeEventPatterns(in map[string][]string) []eventCategoryRule {
	if len(in) == 0 {
		return nil
	}
	out := make([]eventCategoryRule, 0, len(in))
	for cat, patterns := range in {
		cat = strings.TrimSpace(cat)
		if cat == "" {
			continue
		}
		var lowered []string
		for _, p := range patterns {
			p = strings.ToLower(strings.TrimSpace(p))
			if p != "" {
				lowered = append(lowered, p)
			}
		}
		if len(lowered) == 0 {
			continue
		}
		out = append(out, eventCategoryRule{
			target:   models.JenniferEventType(cat),
			patterns: lowered,
		})
	}
	return out
}

func matchAnyLower(haystack string, needles []string) bool {
	for _, n := range needles {
		if strings.Contains(haystack, n) {
			return true
		}
	}
	return false
}

func classifyOne(ev *models.JenniferProfileEvent) models.JenniferEventType {
	msg := ev.RawMessage
	upper := strings.ToUpper(strings.TrimSpace(msg))

	switch upper {
	case "START":
		return models.JenniferEventStart
	case "END":
		return models.JenniferEventEnd
	case "TOTAL":
		return models.JenniferEventTotal
	}

	if strings.HasPrefix(upper, "EXTERNAL_CALL") || strings.HasPrefix(upper, "EXTERNAL-CALL") {
		return models.JenniferEventExternalCall
	}
	if strings.HasPrefix(upper, "FETCH") {
		return models.JenniferEventFetch
	}

	// SQL family — must inspect detail-lines first to detect Check
	// Query and 2PC/XA before defaulting to plain SQL_EXECUTE.
	if strings.HasPrefix(upper, "SQL-EXECUTE") || strings.HasPrefix(upper, "SQL_EXECUTE") {
		joined := strings.ToLower(strings.Join(ev.DetailLines, " "))
		joined = collapseSpaces(joined)

		// Check query: dialed-in well-known forms only.
		for q := range checkQueryNormalized {
			if joined == q {
				return models.JenniferEventCheckQuery
			}
		}
		// 2PC / XA: inspect more specific suffix first.
		if twoPCStartRE.MatchString(joined) {
			return models.JenniferEventTwoPCStart
		}
		if twoPCEndRE.MatchString(joined) {
			return models.JenniferEventTwoPCEnd
		}
		if twoPCPrepareRE.MatchString(joined) {
			return models.JenniferEventTwoPCPrepare
		}
		if twoPCCommitRE.MatchString(joined) {
			return models.JenniferEventTwoPCCommit
		}
		if twoPCRollbackRE.MatchString(joined) {
			return models.JenniferEventTwoPCRollback
		}
		if twoPCAnyRE.MatchString(joined) {
			return models.JenniferEventTwoPCUnknown
		}
		// JAVA_XA wrapper (Atomikos / WebLogic XA / Spring JtaTransactionManager
		// etc.) — the SQL body carries the marker even when the actual
		// xa_start / xa_commit calls are emitted as plain method
		// events. Counting the wrapping SQL_EXECUTE keeps the 2PC
		// ledger aligned with the user's mental model: "every line
		// containing JAVA_XA is a 2PC step".
		if twoPCJavaRE.MatchString(joined) {
			return models.JenniferEventTwoPCUnknown
		}
		// Default SQL classifier — distinguish UPDATE / QUERY / GENERIC.
		switch {
		case strings.HasPrefix(upper, "SQL-EXECUTE-UPDATE"), strings.HasPrefix(upper, "SQL_EXECUTE_UPDATE"):
			return models.JenniferEventSQLUpdate
		case strings.HasPrefix(upper, "SQL-EXECUTE-QUERY"), strings.HasPrefix(upper, "SQL_EXECUTE_QUERY"):
			return models.JenniferEventSQLQuery
		default:
			return models.JenniferEventSQLExecute
		}
	}

	// JAVA_XA can also appear at the message level rather than only
	// inside a SQL body — many tracing libraries surface it as a
	// pseudo-method name. Recognising it here means the 2PC ledger
	// catches both shapes without relying on the SQL fall-through.
	if twoPCJavaRE.MatchString(msg) {
		return models.JenniferEventTwoPCUnknown
	}

	// Connection acquire — the message itself usually carries a
	// JDBC method signature (`org.apache.jsp.getConnection() …`).
	for _, re := range connectionAcquireREs {
		if re.MatchString(msg) {
			return models.JenniferEventConnAcquire
		}
	}

	if socketRE.MatchString(upper) {
		return models.JenniferEventSocket
	}
	if externalCallInfoRE.MatchString(msg) {
		return models.JenniferEventExternalCallInfo
	}

	// Anything left that looks like a Java method signature (has `(`
	// and `)` and a return-type / package prefix) gets classified as
	// METHOD; the rest fall through to UNKNOWN.
	if strings.Contains(msg, "(") && strings.Contains(msg, ")") {
		return models.JenniferEventMethod
	}
	return models.JenniferEventUnknown
}

// fillExternalCall lifts protocol / client / url out of an
// `EXTERNAL_CALL [HTTP] APACHE_HTTP_CLIENT_V5 ( url=…)` message.
func fillExternalCall(ev *models.JenniferProfileEvent) {
	m := externalCallRE.FindStringSubmatch(ev.RawMessage)
	if m == nil {
		return
	}
	ev.ExternalProtocol = strings.TrimSpace(m[1])
	ev.ExternalClient = strings.TrimSpace(m[2])
	ev.ExternalURL = strings.TrimSpace(m[3])
}

// fillFetch lifts current/cumulative row counts out of a FETCH
// message. The message-level elapsed parser already populated
// `ElapsedMs`, but we re-parse here in case the trailing-elapsed
// regex matched a different ms (it won't, FETCH has only one
// `[NNN ms]` token but defensive parsing is cheap).
func fillFetch(ev *models.JenniferProfileEvent) {
	m := fetchRE.FindStringSubmatch(ev.RawMessage)
	if m == nil {
		return
	}
	cur, _ := strconv.Atoi(strings.ReplaceAll(m[1], ",", ""))
	cum, _ := strconv.Atoi(strings.ReplaceAll(m[2], ",", ""))
	elapsed, _ := strconv.Atoi(strings.ReplaceAll(m[3], ",", ""))
	ev.CurrentFetchRows = &cur
	ev.CumulativeFetchRows = &cum
	if ev.ElapsedMs == nil {
		ev.ElapsedMs = &elapsed
	}
}

func collapseSpaces(s string) string {
	s = strings.TrimSpace(s)
	// Drop trailing semicolon to mirror §11.3 normalisation.
	s = strings.TrimRight(s, ";")
	// Collapse runs of whitespace.
	out := strings.Builder{}
	out.Grow(len(s))
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if !prevSpace {
				out.WriteByte(' ')
			}
			prevSpace = true
			continue
		}
		out.WriteRune(r)
		prevSpace = false
	}
	return strings.TrimSpace(out.String())
}
