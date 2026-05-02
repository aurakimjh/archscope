"""Tests for OTel JSONL parser covering edge cases and field alias resolution."""

import json
from pathlib import Path

from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.parsers.otel_parser import parse_otel_jsonl


def _write_jsonl(tmp_path: Path, lines: list[dict]) -> Path:
    path = tmp_path / "otel.jsonl"
    path.write_text(
        "\n".join(json.dumps(line) for line in lines),
        encoding="utf-8",
    )
    return path


class TestOtelParserFieldAliases:
    def test_parses_standard_otel_fields(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [
                {
                    "timestamp": "2026-01-01T00:00:00Z",
                    "trace_id": "abc123",
                    "span_id": "span1",
                    "severity_text": "ERROR",
                    "service_name": "order-service",
                    "body": "Connection timeout",
                }
            ],
        )
        records = parse_otel_jsonl(path)

        assert len(records) == 1
        assert records[0].trace_id == "abc123"
        assert records[0].span_id == "span1"
        assert records[0].severity == "ERROR"
        assert records[0].service_name == "order-service"
        assert records[0].body == "Connection timeout"

    def test_parses_camelcase_field_aliases(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [
                {
                    "traceId": "t1",
                    "spanId": "s1",
                    "parentSpanId": "ps1",
                    "severityText": "WARN",
                    "service.name": "api-gw",
                    "body": "Slow response",
                }
            ],
        )
        records = parse_otel_jsonl(path)

        assert len(records) == 1
        assert records[0].trace_id == "t1"
        assert records[0].span_id == "s1"
        assert records[0].parent_span_id == "ps1"
        assert records[0].service_name == "api-gw"

    def test_extracts_service_from_resource_attributes(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [
                {
                    "trace_id": "t1",
                    "body": "test",
                    "resource": {
                        "attributes": {"service.name": "payment-service"}
                    },
                }
            ],
        )
        records = parse_otel_jsonl(path)
        assert records[0].service_name == "payment-service"

    def test_extracts_service_from_top_level_attributes(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [
                {
                    "trace_id": "t1",
                    "body": "test",
                    "attributes": {"service.name": "user-service"},
                }
            ],
        )
        records = parse_otel_jsonl(path)
        assert records[0].service_name == "user-service"

    def test_extracts_parent_span_from_attributes(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [
                {
                    "trace_id": "t1",
                    "body": "test",
                    "attributes": {"parentSpanId": "parent1"},
                }
            ],
        )
        records = parse_otel_jsonl(path)
        assert records[0].parent_span_id == "parent1"

    def test_body_from_nested_string_value(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [
                {
                    "trace_id": "t1",
                    "body": {"stringValue": "Nested body text"},
                }
            ],
        )
        records = parse_otel_jsonl(path)
        assert records[0].body == "Nested body text"

    def test_body_from_message_field(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [{"trace_id": "t1", "message": "A log message"}],
        )
        records = parse_otel_jsonl(path)
        assert records[0].body == "A log message"

    def test_numeric_trace_id_converted_to_string(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [{"trace_id": 12345, "body": "numeric id"}],
        )
        records = parse_otel_jsonl(path)
        assert records[0].trace_id == "12345"

    def test_boolean_body_converted_to_string(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [{"trace_id": "t1", "body": True}],
        )
        records = parse_otel_jsonl(path)
        assert records[0].body == "True"


class TestOtelParserDiagnostics:
    def test_skips_invalid_json_lines(self, tmp_path: Path) -> None:
        path = tmp_path / "otel.jsonl"
        lines = [
            '{"trace_id": "t1", "body": "ok"}',
            "not json at all",
            '{"trace_id": "t2", "body": "ok2"}',
        ]
        path.write_text("\n".join(lines) + "\n", encoding="utf-8")
        diag = ParserDiagnostics()
        records = parse_otel_jsonl(path, diagnostics=diag)

        assert len(records) == 2
        assert diag.skipped_lines == 1
        assert "INVALID_OTEL_JSON" in diag.skipped_by_reason

    def test_skips_json_without_otel_fields(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [
                {"unrelated_key": "value", "another": 42},
                {"trace_id": "t1", "body": "valid"},
            ],
        )
        diag = ParserDiagnostics()
        records = parse_otel_jsonl(path, diagnostics=diag)

        assert len(records) == 1
        assert diag.skipped_lines == 1
        assert "INVALID_OTEL_RECORD" in diag.skipped_by_reason

    def test_skips_non_dict_json_values(self, tmp_path: Path) -> None:
        path = tmp_path / "otel.jsonl"
        path.write_text(
            '"just a string"\n[1,2,3]\n{"trace_id":"t1","body":"ok"}\n',
            encoding="utf-8",
        )
        diag = ParserDiagnostics()
        records = parse_otel_jsonl(path, diagnostics=diag)

        assert len(records) == 1
        assert diag.skipped_lines == 2

    def test_blank_lines_are_ignored(self, tmp_path: Path) -> None:
        path = tmp_path / "otel.jsonl"
        path.write_text(
            '\n{"trace_id":"t1","body":"ok"}\n\n\n{"trace_id":"t2","body":"ok2"}\n',
            encoding="utf-8",
        )
        diag = ParserDiagnostics()
        records = parse_otel_jsonl(path, diagnostics=diag)

        assert len(records) == 2
        assert diag.skipped_lines == 0

    def test_severity_defaults_to_unspecified(self, tmp_path: Path) -> None:
        path = _write_jsonl(
            tmp_path,
            [{"trace_id": "t1", "body": "no severity"}],
        )
        records = parse_otel_jsonl(path)
        assert records[0].severity == "UNSPECIFIED"
