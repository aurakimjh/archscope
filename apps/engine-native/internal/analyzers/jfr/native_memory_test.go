package jfr

import (
	"testing"

	jfrparser "github.com/aurakimjh/archscope/apps/engine-native/internal/parsers/jfr"
)

func ptrInt64(v int64) *int64 { return &v }

func TestNativeMemoryLeakOnlyDefault(t *testing.T) {
	// 3 allocs, 1 free at addr=2 → addrs 1 and 3 unfreed.
	// Without a tail cutoff (only 2 timestamps spread, all kept) the
	// leak set is the alloc events at addr 1 and 3.
	events := []jfrparser.Event{
		{
			EventType: "jdk.NativeMemoryAllocation",
			Time:      "2026-04-30T01:00:00+00:00",
			Address:   ptrInt64(1),
			Size:      ptrInt64(1024),
			Frames:    []string{"libc.malloc", "app.boot"},
		},
		{
			EventType: "jdk.NativeMemoryAllocation",
			Time:      "2026-04-30T01:00:01+00:00",
			Address:   ptrInt64(2),
			Size:      ptrInt64(2048),
			Frames:    []string{"libc.malloc", "app.cache"},
		},
		{
			EventType: "jdk.NativeMemoryAllocation",
			Time:      "2026-04-30T01:00:02+00:00",
			Address:   ptrInt64(3),
			Size:      ptrInt64(512),
			Frames:    []string{"libc.malloc", "app.boot"},
		},
		{
			EventType: "jdk.NativeMemoryFree",
			Time:      "2026-04-30T01:00:03+00:00",
			Address:   ptrInt64(2),
			Size:      ptrInt64(2048),
		},
	}
	opts := NewNativeMemoryOptions()
	// tail_ratio=0.10 → cutoff at ~start + 90% of 2s = 1.8s. Alloc at
	// +2s lands AFTER the cutoff, so it's excluded.
	result := BuildNativeMemory(events, "/tmp/fake.jfr", jfrparser.SourceInfo{SourceFormat: "binary_jfr"}, nil, opts)

	if result.Type != NativeMemoryResultType {
		t.Fatalf("Type = %q, want %q", result.Type, NativeMemoryResultType)
	}
	if got := getInt(result.Summary, "alloc_event_count"); got != 3 {
		t.Errorf("alloc_event_count = %d, want 3", got)
	}
	if got := getInt(result.Summary, "free_event_count"); got != 1 {
		t.Errorf("free_event_count = %d, want 1", got)
	}
	if got := getInt(result.Summary, "alloc_bytes_total"); got != 3584 {
		t.Errorf("alloc_bytes_total = %d, want 3584", got)
	}
	// Tail cutoff filters out the +2s alloc (size 512), and addr 2 is
	// freed → only addr 1 (1024B) survives in the leak set.
	if got := getInt(result.Summary, "unfreed_event_count"); got != 1 {
		t.Errorf("unfreed_event_count = %d, want 1", got)
	}
	if got := getInt(result.Summary, "unfreed_bytes_total"); got != 1024 {
		t.Errorf("unfreed_bytes_total = %d, want 1024", got)
	}
	if got := result.Summary["leak_only"]; got != true {
		t.Errorf("leak_only = %v, want true", got)
	}
	if result.Summary["tail_cutoff"] == nil {
		t.Errorf("tail_cutoff should not be nil for non-zero tail_ratio")
	}
}

func TestNativeMemoryAllAllocations(t *testing.T) {
	// leak_only=false → all allocations survive.
	events := []jfrparser.Event{
		{
			EventType: "jdk.NativeMemoryAllocation",
			Time:      "2026-04-30T01:00:00+00:00",
			Address:   ptrInt64(1),
			Size:      ptrInt64(1024),
			Frames:    []string{"a.b"},
		},
		{
			EventType: "jdk.NativeMemoryFree",
			Address:   ptrInt64(1),
		},
	}
	opts := NewNativeMemoryOptions()
	opts.LeakOnly = false
	result := BuildNativeMemory(events, "/tmp/fake.jfr", jfrparser.SourceInfo{SourceFormat: "json"}, nil, opts)
	if got := getInt(result.Summary, "unfreed_event_count"); got != 1 {
		t.Errorf("unfreed_event_count under leak_only=false = %d, want 1", got)
	}
	if got := getInt(result.Summary, "unfreed_bytes_total"); got != 1024 {
		t.Errorf("unfreed_bytes_total under leak_only=false = %d, want 1024", got)
	}
}

