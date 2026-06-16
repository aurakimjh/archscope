package jenniferprofile

import (
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/models"
)

func ip(v int) *int { return &v }

// ev builds a body event with a [start, start+elapsed] interval.
func ev(t models.JenniferEventType, raw string, start, elapsed int) models.JenniferProfileEvent {
	return models.JenniferProfileEvent{
		EventType:     t,
		RawMessage:    raw,
		ElapsedMs:     ip(elapsed),
		StartOffsetMs: ip(start),
		EndOffsetMs:   ip(start + elapsed),
	}
}

func profileOf(app string, evs ...models.JenniferProfileEvent) models.JenniferTransactionProfile {
	return models.JenniferTransactionProfile{
		Header: models.JenniferProfileHeader{Application: app, TXID: "tx-" + app},
		Body:   models.JenniferProfileBody{Events: evs},
	}
}

func findHotspot(hs []models.JenniferMethodHotspot, method string) (models.JenniferMethodHotspot, bool) {
	for _, h := range hs {
		if h.Method == method {
			return h, true
		}
	}
	return models.JenniferMethodHotspot{}, false
}

const (
	mMETHOD = models.JenniferEventMethod
	mSQL    = models.JenniferEventSQLQuery
	mEXT    = models.JenniferEventExternalCall
)

// Nested SQL + external are subtracted from the enclosing method; the
// leaf SQL/external frames are not ranked as methods.
func TestMethodHotspots_SubtractsNestedSQLAndExternal(t *testing.T) {
	p := profileOf("api",
		ev(mMETHOD, "handle()", 0, 100),
		ev(mMETHOD, "process()", 10, 80), // child of handle
		ev(mSQL, "SELECT ...", 20, 30),   // child of process
		ev(mEXT, "POST /svc", 60, 20),    // child of process (gap 50..60 is process self)
	)
	hs := MethodHotspots(p, 0)

	if len(hs) != 2 {
		t.Fatalf("want 2 method hotspots, got %d: %+v", len(hs), hs)
	}
	// process: 80 - union(30,20)=50 => self 30 ; handle: 100 - 80 => self 20
	process, ok := findHotspot(hs, "process()")
	if !ok {
		t.Fatalf("process() missing: %+v", hs)
	}
	if process.SelfTimeMs != 30 {
		t.Errorf("process self = %d, want 30", process.SelfTimeMs)
	}
	if process.SqlMs != 30 || process.ExternalMs != 20 {
		t.Errorf("process breakdown sql=%d ext=%d, want 30/20", process.SqlMs, process.ExternalMs)
	}
	if process.TotalElapsedMs != 80 || process.Calls != 1 {
		t.Errorf("process total=%d calls=%d, want 80/1", process.TotalElapsedMs, process.Calls)
	}

	handle, _ := findHotspot(hs, "handle()")
	if handle.SelfTimeMs != 20 || handle.ChildMethodMs != 80 {
		t.Errorf("handle self=%d childMethod=%d, want 20/80", handle.SelfTimeMs, handle.ChildMethodMs)
	}
	// ranking: process(30) before handle(20)
	if hs[0].Method != "process()" || hs[1].Method != "handle()" {
		t.Errorf("ranking = [%s, %s], want [process(), handle()]", hs[0].Method, hs[1].Method)
	}
	// leaf SQL/external must not appear as methods
	if _, ok := findHotspot(hs, "SELECT ..."); ok {
		t.Error("SQL leaf should not be ranked as a method")
	}
}

