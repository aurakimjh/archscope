// Package timeutil ports archscope_engine.common.time_utils — the
// shared nginx timestamp parser and the per-minute bucket key the
// access-log / GC-log / OTel analyzers all use as their time-series
// index.
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