func TestNativeMemoryDisablesTailCutoff(t *testing.T) {
	events := []jfrparser.Event{
		{
			EventType: "jdk.NativeMemoryAllocation",
			Time:      "2026-04-30T01:00:00+00:00",
			Address:   ptrInt64(1),
			Size:      ptrInt64(100),
			Frames:    []string{"f"},
		},
		{
			EventType: "jdk.NativeMemoryAllocation",
			Time:      "2026-04-30T01:00:10+00:00",
			Address:   ptrInt64(2),
			Size:      ptrInt64(200),
			Frames:    []string{"f"},
		},
	}
	opts := NewNativeMemoryOptions()
	opts.TailRatio = 0
	opts.tailRatioSet = true
	result := BuildNativeMemory(events, "/tmp/fake.jfr", jfrparser.SourceInfo{SourceFormat: "json"}, nil, opts)
	if got := getInt(result.Summary, "unfreed_event_count"); got != 2 {
		t.Errorf("tail_ratio=0 unfreed_event_count = %d, want 2 (no exclusion)", got)
	}
	if result.Summary["tail_cutoff"] != nil {
		t.Errorf("tail_cutoff should be nil when tail_ratio=0, got %v", result.Summary["tail_cutoff"])
	}
}

func TestNativeMemoryFlamegraphAndTopSites(t *testing.T) {
	events := []jfrparser.Event{
		{
			EventType: "jdk.NativeMemoryAllocation",
			Address:   ptrInt64(1),
			Size:      ptrInt64(100),
			Frames:    []string{"a", "b", "c"},
		},
		{
			EventType: "jdk.NativeMemoryAllocation",
			Address:   ptrInt64(2),
			Size:      ptrInt64(50),
			Frames:    []string{"a", "b", "c"},
		},
		{
			EventType: "jdk.NativeMemoryAllocation",
			Address:   ptrInt64(3),
			Size:      ptrInt64(30),
			Frames:    []string{"a", "b", "d"},
		},
	}
	opts := NewNativeMemoryOptions()
	opts.TailRatio = 0
	opts.tailRatioSet = true
	result := BuildNativeMemory(events, "/tmp/fake.jfr", jfrparser.SourceInfo{SourceFormat: "json"}, nil, opts)

	tops := result.Tables["top_call_sites"].([]map[string]any)
	if len(tops) != 2 {
		t.Fatalf("top_call_sites length = %d, want 2", len(tops))
	}
	if tops[0]["stack"] != "a;b;c" {
		t.Errorf("top_call_sites[0].stack = %v, want a;b;c", tops[0]["stack"])
	}
	if asInt64(tops[0]["bytes"]) != 150 {
		t.Errorf("top_call_sites[0].bytes = %v, want 150", tops[0]["bytes"])
	}

	flame := result.Charts["flamegraph"].(map[string]any)
	if flame["name"] != "All" {
		t.Errorf("flame.name = %v, want All", flame["name"])
	}
	if asInt64(flame["samples"]) != 180 {
		t.Errorf("flame.samples = %v, want 180", flame["samples"])
	}
	// Root has one child 'a' (samples 180), which has one child 'b'
	// (samples 180), which splits into 'c' (150) and 'd' (30).
	rootChildren := flame["children"].([]map[string]any)
	if len(rootChildren) != 1 || rootChildren[0]["name"] != "a" {
		t.Fatalf("flame.children = %+v, want single 'a'", rootChildren)
	}
	if asInt64(rootChildren[0]["samples"]) != 180 {
		t.Errorf("a.samples = %v, want 180", rootChildren[0]["samples"])
	}
	bChildren := rootChildren[0]["children"].([]map[string]any)
	if len(bChildren) != 1 || bChildren[0]["name"] != "b" {
		t.Fatalf("a.children = %+v, want single 'b'", bChildren)
	}
	cdChildren := bChildren[0]["children"].([]map[string]any)
	if len(cdChildren) != 2 {
		t.Fatalf("b.children length = %d, want 2", len(cdChildren))
	}
	// Sorted by samples desc → 'c' (150) before 'd' (30).
	if cdChildren[0]["name"] != "c" {
		t.Errorf("b.children[0].name = %v, want c", cdChildren[0]["name"])
	}
	if cdChildren[1]["name"] != "d" {
		t.Errorf("b.children[1].name = %v, want d", cdChildren[1]["name"])
	}
}

