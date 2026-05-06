package jfr

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aurakimjh/archscope/apps/engine-native/internal/diagnostics"
)

// writeJFRJSON marshals `payload` and writes it under tmp, mirroring
// the Python test helper `_write_jfr_json`.
func writeJFRJSON(t *testing.T, tmp string, payload any) string {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	path := filepath.Join(tmp, "jfr.json")
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func TestParsesRecordingFromSampleFixture(t *testing.T) {
	// Walk up from the package dir to repo root: …/apps/engine-native/internal/parsers/jfr
	// → ../../../../../examples/jfr/sample-jfr-print.json
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	repoRoot := filepath.Clean(filepath.Join(wd, "..", "..", "..", "..", ".."))
	sample := filepath.Join(repoRoot, "examples", "jfr", "sample-jfr-print.json")
	if _, err := os.Stat(sample); err != nil {
		t.Skipf("sample JFR data not available at %s", sample)
	}

	diags := diagnostics.New("jfr")
	events, err := ParseJSONFile(sample, diags)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("no events parsed from fixture")
	}
	if diags.ParsedRecords == 0 {
		t.Fatalf("ParsedRecords = 0, want > 0")
	}
}

func TestParsesExecutionSampleWithEventThread(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.ExecutionSample",
					"values": map[string]any{
						"startTime":   "2026-01-01T00:00:00.000Z",
						"eventThread": map[string]any{"javaName": "main", "osThreadId": 1},
						"stackTrace": map[string]any{
							"frames": []any{
								map[string]any{
									"method": map[string]any{
										"type": map[string]any{"name": "com.app.Main"},
										"name": "run",
									},
									"lineNumber": 42,
								},
							},
						},
						"state": "STATE_RUNNABLE",
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if events[0].EventType != "jdk.ExecutionSample" {
		t.Errorf("EventType = %q", events[0].EventType)
	}
	if events[0].Thread == nil || *events[0].Thread != "main" {
		t.Errorf("Thread = %v, want main", events[0].Thread)
	}
	if len(events[0].Frames) != 1 || !strings.Contains(events[0].Frames[0], "com.app.Main.run") {
		t.Errorf("Frames = %v", events[0].Frames)
	}
}

func TestParsesGCEventWithSuffixDuration(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.GarbageCollection",
					"values": map[string]any{
						"startTime": "2026-01-01T00:00:01.000Z",
						"duration":  "45 ms",
						"name":      "G1 Young Generation",
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d", len(events))
	}
	if events[0].DurationMS == nil || *events[0].DurationMS != 45.0 {
		t.Errorf("DurationMS = %v, want 45.0", events[0].DurationMS)
	}
	if events[0].Message != "G1 Young Generation" {
		t.Errorf("Message = %q", events[0].Message)
	}
}

func TestParsesNumericDuration(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.SocketRead",
					"values": map[string]any{
						"startTime": "2026-01-01T00:00:02.000Z",
						"duration":  120.5,
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if events[0].DurationMS == nil || *events[0].DurationMS != 120.5 {
		t.Errorf("DurationMS = %v, want 120.5", events[0].DurationMS)
	}
}

func TestParsesNanosecondDuration(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.FileRead",
					"values": map[string]any{
						"startTime": "2026-01-01T00:00:00Z",
						"duration":  "5000000 ns",
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if events[0].DurationMS == nil || *events[0].DurationMS != 5.0 {
		t.Errorf("DurationMS = %v, want 5.0", events[0].DurationMS)
	}
}

func TestParsesSecondsDuration(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.GCPhasePause",
					"values": map[string]any{
						"startTime": "2026-01-01T00:00:00Z",
						"duration":  "0.120 s",
						"name":      "Pause",
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if events[0].DurationMS == nil || *events[0].DurationMS != 120.0 {
		t.Errorf("DurationMS = %v, want 120.0", events[0].DurationMS)
	}
}

func TestParsesMicrosecondDuration(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.FileWrite",
					"values": map[string]any{
						"startTime": "2026-01-01T00:00:00Z",
						"duration":  "2500 us",
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if events[0].DurationMS == nil || *events[0].DurationMS != 2.5 {
		t.Errorf("DurationMS = %v, want 2.5", events[0].DurationMS)
	}
}

func TestParsesTopLevelEventsArray(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"events": []any{
			map[string]any{
				"type": "jdk.ExecutionSample",
				"values": map[string]any{
					"startTime":   "2026-01-01T00:00:00Z",
					"eventThread": map[string]any{"javaName": "pool-1"},
					"stackTrace":  map[string]any{"frames": []any{}},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d", len(events))
	}
	if events[0].Thread == nil || *events[0].Thread != "pool-1" {
		t.Errorf("Thread = %v, want pool-1", events[0].Thread)
	}
}

func TestParsesPlainEventsList(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), []any{
		map[string]any{
			"type": "jdk.GarbageCollection",
			"values": map[string]any{
				"startTime": "2026-01-01T00:00:00Z",
				"duration":  "10 ms",
				"name":      "Young",
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if len(events) != 1 || events[0].EventType != "jdk.GarbageCollection" {
		t.Fatalf("events = %+v", events)
	}
}

func TestEmptyEventsList(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{"events": []any{}},
	})
	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("events = %+v, want empty", events)
	}
}

func TestMissingRecordingKeyRaisesShapeError(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{"other": "data"})
	diags := diagnostics.New("jfr")
	_, err := ParseJSONFile(path, diags)
	if err == nil {
		t.Fatalf("expected error")
	}
	var shape *ShapeError
	if !errors.As(err, &shape) {
		t.Fatalf("want *ShapeError, got %T: %v", err, err)
	}
	if !strings.Contains(shape.Message, "does not contain an events array") {
		t.Errorf("message = %q", shape.Message)
	}
	if diags.SkippedByReason[ReasonInvalidJFRShape] != 1 {
		t.Errorf("SkippedByReason[%q] = %d", ReasonInvalidJFRShape, diags.SkippedByReason[ReasonInvalidJFRShape])
	}
}

func TestEventsWithoutValuesStillParsed(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{"type": "jdk.SomeEvent"},
			},
		},
	})

	diags := diagnostics.New("jfr")
	events, err := ParseJSONFile(path, diags)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d", len(events))
	}
	if events[0].EventType != "jdk.SomeEvent" {
		t.Errorf("EventType = %q", events[0].EventType)
	}
	if events[0].DurationMS != nil {
		t.Errorf("DurationMS = %v, want nil", events[0].DurationMS)
	}
	if events[0].Thread != nil {
		t.Errorf("Thread = %v, want nil", events[0].Thread)
	}
}

