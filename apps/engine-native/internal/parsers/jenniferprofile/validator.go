package jenniferprofile

import (
	"strconv"
	"strings"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

// validateFullProfile applies the §10 FULL-Profile rules. Errors are
// appended to `profile.Errors`; STRICT_MODE callers (the default)
// will then mark the surrounding GUID group as failed.
func validateFullProfile(profile *models.JenniferTransactionProfile, opts Options) {
	add := func(code, msg string) {
		profile.Errors = append(profile.Errors, models.JenniferProfileIssue{Code: code, Message: msg})
	}
	if profile.Header.TXID == "" {
		add("MISSING_TXID", "TXID is empty or absent")
	}
	if profile.Header.GUID == "" && !opts.FallbackCorrelationToTxid {
		add("MISSING_GUID", "GUID is empty or absent")
	}
	if profile.Header.Application == "" {
		add("MISSING_APPLICATION", "APPLICATION is empty or absent")
	}
	if profile.Header.ResponseTimeMs == nil {
		add("MISSING_RESPONSE_TIME", "RESPONSE_TIME is empty or absent")
	}
	if !profile.Body.HasBodyHeader {
		add("MISSING_BODY_HEADER", "Body table header (`[ No.][ START_TIME ][ GAP ][CPU_T]`) not found")
	}
	if !profile.Body.HasStart {
		add("MISSING_PROFILE_START", "Body START event not found")
	}
	if !profile.Body.HasEnd {
		add("MISSING_PROFILE_END", "Body END event not found")
	}
	if !profile.Body.HasTotal {
		add("MISSING_TOTAL", "Body TOTAL line not found")
	}
}

// calculateOffsets computes startOffsetMs / endOffsetMs for every
// body event relative to the body START time. Per §16.3-§16.4. We
// only fill offsets when both START and the event time parse cleanly;
// otherwise the field stays nil so JSON renders null.
func calculateOffsets(profile *models.JenniferTransactionProfile) {
	startMs, ok := parseHHMMSSms(profile.Body.BodyStartTime)
	if !ok {
		return
	}
	for i := range profile.Body.Events {
		ev := &profile.Body.Events[i]
		evStart, ok := parseHHMMSSms(ev.EventStart)
		if !ok {
			continue
		}
		offset := evStart - startMs
		ev.StartOffsetMs = &offset
		if ev.ElapsedMs != nil {
			end := offset + *ev.ElapsedMs
			ev.EndOffsetMs = &end
		}
	}
}

// parseHHMMSSms turns `16:10:52 608` into total milliseconds since
// midnight. Returns ok=false on any structural issue.
func parseHHMMSSms(value string) (int, bool) {
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
	h, err := strconv.Atoi(hms[0])
	if err != nil {
		return 0, false
	}
	m, err := strconv.Atoi(hms[1])
	if err != nil {
		return 0, false
	}
	s, err := strconv.Atoi(hms[2])
	if err != nil {
		return 0, false
	}
	ms, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, false
	}
	return ((h*60+m)*60+s)*1000 + ms, true
}
