package accesslog

import "testing"

func TestDurationAvailabilityKeepsMeasuredZero(t *testing.T) {
	for _, raw := range []any{0, float64(0), "0", "0.000s", "0ms", "1.5s", "100us"} {
		if !durationAvailable(raw) {
			t.Fatal(raw)
		}
	}
	for _, raw := range []any{nil, "", "-", "-1", "NaN", "Inf", "invalid", "-1ms", true} {
		if durationAvailable(raw) {
			t.Fatal(raw)
		}
	}
}
func TestJSONDurationPresenceAndZero(t *testing.T) {
	for _, suffix := range []string{"", `,"duration":null`, `,"duration":"-"`, `,"duration":"invalid"`, `,"duration":0`} {
		row, err := parseCaddyJSONLine(`{"ts":1700000000,"status":200,"request":{"method":"GET","uri":"/synthetic"}`+suffix+`}`, "caddy-json")
		if err != nil {
			t.Fatal(err)
		}
		if row.ResponseTimeUnavailable != (suffix != `,"duration":0`) {
			t.Fatal(suffix, row.ResponseTimeUnavailable)
		}
	}
}
func TestIISWithoutTimeTakenStillCountsRequest(t *testing.T) {
	row, err := parseIISW3CLine("2026-11-10 10:00:00 GET /synthetic 200", []string{"date", "time", "cs-method", "cs-uri-stem", "sc-status"})
	if err != nil || !row.ResponseTimeUnavailable {
		t.Fatal(row, err)
	}
}
