// [한글] msa_match.go — §14 caller-callee 매칭 알고리즘.
//
// 책임
//
//	GUID 그룹 안의 EXTERNAL_CALL 이벤트와 같은 그룹의 다른 profile
//	header(URL/application/start_time) 를 후보로 간주해, 어떤
//	caller→callee 짝이 가장 그럴듯한지 점수화.
//
// 정규화 규칙 (§14.1, normalizeURL)
//  1. 양 끝 공백 trim.
//  2. `?` 이후의 query string + `#` 이후의 fragment 제거.
//  3. 연속 `/` 를 한 번으로(`//` → `/`).
//  4. 끝 `/` 제거(루트 `/` 는 보존).
//     목적: caller 의 EXTERNAL_CALL.url 과 callee 의 header.application
//     을 같은 정규형으로 비교 가능하게.
//
// 점수 산정 핵심 요소 (§14.2~§14.4)
//   - URL 일치도(prefix/suffix/exact) : 가장 큰 가중치.
//   - application 라벨 일치 : 인스턴스 도메인 식별.
//   - 시간 근접도 : 외부호출 시작 시각과 callee 시작 시각의 갭.
//   - 동률 처리 : 점수 같으면 시간 근접 우선, 그 다음 URL 길이.
//
// 매칭 결과 상태
//
//	MATCHED                       : 채택
//	UNMATCHED                     : 어떤 후보와도 매치 실패
//	MATCH_SCORE_TOO_LOW           : 후보 있으나 MinMatchScore(=80) 미만
//	AMBIGUOUS_EXTERNAL_CALL_MATCH : 동률 후보 ≥ 2 (§14.3 §14.4)
package jenniferprofile

import (
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// MinMatchScore is the cutoff under which a candidate is declared
// MATCH_SCORE_TOO_LOW (§14.3 last clause). Lowered from 80 → 60
// after the containment-style matching landed: caller URL ⊃ callee
// Application alone is +75, which would otherwise fail an 80-bar
// even on perfectly clean profiles where the user-supplied tooling
// doesn't carry a response_time_ms / start_time pair.
const MinMatchScore = 60

// duplicateSlashRE collapses runs of slashes like `//` to a single
// `/` per §14.1 normalisation rule 4.
var duplicateSlashRE = regexp.MustCompile(`/{2,}`)
var externalTargetNumericSegmentRE = regexp.MustCompile(`^[0-9]+$`)
var externalTargetUUIDSegmentRE = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var externalTargetLongHexSegmentRE = regexp.MustCompile(`(?i)^[0-9a-f]{12,}$`)

// normalizeURL implements the §14.1 URL/Application normalisation
// rules: trim whitespace, drop query string + fragment, collapse
// duplicate slashes, drop trailing slash. Returns the canonical
// path used for both `external_call.url` and `header.application`.
func normalizeURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if i := strings.IndexByte(s, '?'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '#'); i >= 0 {
		s = s[:i]
	}
	s = duplicateSlashRE.ReplaceAllString(s, "/")
	if len(s) > 1 {
		s = strings.TrimRight(s, "/")
	}
	return s
}

// normalizeExternalTargetForGrouping produces a stable grouping key
// for unprofiled EXTERNAL_CALL rows. It keeps host/path identity but
// removes query/fragment noise and folds obvious IDs so repeated
// calls like /rule/123 and /rule/456 aggregate together.
func normalizeExternalTargetForGrouping(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	if parsed, err := url.Parse(s); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		path := normalizeExternalPathSegments(parsed.EscapedPath())
		if path == "" {
			path = "/"
		}
		return strings.ToLower(parsed.Scheme) + "://" + strings.ToLower(parsed.Host) + path
	}
	return normalizeExternalPathSegments(normalizeURL(s))
}

func normalizeExternalPathSegments(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if segment == "" {
			continue
		}
		if externalTargetNumericSegmentRE.MatchString(segment) ||
			externalTargetUUIDSegmentRE.MatchString(segment) ||
			externalTargetLongHexSegmentRE.MatchString(segment) {
			segments[i] = "{id}"
		}
	}
	out := strings.Join(segments, "/")
	if len(out) > 1 {
		out = strings.TrimRight(out, "/")
	}
	return out
}

