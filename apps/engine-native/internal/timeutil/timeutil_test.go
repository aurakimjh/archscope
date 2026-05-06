package timeutil

import (
	"testing"
	"time"
)

func TestParseNginxTimestamp(t *testing.T) {
	got, err := ParseNginxTimestamp("27/Apr/2026:10:00:01 +0900")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Year() != 2026 || got.Month() != time.April || got.Day() != 27 {
		t.Fatalf("date wrong: %s", got)
	}
	if got.Hour() != 10 || got.Minute() != 0 || got.Second() != 1 {
		t.Fatalf("time wrong: %s", got)
	}
	if _, offset := got.Zone(); offset != 9*3600 {
		t.Fatalf("offset wrong: %d", offset)
	}
}

func TestParseNginxTimestampInvalid(t *testing.T) {
	if _, err := ParseNginxTimestamp("bad-time"); err == nil {
		t.Fatalf("expected error for malformed input")
	}
}

func TestMinuteBucketPythonParity(t *testing.T) {
	ts, err := ParseNginxTimestamp("27/Apr/2026:10:00:01 +0900")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	const want = "2026-04-27T10:00:00+0900"
	if got := MinuteBucket(ts); got != want {
		t.Fatalf("MinuteBucket = %q, want %q", got, want)
	}
}

func TestMinuteBucketHonorsLocation(t *testing.T) {
	ts, err := ParseNginxTimestamp("27/Apr/2026:10:00:59 -0500")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	const want = "2026-04-27T10:00:00-0500"
	if got := MinuteBucket(ts); got != want {
		t.Fatalf("MinuteBucket = %q, want %q", got, want)
	}
}
