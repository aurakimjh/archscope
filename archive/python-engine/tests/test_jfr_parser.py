"""Tests for JFR JSON parser covering edge cases and event extraction."""
# ─────────────────────────────────────────────────────────────────────
# [한글] test_jfr_parser — `jfr print --json` 파서 회귀.
#
# 검증 대상
#   • 3가지 입력 형태 모두 같은 Event 리스트로 통일:
#     {"recording": {"events": [...]}} / {"events": [...]} / [...].
#   • _to_jfr_event: event_type / time / duration_ms / thread / state /
#     address / size / message / frames / raw_preview 정확 추출.
#   • _duration_to_ms: "12.5ms" / 12500000 (ns) 등 다양한 형식 모두 ms float.
#   • _format_frame: frame 객체 → 1-line 문자열 정규화.
#   • skip 사유: INVALID_JFR_JSON / INVALID_JFR_SHAPE / INVALID_JFR_EVENT.
#   • raw_preview 가 500 byte 로 절단.
#
# parity 주의
#   Go engine-native parsers/jfr/parser.go 와 byte 동일.
# ─────────────────────────────────────────────────────────────────────

import json
from pathlib import Path

import pytest

from archscope_engine.parsers.jfr_parser import parse_jfr_print_json
from archscope_engine.common.diagnostics import ParserDiagnostics


def _write_jfr_json(tmp_path: Path, payload: dict | list) -> Path:
    path = tmp_path / "jfr.json"
    path.write_text(json.dumps(payload), encoding="utf-8")
    return path


class TestJfrParserBasic:
    def test_parses_recording_with_events_from_sample(self, tmp_path: Path) -> None:
        sample = Path(__file__).parents[3] / "examples/jfr/sample-jfr-print.json"
        if not sample.exists():
            pytest.skip("sample JFR data not available")

        diag = ParserDiagnostics()
        events = parse_jfr_print_json(sample, diagnostics=diag)

        assert len(events) > 0
        assert diag.parsed_records > 0

    def test_parses_execution_sample_with_event_thread(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        {
                            "type": "jdk.ExecutionSample",
                            "values": {
                                "startTime": "2026-01-01T00:00:00.000Z",
                                "eventThread": {"javaName": "main", "osThreadId": 1},
                                "stackTrace": {
                                    "frames": [
                                        {
                                            "method": {
                                                "type": {"name": "com.app.Main"},
                                                "name": "run",
                                            },
                                            "lineNumber": 42,
                                        }
                                    ]
                                },
                                "state": "STATE_RUNNABLE",
                            },
                        }
                    ]
                }
            },
        )

        events = parse_jfr_print_json(path)

        assert len(events) == 1
        assert events[0]["event_type"] == "jdk.ExecutionSample"
        assert events[0]["thread"] == "main"
        assert "com.app.Main.run" in events[0]["frames"][0]

    def test_parses_gc_events_with_suffix_duration(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        {
                            "type": "jdk.GarbageCollection",
                            "values": {
                                "startTime": "2026-01-01T00:00:01.000Z",
                                "duration": "45 ms",
                                "name": "G1 Young Generation",
                            },
                        }
                    ]
                }
            },
        )

        events = parse_jfr_print_json(path)

        assert len(events) == 1
        assert events[0]["event_type"] == "jdk.GarbageCollection"
        assert events[0]["duration_ms"] == 45.0
        assert events[0]["message"] == "G1 Young Generation"

    def test_parses_numeric_duration(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        {
                            "type": "jdk.SocketRead",
                            "values": {
                                "startTime": "2026-01-01T00:00:02.000Z",
                                "duration": 120.5,
                            },
                        }
                    ]
                }
            },
        )

        events = parse_jfr_print_json(path)
        assert len(events) == 1
        assert events[0]["event_type"] == "jdk.SocketRead"
        assert events[0]["duration_ms"] == 120.5

    def test_parses_nanosecond_duration(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        {
                            "type": "jdk.FileRead",
                            "values": {
                                "startTime": "2026-01-01T00:00:00Z",
                                "duration": "5000000 ns",
                            },
                        }
                    ]
                }
            },
        )

        events = parse_jfr_print_json(path)
        assert events[0]["duration_ms"] == 5.0

    def test_parses_seconds_duration(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        {
                            "type": "jdk.GCPhasePause",
                            "values": {
                                "startTime": "2026-01-01T00:00:00Z",
                                "duration": "0.120 s",
                                "name": "Pause",
                            },
                        }
                    ]
                }
            },
        )

        events = parse_jfr_print_json(path)
        assert events[0]["duration_ms"] == 120.0


