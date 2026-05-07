package jenniferprofile

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// Event-message regexes per §11.
var (
	externalCallRE = regexp.MustCompile(
		`^EXTERNAL_CALL\s+\[([^\]]+)\]\s+(\S+)\s+\(\s*url\s*=\s*([^,)]+)`,
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
	twoPCAnyRE      = regexp.MustCompile(`(?i)(?:^|[^a-z0-9_])xa_[a-z]\w*`)
)

// checkQueryNormalized are the canonical Check Query SQL bodies per §11.3.
var checkQueryNormalized = map[string]struct{}{
	"select 1 from dual":             {},
	"select 1":                       {},
	"select 1 from sysibm.sysdummy1": {},
	"select 1 from rdb$database":     {},
}

// classifyEvents walks the body events in priority order (§11.1) and
// stamps `EventType`. SQL events are inspected against their
// detail-lines first so `select 1 from dual` becomes CHECK_QUERY
// rather than SQL_EXECUTE_GENERIC.
func classifyEvents(profile *models.JenniferTransactionProfile) {
	for i := range profile.Body.Events {
		ev := &profile.Body.Events[i]
		ev.EventType = classifyOne(ev)
		if ev.EventType == models.JenniferEventExternalCall {
			fillExternalCall(ev)
		}
		if ev.EventType == models.JenniferEventFetch {
			fillFetch(ev)
		}
	}
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

	if strings.HasPrefix(upper, "EXTERNAL_CALL") {
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
