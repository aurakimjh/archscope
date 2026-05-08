// [한글] validator.go — §10 FULL-Profile 검증기.
//
// 책임
//   각 transaction profile 의 강제 필드 누락을 검출해 Errors/Warnings
//   에 추가. STRICT_MODE (기본값) 에서는 Errors 가 1개라도 있으면
//   분석기가 그 profile 을 "GROUP_FAILED" 로 표시.
//
// 검사 항목
//   MISSING_TXID         : TXID 누락.
//   MISSING_GUID         : GUID 누락 (FallbackCorrelationToTxid=false 시).
//   MISSING_APPLICATION  : Application 누락.
//   MISSING_START_TIME   : 헤더의 START_TIME 누락.
//   MISSING_END_TIME     : 헤더의 END_TIME 누락.
//   MISSING_BODY_HEADER  : `[No.][START_TIME][GAP][CPU_T]` 라인 없음.
//   MISSING_BODY_START   : 본문에 START 이벤트 없음.
//   MISSING_BODY_END     : 본문에 END 이벤트 없음 (잘린 export 신호).
//   MISSING_BODY_TOTAL   : TOTAL 라인 없음.
//   HEADER_BODY_MISMATCH : 헤더의 사전 집계와 본문 합 차이 > tolerance.
//
// 비-치명적 warning
//   • NEGATIVE_NETWORK_GAP_ADJUSTED : §16.2 음수 갭 조정.
//   • PARSE_AMBIGUITY_*             : 분류기에서 확신 없는 경우.
//
// FallbackCorrelationToTxid
//   true 이면 GUID 누락이 fatal 이 아닌 (TXID 로 대체) — 구버전 export
//   호환용.
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
