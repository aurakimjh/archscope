package gclog

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	parser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/gclog"
)

func TestUnifiedClockAndMeasuredWindow(t *testing.T) {
	for _, mode := range []string{"uptime", "date_uptime", "date"} {
		t.Run(mode, func(t *testing.T) {
			body := ""
			for i := 0; i < 2; i++ {
				prefix := fmt.Sprintf("[%d.000s]", 1+i*10)
				if mode != "uptime" {
					prefix = fmt.Sprintf("[2026-11-09T10:00:%02d.000+0900]", 1+i*10)
					if mode == "date_uptime" {
						prefix += fmt.Sprintf("[%d.000s]", 1+i*10)
					}
				}
				body += fmt.Sprintf("%s[info][gc] GC(%d) Pause Young (G1 Evacuation Pause) %dM->%dM(200M) %d.000ms\n", prefix, i, 100+50*i, 50+10*i, 10+10*i)
			}
			path := filepath.Join(t.TempDir(), "clock.log")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			r, err := Analyze(path, Options{})
			if err != nil {
				t.Fatal(err)
			}
			if r.Summary["wall_time_sec"] != 10.0 || r.Summary["throughput_percent"] != 99.8 || r.Summary["throughput_pause_ms"] != 20.0 || r.Summary["avg_allocation_rate_mb_per_sec"] != 10.0 {
				t.Fatalf("wrong measured window: %+v", r.Summary)
			}
			axis := "absolute"
			if mode == "uptime" {
				axis = "uptime_seconds"
			}
			if r.Metadata.Extra["time_axis"] != axis {
				t.Fatal(r.Metadata.Extra)
			}
			if mode == "uptime" && r.Series["pause_timeline"].([]map[string]any)[0]["time"] != "1.0" {
				t.Fatal("lost uptime coordinate")
			}
		})
	}
}

func TestUnavailableGCMeasurements(t *testing.T) {
	f := func(n float64) *float64 { return &n }
	now := time.Now()
	for _, tc := range []struct {
		name   string
		events []parser.Event
		reason string
	}{
		{"single", []parser.Event{{UptimeSec: f(1), PauseMS: f(20)}}, "insufficient_events"},
		{"missing", []parser.Event{{PauseMS: f(10)}, {PauseMS: f(20)}}, "missing_time"},
		{"zero", []parser.Event{{UptimeSec: f(1)}, {UptimeSec: f(1)}}, "zero_time_span"},
		{"restart_with_dates", []parser.Event{{Timestamp: &now, UptimeSec: f(10)}, {Timestamp: &now, UptimeSec: f(1)}}, "clock_reset"},
		{"invalid_uptime", []parser.Event{{UptimeSec: f(math.NaN())}, {UptimeSec: f(2)}}, "missing_time"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Build(tc.events, "", parser.FormatUnified, nil, nil, Options{})
			for _, metric := range []string{"wall_time_sec", "throughput_percent", "avg_allocation_rate_mb_per_sec", "avg_promotion_rate_mb_per_sec"} {
				if r.Summary[metric] != nil {
					t.Fatalf("%s falsely measured: %v", metric, r.Summary[metric])
				}
			}
			if got := r.Metadata.Extra["measurement_unavailable"].(map[string]string)["throughput_percent"]; got != tc.reason {
				t.Fatalf("reason %s != %s", got, tc.reason)
			}
		})
	}
}

func TestGCWindowRejectsImpossiblePauseAndKeepsZeroRates(t *testing.T) {
	f := func(n float64) *float64 { return &n }
	events := []parser.Event{{UptimeSec: f(1), HeapAfterMB: f(50), OldAfterMB: f(10), PauseMS: f(20)}, {UptimeSec: f(2), HeapBeforeMB: f(50), OldAfterMB: f(10), PauseMS: f(2000)}}
	r := Build(events, "", parser.FormatUnified, nil, nil, Options{})
	if r.Summary["throughput_percent"] != nil || r.Summary["avg_allocation_rate_mb_per_sec"] != 0.0 || r.Summary["avg_promotion_rate_mb_per_sec"] != 0.0 {
		t.Fatal(r.Summary)
	}
	if r.Metadata.Extra["measurement_unavailable"].(map[string]string)["throughput_percent"] != "pause_exceeds_window" {
		t.Fatal(r.Metadata.Extra)
	}
}

func TestRatesUseOneClockWhenUptimeIsPartial(t *testing.T) {
	f := func(n float64) *float64 { return &n }
	start := time.Unix(0, 0)
	middle := start.Add(10 * time.Second)
	end := start.Add(20 * time.Second)
	events := []parser.Event{
		{Timestamp: &start, UptimeSec: f(0), HeapAfterMB: f(0), PauseMS: f(10)},
		{Timestamp: &middle, UptimeSec: f(1), HeapBeforeMB: f(100), HeapAfterMB: f(0), PauseMS: f(10)},
		{Timestamp: &end, HeapBeforeMB: f(100), PauseMS: f(10)},
	}
	r := Build(events, "", parser.FormatUnified, nil, nil, Options{})
	if r.Summary["avg_allocation_rate_mb_per_sec"] != 10.0 || r.Metadata.Extra["measurement_clock"] != "absolute" {
		t.Fatalf("mixed incompatible rate clocks: %+v", r.Summary)
	}
}
