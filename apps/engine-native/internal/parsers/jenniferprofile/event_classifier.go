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
// wrapper methods. The user can extend or replace this via Options.
// Matched on the message text (case-insensitive substring).
var defaultNetworkPrepPatterns = []string{
	"integrationutil.sendtoservice",
	"sendtoservice(",
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
	prepPatterns := opts.NetworkPrepPatterns
	if len(prepPatterns) == 0 {
		prepPatterns = defaultNetworkPrepPatterns
	}
	prepLower := make([]string, 0, len(prepPatterns))
	for _, p := range prepPatterns {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			prepLower = append(prepLower, p)
		}
	}
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
	validateNetworkPrep(profile)
}

// validateNetworkPrep enforces the user-stated invariant: a method
// classified as NETWORK_PREP_METHOD must be immediately followed by
// an EXTERNAL_CALL event in the body. If not, the method was a
// false positive (the wrapper name happens to appear in non-network
// code paths) and we demote it back to METHOD so it doesn't pollute
// the network_prep ledger.
//
// We also surface a per-profile warning when the wrapper-method
// elapsed differs significantly from the following EXTERNAL_CALL
// elapsed — that's the "이상한" signal the user wanted: either the
// wrapper has heavy non-network code OR the call took longer than
// the wrapper recorded (clock drift / overlapping events).
func validateNetworkPrep(profile *models.JenniferTransactionProfile) {
	events := profile.Body.Events
	for i := range events {
		ev := &events[i]
		if ev.EventType != models.JenniferEventNetworkPrep {
			continue
		}
		// Skip START / END / continuation rows when looking for the
		// "next real event". The classifier already collapsed those
		// into recognised types so we only need to walk forward.
		nextIdx := -1
		for j := i + 1; j < len(events); j++ {
			t := events[j].EventType
			if t == models.JenniferEventStart || t == models.JenniferEventEnd || t == models.JenniferEventTotal {
				continue
			}
			nextIdx = j
			break
		}
		if nextIdx < 0 || events[nextIdx].EventType != models.JenniferEventExternalCall {
			ev.EventType = models.JenniferEventMethod
			profile.Warnings = append(profile.Warnings, models.JenniferProfileIssue{
				Code:    "NETWORK_PREP_NOT_FOLLOWED_BY_EXTERNAL_CALL",
				Message: "NETWORK_PREP wrapper method demoted: next event was not EXTERNAL_CALL",
			})
			continue
		}
		// Suspicious-elapsed warning: wrapper elapsed should be ≥
		// next EXTERNAL_CALL elapsed (wrapper enclosing the call).
		// If the EXTERNAL_CALL is much larger, the wrapper isn't
		// actually wrapping it and the prep math will under-report.
		// If wrapper is much larger than the call, there's hidden
		// work inside the wrapper that's not actually network-prep.
		if ev.ElapsedMs != nil && events[nextIdx].ElapsedMs != nil {
			wrapperMs := *ev.ElapsedMs
			callMs := *events[nextIdx].ElapsedMs
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
					Message: "NETWORK_PREP wrapper elapsed differs from following EXTERNAL_CALL by " +
						intToString(diff) + " ms (wrapper=" + intToString(wrapperMs) + " call=" + intToString(callMs) + ")",
				})
			}
		}
	}
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
