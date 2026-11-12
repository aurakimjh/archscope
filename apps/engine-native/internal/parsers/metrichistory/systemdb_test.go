package metrichistory_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	a "github.com/aurakimjh/archscope/apps/engine-native/internal/analyzers/metrichistory"
	m "github.com/aurakimjh/archscope/apps/engine-native/internal/metricmodel"
	p "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/metrichistory"
	"os"
	"strings"
	"testing"
)

func systemDBFixture(t *testing.T, name string) []byte {
	t.Helper()
	raw, e := os.ReadFile("../../catalog/contracts/system-db-" + name + "-capture.json")
	if e != nil {
		t.Fatal(e)
	}
	return raw
}
func changedCapture(t *testing.T, raw []byte, change func(map[string]any)) []byte {
	t.Helper()
	var c map[string]any
	if e := json.Unmarshal(raw, &c); e != nil {
		t.Fatal(e)
	}
	change(c)
	out, e := json.Marshal(c)
	if e != nil {
		t.Fatal(e)
	}
	return out
}
func TestActualPostgresEpochsAndExactCommonConsumers(t *testing.T) {
	raw := systemDBFixture(t, "postgres")
	set := parse(t, raw, p.Options{})
	if len(set.Series) != 15 || len(set.Points) != 75 {
		t.Fatal(len(set.Series), len(set.Points))
	}
	h := sha256.Sum256(raw)
	if set.Sources[0].SHA256 != hex.EncodeToString(h[:]) || set.Sources[0].Producer != "postgresql" {
		t.Fatal(set.Sources)
	}
	counts := map[string]int{}
	commit := []m.Interval{}
	for _, r := range rows(t, set) {
		if r.Name == "postgres.database.xact_commit" {
			commit = append(commit, r)
			counts[r.Status]++
		}
		if r.Name == "postgres.database.numbackends" && (r.Rate != nil || r.Status != "observed_gauge") {
			t.Fatal(r)
		}
	}
	if counts["start_time_unavailable"] != 1 || counts["reset"] != 1 || counts["derived"] != 2 || len(commit) != 5 {
		t.Fatal(counts)
	}
	for _, r := range commit {
		if r.Status == "start_time_unavailable" && r.Rate != nil {
			t.Fatal("unknown reset fabricated rate")
		}
		if r.Status == "derived" && (r.Delta == nil || *r.Delta != "7") {
			t.Fatal(r)
		}
	}
	result, e := a.Build(ctx, set, selected(set), "actual fixture")
	if e != nil {
		t.Fatal(e)
	}
	if e = a.Validate(result); e != nil {
		t.Fatal(e)
	}
	encoded, _ := json.Marshal(result)
	if bytes.Contains(encoded, []byte("fixture_rows")) || bytes.Contains(encoded, []byte("SELECT ")) {
		t.Fatal("SQL escaped allowlist")
	}
	// Larger-than-JS exact counts are structural, not additional actual observations.
	large := changedCapture(t, raw, func(c map[string]any) {
		for _, s := range c["snapshots"].([]any) {
			s.(map[string]any)["temp_bytes"] = "9007199254740993"
		}
	})
	big := parse(t, large, p.Options{})
	found := false
	for _, r := range rows(t, big) {
		if r.Name == "postgres.database.temp_bytes" && r.Value != nil && *r.Value == "9007199254740993" {
			found = true
		}
	}
	if !found {
		t.Fatal("rounded large integer")
	}
	// Same database after a process lifetime change must not bridge rates, even if counters survived.
	restarted := changedCapture(t, raw, func(c map[string]any) {
		snap := c["snapshots"].([]any)
		snap[4].(map[string]any)["postmaster_start_ns"] = snap[3].(map[string]any)["capture_end_ns"]
	})
	split := parse(t, restarted, p.Options{})
	if len(split.Series) != 30 {
		t.Fatal("postmaster lifetime not scoped")
	}
	for _, r := range rows(t, split) {
		if r.Name == "postgres.database.xact_commit" && r.End != nil && *r.End == *commit[4].End && r.Status != "first_cumulative_point" {
			t.Fatal(r)
		}
	}
}
func TestActualSocketSummaryScopeAndGaugeChanges(t *testing.T) {
	raw := systemDBFixture(t, "linux-ss")
	set := parse(t, raw, p.Options{})
	if len(set.Series) != 21 || len(set.Points) != 63 {
		t.Fatal(len(set.Series), len(set.Points))
	}
	observed := []string{}
	for _, r := range rows(t, set) {
		if r.Rate != nil || r.Delta != nil || r.Status != "observed_gauge" {
			t.Fatal(r)
		}
		if r.Name == "linux.ss.tcp.established" {
			observed = append(observed, *r.Value)
		}
	}
	if strings.Join(observed, ",") != "0,8,0" {
		t.Fatal(observed)
	}
	body, _ := json.Marshal(set)
	if bytes.Contains(body, []byte("127.0.0.1")) || bytes.Contains(body, []byte("\"summary\"")) {
		t.Fatal("raw endpoint/body retained")
	}
	signed := bytes.ReplaceAll(raw, []byte("closed 468"), []byte("closed -1"))
	if bytes.Equal(raw, signed) {
		signed = changedCapture(t, raw, func(c map[string]any) {
			s := c["snapshots"].([]any)[0].(map[string]any)
			start := strings.Index(s["summary"].(string), "closed ") + 7
			v := s["summary"].(string)
			end := strings.Index(v[start:], ",") + start
			s["summary"] = v[:start] + "-1" + v[end:]
		})
	}
	signedSet := parse(t, signed, p.Options{})
	hasNegative := false
	for _, r := range rows(t, signedSet) {
		if r.Value != nil && *r.Value == "-1" {
			hasNegative = true
		}
	}
	if !hasNegative {
		t.Fatal("signed producer calculation lost")
	}
	other := changedCapture(t, raw, func(c map[string]any) { c["network_namespace"] = "other-netns" })
	otherSet := parse(t, other, p.Options{})
	if set.Series[0].ID == otherSet.Series[0].ID {
		t.Fatal("namespace collision")
	}
}
func TestSnapshotCaptureRefusalsAndCancellation(t *testing.T) {
	for _, name := range []string{"postgres", "linux-ss"} {
		raw := systemDBFixture(t, name)
		for _, test := range []struct {
			name   string
			change func(map[string]any)
		}{
			{"unknown-field", func(c map[string]any) { c["password"] = "never-retain" }},
			{"version", func(c map[string]any) { c["collector_version"] = "2.0.0" }},
			{"path-id", func(c map[string]any) { c["host_id"] = "/private/path" }},
			{"overlap", func(c map[string]any) {
				s := c["snapshots"].([]any)
				s[1].(map[string]any)["capture_start_ns"] = s[0].(map[string]any)["capture_start_ns"]
			}},
			{"missing-end", func(c map[string]any) { delete(c["snapshots"].([]any)[0].(map[string]any), "capture_end_ns") }},
			{"empty", func(c map[string]any) { c["snapshots"] = []any{} }},
			{"count-bound", func(c map[string]any) {
				first := c["snapshots"].([]any)[0]
				many := make([]any, 257)
				for i := range many {
					many[i] = first
				}
				c["snapshots"] = many
			}},
		} {
			t.Run(name+"/"+test.name, func(t *testing.T) {
				if _, e := p.ParseBytes(ctx, changedCapture(t, raw, test.change), "fixture.json", p.Options{}); e == nil {
					t.Fatal("invalid capture accepted")
				}
			})
		}
		for _, o := range []p.Options{{Namespace: "override"}, {Producer: "wrong"}, {Query: json.RawMessage(`{"x":1}`)}, {ExportID: "override"}} {
			if _, e := p.ParseBytes(ctx, raw, "fixture.json", o); e == nil {
				t.Fatal("identity override accepted")
			}
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if _, e := p.ParseBytes(canceled, raw, "fixture.json", p.Options{}); e == nil {
			t.Fatal("cancellation ignored")
		}
	}
	pg := systemDBFixture(t, "postgres")
	for _, change := range []func(map[string]any){
		func(c map[string]any) { c["track_counts"] = false }, func(c map[string]any) { c["server_version_num"] = "160013" }, func(c map[string]any) { c["database_oid"] = "0" }, func(c map[string]any) { c["stats_fetch_consistency"] = "cache" },
		func(c map[string]any) { delete(c["snapshots"].([]any)[0].(map[string]any), "blks_read") }, func(c map[string]any) {
			c["snapshots"].([]any)[0].(map[string]any)["temp_bytes"] = strings.Repeat("9", 100000)
		},
		func(c map[string]any) {
			c["snapshots"].([]any)[0].(map[string]any)["stats_reset_ns"] = "18446744073709551615"
		},
	} {
		if _, e := p.ParseBytes(ctx, changedCapture(t, pg, change), "fixture.json", p.Options{}); e == nil {
			t.Fatal("invalid PG accepted")
		}
	}
	ss := systemDBFixture(t, "linux-ss")
	for _, change := range []func(map[string]any){
		func(c map[string]any) { c["iproute2_version"] = "6.2.0" }, func(c map[string]any) {
			c["snapshots"].([]any)[0].(map[string]any)["summary"] = "Netid State Recv-Q Send-Q Local Address:Port Peer Address:Port"
		},
		func(c map[string]any) {
			s := c["snapshots"].([]any)[0].(map[string]any)
			s["summary"] = s["summary"].(string) + "TCP 1 1 0\n"
		},
	} {
		if _, e := p.ParseBytes(ctx, changedCapture(t, ss, change), "fixture.json", p.Options{}); e == nil {
			t.Fatal("invalid ss accepted")
		}
	}
}