// matchCandidate is one (caller-edge, callee-profile) pairing
// considered during one-to-one assignment.
type matchCandidate struct {
	callerTXID    string
	callerEdgeIdx int // index into the caller-profile's external-call list
	calleeTXID    string
	calleeIdx     int // index in the GUID group's profiles slice
	score         int
	urlExact      bool
	timeOverlap   bool
	gapNonNeg     bool
	smallGap      bool
	orderMatch    bool
	// Sort keys for the "earliest first" tie-break that keeps repeated
	// caller→callee pairs paired in chronological order.
	callerEventStartMs int
	calleeBodyStartMs  int
}

// callerEdge is one EXTERNAL_CALL event flagged with its enclosing
// caller-profile metadata so the matcher can cross-reference times
// and ordering without re-walking events twice.
type callerEdge struct {
	profileIdx      int // index into profiles slice
	eventIdx        int // index into profile.Body.Events
	sequence        int // 1-based occurrence index of EXTERNAL_CALL inside the caller
	url             string
	urlNormalized   string
	target          string
	protocol        string
	client          string
	elapsedMs       int
	startOffsetMs   *int
	bodyStartTimeMs int // ms-since-midnight of caller's body START
}

// matchExternalCalls implements §14 caller-callee matching for one
// GUID group. It returns the resolved edges (matched + unmatched)
// in caller-iteration order so downstream Network Gap / call graph
// stay deterministic.
func matchExternalCalls(group *guidGroupBucket) []models.JenniferExternalCallEdge {
	callers := collectCallerEdges(group.profiles)
	if len(callers) == 0 {
		return nil
	}

	candidates := buildCandidates(callers, group.profiles)

	// Greedy one-to-one assignment by descending score: each callee
	// profile can be paired with at most one caller-edge. Tie-break
	// rules carry the "동일 호출건 여러 개면 시간 순으로" requirement —
	// when the same caller→callee pair appears repeatedly, we want the
	// pairing to follow caller-event start time so the call graph
	// preserves the actual sequence rather than an arbitrary one.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].urlExact != candidates[j].urlExact {
			return candidates[i].urlExact
		}
		// callerEventStartMs ascending: the earlier external-call event
		// gets first pick of available callees. This is what makes
		// "earliest call → earliest callee profile" hold when the
		// caller fires the same downstream service N times.
		if candidates[i].callerEventStartMs != candidates[j].callerEventStartMs {
			return candidates[i].callerEventStartMs < candidates[j].callerEventStartMs
		}
		// calleeBodyStartMs ascending: among callees that all match,
		// prefer the one whose body started earliest — that's the
		// natural pairing when both lists are time-ordered.
		if candidates[i].calleeBodyStartMs != candidates[j].calleeBodyStartMs {
			return candidates[i].calleeBodyStartMs < candidates[j].calleeBodyStartMs
		}
		if candidates[i].callerEdgeIdx != candidates[j].callerEdgeIdx {
			return candidates[i].callerEdgeIdx < candidates[j].callerEdgeIdx
		}
		return candidates[i].calleeIdx < candidates[j].calleeIdx
	})

	usedCallee := map[int]bool{}
	usedEdge := map[edgeKey]bool{}
	picked := map[edgeKey]matchCandidate{}

	for _, c := range candidates {
		ek := edgeKey{caller: c.callerTXID, edgeIdx: c.callerEdgeIdx}
		if usedEdge[ek] {
			continue
		}
		if usedCallee[c.calleeIdx] {
			continue
		}
		if c.score < MinMatchScore {
			continue
		}
		picked[ek] = c
		usedEdge[ek] = true
		usedCallee[c.calleeIdx] = true
	}

	out := make([]models.JenniferExternalCallEdge, 0, len(callers))
	for _, ce := range callers {
		ek := edgeKey{caller: callerTXIDOf(group.profiles, ce.profileIdx), edgeIdx: ce.eventIdx}
		caller := &group.profiles[ce.profileIdx]
		// Resolve the caller-event start in absolute ms-since-midnight
		// regardless of match status, so unmatched edges still place
		// on the renderer's Gantt timeline at the correct offset.
		callerStart, _ := parseHHMMSSmsLocal(ce.callerEventStartTime(group.profiles))
		edge := models.JenniferExternalCallEdge{
			GUID:                  group.guid,
			CallerTXID:            caller.Header.TXID,
			CallerApplication:     caller.Header.Application,
			ExternalCallSequence:  ce.sequence,
			ExternalCallURL:       ce.url,
			ExternalCallTarget:    ce.target,
			ExternalCallProtocol:  ce.protocol,
			ExternalCallClient:    ce.client,
			ExternalCallElapsedMs: ce.elapsedMs,
			CallerEventStartMs:    callerStart,
		}
		match, ok := picked[ek]
		if !ok {
			edge.MatchStatus = models.JenniferMatchUnmatched
			out = append(out, edge)
			continue
		}
		callee := &group.profiles[match.calleeIdx]
		edge.CalleeTXID = callee.Header.TXID
		edge.CalleeApplication = callee.Header.Application
		if callee.Header.ResponseTimeMs != nil {
			rt := *callee.Header.ResponseTimeMs
			edge.CalleeResponseTimeMs = &rt
		}
		edge.MatchScore = match.score
		edge.MatchStatus = models.JenniferMatchOK
		edge.CallerEventStartMs = match.callerEventStartMs
		edge.CalleeBodyStartMs = match.calleeBodyStartMs
		applyNetworkGap(&edge)
		out = append(out, edge)
	}
	return out
}