func TestNonDictEventsAreSkipped(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				"not a dict",
				42,
				map[string]any{
					"type": "jdk.Valid",
					"values": map[string]any{
						"startTime": "2026-01-01T00:00:00Z",
					},
				},
			},
		},
	})

	diags := diagnostics.New("jfr")
	events, err := ParseJSONFile(path, diags)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d, want 1", len(events))
	}
	if diags.SkippedLines != 2 {
		t.Errorf("SkippedLines = %d, want 2", diags.SkippedLines)
	}
	if diags.SkippedByReason[ReasonInvalidJFREvent] != 2 {
		t.Errorf("SkippedByReason[%q] = %d", ReasonInvalidJFREvent, diags.SkippedByReason[ReasonInvalidJFREvent])
	}
}

func TestInvalidJSONRaisesDecodeError(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "jfr.json")
	if err := os.WriteFile(path, []byte("not valid json {{{"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	diags := diagnostics.New("jfr")
	_, err := ParseJSONFile(path, diags)
	if err == nil {
		t.Fatalf("expected error")
	}
	var decode *JSONDecodeError
	if !errors.As(err, &decode) {
		t.Fatalf("want *JSONDecodeError, got %T: %v", err, err)
	}
	if diags.SkippedByReason[ReasonInvalidJFRJSON] != 1 {
		t.Errorf("SkippedByReason[%q] = %d", ReasonInvalidJFRJSON, diags.SkippedByReason[ReasonInvalidJFRJSON])
	}
}

func TestStackFramesExtractedCorrectly(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.ExecutionSample",
					"values": map[string]any{
						"startTime":   "2026-01-01T00:00:00Z",
						"eventThread": map[string]any{"javaName": "worker-1"},
						"stackTrace": map[string]any{
							"frames": []any{
								map[string]any{
									"method": map[string]any{
										"type": map[string]any{"name": "java.lang.Thread"},
										"name": "sleep",
									},
									"lineNumber": -1,
								},
								map[string]any{
									"method": map[string]any{
										"type": map[string]any{"name": "com.app.Worker"},
										"name": "process",
									},
									"lineNumber": 55,
								},
							},
						},
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	frames := events[0].Frames
	if len(frames) != 2 {
		t.Fatalf("len(frames) = %d", len(frames))
	}
	if !strings.Contains(frames[0], "java.lang.Thread.sleep") {
		t.Errorf("frames[0] = %q", frames[0])
	}
	if !strings.Contains(frames[1], "com.app.Worker.process") {
		t.Errorf("frames[1] = %q", frames[1])
	}
}

func TestThreadFromStringValue(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.ExecutionSample",
					"values": map[string]any{
						"startTime":   "2026-01-01T00:00:00Z",
						"eventThread": "simple-thread-name",
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if events[0].Thread == nil || *events[0].Thread != "simple-thread-name" {
		t.Errorf("Thread = %v", events[0].Thread)
	}
}

func TestDurationNoneForInvalidString(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.SomeEvent",
					"values": map[string]any{
						"startTime": "2026-01-01T00:00:00Z",
						"duration":  "not-a-number",
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if events[0].DurationMS != nil {
		t.Errorf("DurationMS = %v, want nil", events[0].DurationMS)
	}
}

func TestStateExtractedFromObject(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.JavaMonitorWait",
					"values": map[string]any{
						"startTime": "2026-01-01T00:00:00Z",
						"state":     map[string]any{"name": "STATE_BLOCKED"},
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if events[0].State == nil || *events[0].State != "STATE_BLOCKED" {
		t.Errorf("State = %v", events[0].State)
	}
}

func TestAddressAndSizeFromHexAndDecimal(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{
					"type": "jdk.ObjectAllocation",
					"values": map[string]any{
						"startTime":      "2026-01-01T00:00:00Z",
						"address":        "0x7fff1234",
						"allocationSize": 4096,
					},
				},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if events[0].Address == nil || *events[0].Address != 0x7fff1234 {
		t.Errorf("Address = %v, want 0x7fff1234", events[0].Address)
	}
	if events[0].Size == nil || *events[0].Size != 4096 {
		t.Errorf("Size = %v, want 4096", events[0].Size)
	}
}

func TestUnknownEventTypeFallback(t *testing.T) {
	path := writeJFRJSON(t, t.TempDir(), map[string]any{
		"recording": map[string]any{
			"events": []any{
				map[string]any{"values": map[string]any{"startTime": "2026-01-01T00:00:00Z"}},
			},
		},
	})

	events, err := ParseJSONFile(path, nil)
	if err != nil {
		t.Fatalf("ParseJSONFile: %v", err)
	}
	if events[0].EventType != "unknown" {
		t.Errorf("EventType = %q, want unknown", events[0].EventType)
	}
}
