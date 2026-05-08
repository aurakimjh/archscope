// Package timeutil ports archscope_engine.common.time_utils — the
// shared nginx timestamp parser and the per-minute bucket key the
// access-log / GC-log / OTel analyzers all use as their time-series
// index.
//
// ─────────────────────────────────────────────────────────────────────
// [한글] timeutil 패키지 — 시계열 분석에서 공유하는 시간 도구.
//
// 두 가지 핵심 기능
//   1) ParseNginxTimestamp(s) — nginx access log 의 timestamp 파싱.
//        형식: `27/Apr/2026:10:00:01 +0900`
//        Go layout reference: `02/Jan/2006:15:04:05 -0700`.
//        timezone-aware 한 time.Time 반환.
//
//   2) MinuteBucket(t) — 분 단위 bucket key 생성.
//        형식: `2006-01-02T15:04:00-0700`
//        seconds 가 항상 "00" 으로 고정 → 같은 분의 모든 이벤트가
//        같은 키를 공유. 분당 통계 (RPS, percentile, 오류율 등) 의
//        index 키로 사용.
//
// 왜 분단위인가?
//   • 초단위 → 너무 잘게 쪼개져 dashboard 라인이 노이즈.
//   • 시간단위 → 너무 거칠어 burst 가 평균에 묻힘.
//   • 분단위 → 보고서/실시간 모니터링에서 가장 자연스러운 해상도.
//
// timezone 보존
//   `-0700` 오프셋이 키 안에 포함되므로 KST/UTC/PST 등 다른 타임존
//   로그가 섞여도 키가 충돌하지 않음. UI 측은 사용자 선호 타임존으로
//   re-render 가능 (data 는 절대 타임존 보존).
//
// Python parity 주의
//   MinuteBucketFormat 의 `-0700` 은 콜론 없는 형식 — Python `%z` 와
//   일치 (`+09:00` 이 아니라 `+0900`). 분석기 출력의 키가 byte 단위로
//   같아야 parity gate 통과.
package timeutil

import "time"

// NginxTimeFormat is `%d/%b/%Y:%H:%M:%S %z` in Go layout reference.
// e.g. `27/Apr/2026:10:00:01 +0900`.
const NginxTimeFormat = "02/Jan/2006:15:04:05 -0700"

// MinuteBucketFormat mirrors Python's
// `value.strftime("%Y-%m-%dT%H:%M:00%z")`. Seconds are hard-coded to
// `00` so all entries within the same minute share a key. The `%z`
// offset has no colon (`+0900`) — same convention as Python.
//
// `00` after `15:04:` is a literal in Go's layout reference (it does
// not collide with any layout token) and the trailing `-0700` matches
// Python's `%z`.
const MinuteBucketFormat = "2006-01-02T15:04:00-0700"

// ParseNginxTimestamp parses an nginx access-log timestamp.
// Returns the timezone-aware time.Time the caller can compare with
// other absolute times.
func ParseNginxTimestamp(value string) (time.Time, error) {
	return time.Parse(NginxTimeFormat, value)
}

// MinuteBucket returns the time-series key for value's enclosing
// minute. Equivalent to Python `minute_bucket`.
func MinuteBucket(value time.Time) string {
	return value.Format(MinuteBucketFormat)
}