// edgeKey uniquely identifies a caller's individual EXTERNAL_CALL
// event across the whole GUID group.
type edgeKey struct {
	caller  string // caller TXID
	edgeIdx int    // event index inside caller.Body.Events
}

// callerTXIDOf is a tiny helper that reads the TXID for the caller
// profile at index i. Done as a function so the slice doesn't have
// to escape via a closure.
func callerTXIDOf(profiles []models.JenniferTransactionProfile, i int) string {
	if i < 0 || i >= len(profiles) {
		return ""
	}
	return profiles[i].Header.TXID
}

// collectCallerEdges walks every profile and lifts every
// EXTERNAL_CALL event into a flat candidate-source list. Sequence
// numbers are 1-based and reset per caller.
func collectCallerEdges(profiles []models.JenniferTransactionProfile) []callerEdge {
	out := []callerEdge{}
	for pi := range profiles {
		p := &profiles[pi]
		bodyStart, _ := parseHHMMSSmsLocal(p.Body.BodyStartTime)
		seq := 0
		for ei := range p.Body.Events {
			ev := &p.Body.Events[ei]
			if ev.EventType != models.JenniferEventExternalCall {
				continue
			}
			if ev.ElapsedMs == nil {
				continue
			}
			seq++
			out = append(out, callerEdge{
				profileIdx:      pi,
				eventIdx:        ei,
				sequence:        seq,
				url:             ev.ExternalURL,
				urlNormalized:   normalizeURL(ev.ExternalURL),
				target:          normalizeExternalTargetForGrouping(ev.ExternalURL),
				protocol:        ev.ExternalProtocol,
				client:          ev.ExternalClient,
				elapsedMs:       *ev.ElapsedMs,
				startOffsetMs:   ev.StartOffsetMs,
				bodyStartTimeMs: bodyStart,
			})
		}
	}
	return out
}