// Two parallel (overlapping) external calls under one method must be
// charged once via interval-union, not summed.
func TestMethodHotspots_OverlapUnionNotSum(t *testing.T) {
	p := profileOf("api",
		ev(mMETHOD, "fanout()", 0, 100),
		ev(mEXT, "callA", 10, 50), // 10..60
		ev(mEXT, "callB", 30, 50), // 30..80  (overlaps A)
	)
	hs := MethodHotspots(p, 0)
	if len(hs) != 1 {
		t.Fatalf("want 1 hotspot, got %d", len(hs))
	}
	// union(10..60, 30..80) = 10..80 = 70 => self 30 (naive sum would be 0)
	if hs[0].SelfTimeMs != 30 {
		t.Errorf("self = %d, want 30 (overlap union)", hs[0].SelfTimeMs)
	}
	if hs[0].ExternalMs != 70 {
		t.Errorf("external = %d, want 70 (union)", hs[0].ExternalMs)
	}
}

// The same method signature called multiple times aggregates with call
// counts and max self.
func TestMethodHotspots_AggregatesBySignature(t *testing.T) {
	p := profileOf("api",
		ev(mMETHOD, "main()", 0, 100),
		ev(mMETHOD, "loop()", 0, 20),  // self 20
		ev(mMETHOD, "loop()", 30, 15), // self 15
	)
	hs := MethodHotspots(p, 0)
	loop, ok := findHotspot(hs, "loop()")
	if !ok {
		t.Fatalf("loop() missing: %+v", hs)
	}
	if loop.Calls != 2 || loop.SelfTimeMs != 35 || loop.MaxSelfMs != 20 {
		t.Errorf("loop calls=%d self=%d max=%d, want 2/35/20", loop.Calls, loop.SelfTimeMs, loop.MaxSelfMs)
	}
	if loop.AvgSelfMs != 17.5 {
		t.Errorf("loop avg=%v, want 17.5", loop.AvgSelfMs)
	}
	main, _ := findHotspot(hs, "main()")
	if main.SelfTimeMs != 65 || main.ChildMethodMs != 35 {
		t.Errorf("main self=%d childMethod=%d, want 65/35", main.SelfTimeMs, main.ChildMethodMs)
	}
	if hs[0].Method != "main()" {
		t.Errorf("top method = %s, want main()", hs[0].Method)
	}
}

// A method whose elapsed is unreported is skipped; empty profile yields
// no hotspots.
func TestMethodHotspots_EdgeCases(t *testing.T) {
	if got := MethodHotspots(profileOf("api"), 0); got != nil {
		t.Errorf("empty profile = %+v, want nil", got)
	}
	noElapsed := models.JenniferProfileEvent{EventType: mMETHOD, RawMessage: "x()", StartOffsetMs: ip(0)}
	p := profileOf("api", noElapsed)
	if got := MethodHotspots(p, 0); len(got) != 0 {
		t.Errorf("method without elapsed should be skipped, got %+v", got)
	}
}

// RollUpMethodHotspots aggregates per-profile rows by application+method
// into a group ranking.
func TestRollUpMethodHotspots(t *testing.T) {
	in := []models.JenniferMethodHotspot{
		{Application: "svc", Method: "m", SelfTimeMs: 30, TotalElapsedMs: 80, Calls: 1, MaxSelfMs: 30, SqlMs: 30},
		{Application: "svc", Method: "m", SelfTimeMs: 20, TotalElapsedMs: 50, Calls: 1, MaxSelfMs: 20, SqlMs: 10},
		{Application: "svc", Method: "n", SelfTimeMs: 5, TotalElapsedMs: 10, Calls: 1, MaxSelfMs: 5},
	}
	out := RollUpMethodHotspots(in, "guid-1", 0)
	if len(out) != 2 {
		t.Fatalf("want 2 rolled methods, got %d", len(out))
	}
	if out[0].Method != "m" || out[0].SelfTimeMs != 50 || out[0].Calls != 2 || out[0].MaxSelfMs != 30 || out[0].SqlMs != 40 {
		t.Errorf("rolled m = %+v, want self50 calls2 max30 sql40", out[0])
	}
	if out[0].GUID != "guid-1" {
		t.Errorf("guid = %q, want guid-1", out[0].GUID)
	}
	if out[1].Method != "n" {
		t.Errorf("second = %s, want n", out[1].Method)
	}
}