func TestNativeMemoryFallsBackToOnePerEventWhenSizeMissing(t *testing.T) {
	events := []jfrparser.Event{
		{
			EventType: "jdk.NativeMemoryAllocation",
			Address:   ptrInt64(1),
			Frames:    []string{"x", "y"},
		},
		{
			EventType: "jdk.NativeMemoryAllocation",
			Address:   ptrInt64(2),
			Frames:    []string{"x", "y"},
		},
	}
	opts := NewNativeMemoryOptions()
	opts.TailRatio = 0
	opts.tailRatioSet = true
	result := BuildNativeMemory(events, "/tmp/fake.jfr", jfrparser.SourceInfo{SourceFormat: "json"}, nil, opts)
	tops := result.Tables["top_call_sites"].([]map[string]any)
	if len(tops) != 1 {
		t.Fatalf("top_call_sites length = %d, want 1", len(tops))
	}
	// Two events, each contributing 1 → 2 bytes-of-events.
	if asInt64(tops[0]["bytes"]) != 2 {
		t.Errorf("top_call_sites[0].bytes = %v, want 2 (1-per-event fallback)", tops[0]["bytes"])
	}
}

func TestNativeMemoryMetadata(t *testing.T) {
	events := []jfrparser.Event{}
	opts := NewNativeMemoryOptions()
	cli := "/usr/bin/jfr"
	info := jfrparser.SourceInfo{SourceFormat: "binary_jfr", JFRCli: &cli}
	result := BuildNativeMemory(events, "/tmp/fake.jfr", info, nil, opts)
	if result.Metadata.Parser != NativeMemoryParserName {
		t.Errorf("parser = %q, want %q", result.Metadata.Parser, NativeMemoryParserName)
	}
	if result.Metadata.SchemaVersion != NativeMemorySchemaVersion {
		t.Errorf("schema_version = %q, want %q", result.Metadata.SchemaVersion, NativeMemorySchemaVersion)
	}
	if result.Metadata.Extra["unit"] != "bytes" {
		t.Errorf("metadata.unit = %v, want bytes", result.Metadata.Extra["unit"])
	}
	if result.Metadata.Extra["jfr_cli"] != "/usr/bin/jfr" {
		t.Errorf("metadata.jfr_cli = %v, want /usr/bin/jfr", result.Metadata.Extra["jfr_cli"])
	}
	if result.Metadata.Extra["source_format"] != "binary_jfr" {
		t.Errorf("metadata.source_format = %v, want binary_jfr", result.Metadata.Extra["source_format"])
	}
}

func TestFlameTreeEmpty(t *testing.T) {
	flame := buildFlameTreeFromCollapsed(map[string]int64{})
	if flame["name"] != "All" {
		t.Errorf("empty flame.name = %v, want All", flame["name"])
	}
	if asInt64(flame["samples"]) != 0 {
		t.Errorf("empty flame.samples = %v, want 0", flame["samples"])
	}
	children := flame["children"].([]map[string]any)
	if len(children) != 0 {
		t.Errorf("empty flame.children length = %d, want 0", len(children))
	}
}

func asInt64(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}