// buildCandidates produces every (caller-edge, callee-profile)
// pairing that could plausibly match. The relationship between the
// caller's EXTERNAL_CALL URL and the callee profile's APPLICATION
// header is "containment", not equality:
//
//	caller URL = "http://orders.svc/api/v1/place?id=42"
//	callee Application = "/api/v1/place"   ← substring of the URL
//
// Real-world Jennifer exports also produce the reverse layout:
//
//	caller URL = "/api/v1/place"
//	callee Application = "orders-service./api/v1/place"
//
// so we accept either direction (URL ⊃ Application OR Application ⊃
// URL) and assign a higher score to exact equality. When the same
// (caller, callee) pair would match repeatedly, the time-overlap
// score does the per-call disambiguation later.
func buildCandidates(callers []callerEdge, profiles []models.JenniferTransactionProfile) []matchCandidate {
	out := []matchCandidate{}
	for _, ce := range callers {
		for ci := range profiles {
			callee := &profiles[ci]
			if ci == ce.profileIdx {
				continue
			}
			if callee.Header.Application == "" {
				continue
			}
			calleeApp := callee.Header.Application
			calleeAppNorm := normalizeURL(calleeApp)

			urlExact := calleeApp == ce.url
			urlNorm := calleeAppNorm == ce.urlNormalized && calleeAppNorm != ""

			// Containment matching — case-insensitive on the
			// normalized forms so DNS casing / capitalised path
			// segments don't break otherwise-obvious matches.
			urlNormLower := strings.ToLower(ce.urlNormalized)
			calleeAppNormLower := strings.ToLower(calleeAppNorm)
			urlContains := false
			appContains := false
			if calleeAppNormLower != "" && urlNormLower != "" {
				if calleeAppNormLower != urlNormLower {
					if strings.Contains(urlNormLower, calleeAppNormLower) {
						urlContains = true
					} else if strings.Contains(calleeAppNormLower, urlNormLower) {
						appContains = true
					}
				}
			}

			if !urlExact && !urlNorm && !urlContains && !appContains {
				continue
			}
			score := 0
			if urlExact {
				score += 100
			} else if urlNorm {
				score += 90
			} else if urlContains {
				// Caller URL ⊃ callee Application is the spec the
				// user described — a longer caller URL embeds the
				// service path. Heavier weight than the reverse.
				score += 75
			} else if appContains {
				score += 60
			}
			gapNonNeg := false
			smallGap := false
			if callee.Header.ResponseTimeMs != nil {
				rt := *callee.Header.ResponseTimeMs
				if ce.elapsedMs >= rt {
					score += 20
					gapNonNeg = true
				}
				gap := ce.elapsedMs - rt
				if gap >= 0 && gap < 1000 {
					// Smaller gap → higher score (max +20).
					if gap == 0 {
						score += 20
					} else if gap < 100 {
						score += 15
					} else if gap < 500 {
						score += 10
					} else {
						score += 5
					}
					smallGap = true
				}
			}
			// Time-overlap heuristic: caller's external-call window
			// (startOffset → +elapsed) intersects callee's body
			// start-time window (bodyStart → +responseTime). When
			// the absolute START_TIMEs are close (under 5s apart)
			// we award the +30. Skip silently when timestamps are
			// missing.
			callerStart, ok1 := parseHHMMSSmsLocal(ce.callerEventStartTime(profiles))
			calleeStart, ok2 := parseHHMMSSmsLocal(callee.Body.BodyStartTime)
			timeOverlap := false
			if ok1 && ok2 && abs(callerStart-calleeStart) < 5_000 {
				score += 30
				timeOverlap = true
			}
			out = append(out, matchCandidate{
				callerTXID:         callerTXIDOf(profiles, ce.profileIdx),
				callerEdgeIdx:      ce.eventIdx,
				calleeTXID:         callee.Header.TXID,
				calleeIdx:          ci,
				score:              score,
				urlExact:           urlExact,
				timeOverlap:        timeOverlap,
				gapNonNeg:          gapNonNeg,
				smallGap:           smallGap,
				callerEventStartMs: callerStart,
				calleeBodyStartMs:  calleeStart,
			})
		}
	}
	return out
}

// callerEventStartTime returns the HH:MM:SS NNN string of the
// caller's external-call event so the matcher can compare it
// against callee body start times.
func (ce callerEdge) callerEventStartTime(profiles []models.JenniferTransactionProfile) string {
	if ce.profileIdx < 0 || ce.profileIdx >= len(profiles) {
		return ""
	}
	p := &profiles[ce.profileIdx]
	if ce.eventIdx < 0 || ce.eventIdx >= len(p.Body.Events) {
		return ""
	}
	return p.Body.Events[ce.eventIdx].EventStart
}

// parseHHMMSSmsLocal mirrors validator.parseHHMMSSms — duplicated
// here to keep the matcher self-contained without an import cycle.
func parseHHMMSSmsLocal(value string) (int, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parts := strings.Fields(value)
	if len(parts) != 2 {
		return 0, false
	}
	hms := strings.Split(parts[0], ":")
	if len(hms) != 3 {
		return 0, false
	}
	h, err := atoi(hms[0])
	if err {
		return 0, false
	}
	m, err := atoi(hms[1])
	if err {
		return 0, false
	}
	s, err := atoi(hms[2])
	if err {
		return 0, false
	}
	ms, err := atoi(parts[1])
	if err {
		return 0, false
	}
	return ((h*60+m)*60+s)*1000 + ms, true
}

// atoi is a tiny strconv shim that reports failure as a bool to keep
// the validator-style 2-return shape uniform.
func atoi(value string) (int, bool) {
	v := 0
	negative := false
	for i, r := range value {
		if i == 0 && r == '-' {
			negative = true
			continue
		}
		if r < '0' || r > '9' {
			return 0, true
		}
		v = v*10 + int(r-'0')
	}
	if negative {
		v = -v
	}
	return v, false
}