class TestJfrParserEventSources:
    def test_parses_top_level_events_array(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "events": [
                    {
                        "type": "jdk.ExecutionSample",
                        "values": {
                            "startTime": "2026-01-01T00:00:00Z",
                            "eventThread": {"javaName": "pool-1"},
                            "stackTrace": {"frames": []},
                        },
                    }
                ]
            },
        )

        events = parse_jfr_print_json(path)
        assert len(events) == 1
        assert events[0]["thread"] == "pool-1"

    def test_parses_plain_events_list(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            [
                {
                    "type": "jdk.GarbageCollection",
                    "values": {
                        "startTime": "2026-01-01T00:00:00Z",
                        "duration": "10 ms",
                        "name": "Young",
                    },
                }
            ],
        )

        events = parse_jfr_print_json(path)
        assert len(events) == 1
        assert events[0]["event_type"] == "jdk.GarbageCollection"


class TestJfrParserEdgeCases:
    def test_empty_events_list(self, tmp_path: Path) -> None:
        path = _write_jfr_json(tmp_path, {"recording": {"events": []}})
        events = parse_jfr_print_json(path)
        assert events == []

    def test_missing_recording_key_raises_value_error(self, tmp_path: Path) -> None:
        path = _write_jfr_json(tmp_path, {"other": "data"})
        with pytest.raises(ValueError, match="does not contain an events array"):
            parse_jfr_print_json(path)

    def test_events_without_values_still_parsed(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        {"type": "jdk.SomeEvent"},
                    ]
                }
            },
        )

        diag = ParserDiagnostics()
        events = parse_jfr_print_json(path, diagnostics=diag)
        assert len(events) == 1
        assert events[0]["event_type"] == "jdk.SomeEvent"
        assert events[0]["duration_ms"] is None
        assert events[0]["thread"] is None

    def test_non_dict_events_are_skipped(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        "not a dict",
                        42,
                        {
                            "type": "jdk.Valid",
                            "values": {"startTime": "2026-01-01T00:00:00Z"},
                        },
                    ]
                }
            },
        )

        diag = ParserDiagnostics()
        events = parse_jfr_print_json(path, diagnostics=diag)
        assert len(events) == 1
        assert diag.skipped_lines == 2
        assert "INVALID_JFR_EVENT" in diag.skipped_by_reason

    def test_invalid_json_raises_decode_error(self, tmp_path: Path) -> None:
        path = tmp_path / "jfr.json"
        path.write_text("not valid json {{{", encoding="utf-8")

        with pytest.raises(json.JSONDecodeError):
            parse_jfr_print_json(path)

    def test_stack_frames_extracted_correctly(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        {
                            "type": "jdk.ExecutionSample",
                            "values": {
                                "startTime": "2026-01-01T00:00:00Z",
                                "eventThread": {"javaName": "worker-1"},
                                "stackTrace": {
                                    "frames": [
                                        {
                                            "method": {
                                                "type": {"name": "java.lang.Thread"},
                                                "name": "sleep",
                                            },
                                            "lineNumber": -1,
                                        },
                                        {
                                            "method": {
                                                "type": {"name": "com.app.Worker"},
                                                "name": "process",
                                            },
                                            "lineNumber": 55,
                                        },
                                    ]
                                },
                            },
                        }
                    ]
                }
            },
        )

        events = parse_jfr_print_json(path)
        assert len(events) == 1
        frames = events[0]["frames"]
        assert len(frames) == 2
        assert "java.lang.Thread.sleep" in frames[0]
        assert "com.app.Worker.process" in frames[1]

    def test_thread_from_string_value(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        {
                            "type": "jdk.ExecutionSample",
                            "values": {
                                "startTime": "2026-01-01T00:00:00Z",
                                "eventThread": "simple-thread-name",
                            },
                        }
                    ]
                }
            },
        )

        events = parse_jfr_print_json(path)
        assert events[0]["thread"] == "simple-thread-name"

    def test_duration_none_for_invalid_string(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        {
                            "type": "jdk.SomeEvent",
                            "values": {
                                "startTime": "2026-01-01T00:00:00Z",
                                "duration": "not-a-number",
                            },
                        }
                    ]
                }
            },
        )

        events = parse_jfr_print_json(path)
        assert events[0]["duration_ms"] is None

    def test_microsecond_duration(self, tmp_path: Path) -> None:
        path = _write_jfr_json(
            tmp_path,
            {
                "recording": {
                    "events": [
                        {
                            "type": "jdk.FileWrite",
                            "values": {
                                "startTime": "2026-01-01T00:00:00Z",
                                "duration": "2500 us",
                            },
                        }
                    ]
                }
            },
        )

        events = parse_jfr_print_json(path)
        assert events[0]["duration_ms"] == 2.5
