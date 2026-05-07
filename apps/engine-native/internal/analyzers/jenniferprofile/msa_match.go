package jenniferprofile

import (
	"regexp"
	"sort"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// MinMatchScore is the cutoff under which a candidate is declared
// MATCH_SCORE_TOO_LOW (§14.3 last clause). Tunable via Options.
const MinMatchScore = 80

// duplicateSlashRE collapses runs of slashes like `//` to a single
// `/` per §14.1 normalisation rule 4.
var duplicateSlashRE = regexp.MustCompile(`/{2,}`)

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

// matchCandidate is one (caller-edge, callee-profile) pairing
// considered during one-to-one assignment.
type matchCandidate struct {
	callerTXID    string
	callerEdgeIdx int          // index into the caller-profile's external-call list
	calleeTXID    string
	calleeIdx     int          // index in the GUID group's profiles slice
	score         int
	urlExact      bool
	timeOverlap   bool
	gapNonNeg     bool
	smallGap      bool
	orderMatch    bool
}

// callerEdge is one EXTERNAL_CALL event flagged with its enclosing
// caller-profile metadata so the matcher can cross-reference times
// and ordering without re-walking events twice.
type callerEdge struct {
	profileIdx       int // index into profiles slice
	eventIdx         int // index into profile.Body.Events
	sequence         int // 1-based occurrence index of EXTERNAL_CALL inside the caller
	url              string
	urlNormalized    string
	elapsedMs        int
	startOffsetMs    *int
	bodyStartTimeMs  int // ms-since-midnight of caller's body START
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
	// profile can be paired with at most one caller-edge. This is
	// good enough for MVP2; the spec recommends Hungarian for
	// optimal but greedy matches the "obvious" cases the doc lists
	// as long as scores diverge cleanly.
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		// Stable tiebreaker: prefer URL-exact, then earlier caller seq.
		if candidates[i].urlExact != candidates[j].urlExact {
			return candidates[i].urlExact
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
		edge := models.JenniferExternalCallEdge{
			GUID:                  group.guid,
			CallerTXID:            caller.Header.TXID,
			CallerApplication:     caller.Header.Application,
			ExternalCallSequence:  ce.sequence,
			ExternalCallURL:       ce.url,
			ExternalCallElapsedMs: ce.elapsedMs,
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
				elapsedMs:       *ev.ElapsedMs,
				startOffsetMs:   ev.StartOffsetMs,
				bodyStartTimeMs: bodyStart,
			})
		}
	}
	return out
}

// buildCandidates produces every (caller-edge, callee-profile)
// pairing that could plausibly match — i.e. URL prefixes match and
// the callee profile is not the caller itself. Score is computed
// here per §14.3.
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
			urlExact := callee.Header.Application == ce.url
			normalisedCallee := normalizeURL(callee.Header.Application)
			urlNorm := normalisedCallee == ce.urlNormalized
			if !urlExact && !urlNorm {
				continue
			}
			score := 0
			if urlExact {
				score += 100
			} else if urlNorm {
				score += 80
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
				callerTXID:    callerTXIDOf(profiles, ce.profileIdx),
				callerEdgeIdx: ce.eventIdx,
				calleeTXID:    callee.Header.TXID,
				calleeIdx:     ci,
				score:         score,
				urlExact:      urlExact,
				timeOverlap:   timeOverlap,
				gapNonNeg:     gapNonNeg,
				smallGap:      smallGap,
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
