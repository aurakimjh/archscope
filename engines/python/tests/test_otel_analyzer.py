import json
import subprocess
import sys
from pathlib import Path

import pytest

from archscope_engine.analyzers.otel_analyzer import analyze_otel_jsonl


def test_otel_jsonl_analyzer_builds_trace_correlation() -> None:
    sample = Path(__file__).parents[3] / "examples/otel/sample-otel-logs.jsonl"

    result = analyze_otel_jsonl(sample).to_dict()

    assert result["type"] == "otel_logs"
    assert result["summary"]["total_records"] == 4
    assert result["summary"]["unique_traces"] == 2
    assert result["summary"]["cross_service_traces"] == 1
    assert result["summary"]["error_records"] == 2
    assert result["summary"]["failed_traces"] == 1
    assert result["tables"]["trace_service_paths"]
    assert result["tables"]["trace_failures"][0]["error_count"] == 2


def test_otel_analyzer_uses_parent_span_path_and_failure_propagation(tmp_path) -> None:
    sample = tmp_path / "otel-parent.jsonl"
    sample.write_text(
        "\n".join(
            [
                json.dumps(
                    {
                        "timestamp": "2026-04-30T10:00:00Z",
                        "trace_id": "trace-1",
                        "span_id": "root",
                        "service_name": "gateway",
                        "severity_text": "INFO",
                        "body": "request started",
                    }
                ),
                json.dumps(
                    {
                        "timestamp": "2026-04-30T10:00:01Z",
                        "trace_id": "trace-1",
                        "span_id": "payment",
                        "parent_span_id": "order",
                        "service_name": "payment",
                        "severity_text": "INFO",
                        "body": "payment call started",
                    }
                ),
                json.dumps(
                    {
                        "timestamp": "2026-04-30T10:00:02Z",
                        "trace_id": "trace-1",
                        "span_id": "order",
                        "parent_span_id": "root",
                        "service_name": "order",
                        "severity_text": "ERROR",
                        "body": "order failed",
                    }
                ),
            ]
        ),
        encoding="utf-8",
    )

    result = analyze_otel_jsonl(sample).to_dict()

    assert result["tables"]["trace_service_paths"][0]["service_path"] == (
        "gateway -> order -> payment"
    )
    assert result["summary"]["failure_propagation_traces"] == 1
    assert result["tables"]["service_failure_propagation"][0]["first_failing_service"] == "order"
    assert result["tables"]["service_failure_propagation"][0]["downstream_services"] == [
        "payment"
    ]
    finding_codes = {finding["code"] for finding in result["metadata"]["findings"]}
    assert "OTEL_FAILURE_PROPAGATION" in finding_codes


def test_otel_analyzer_reports_span_parent_cycles(tmp_path) -> None:
    sample = tmp_path / "otel-cycle.jsonl"
    sample.write_text(
        "\n".join(
            [
                json.dumps(
                    {
                        "timestamp": "2026-04-30T10:00:00Z",
                        "trace_id": "trace-cycle",
                        "span_id": "span-a",
                        "parent_span_id": "span-b",
                        "service_name": "gateway",
                        "severity_text": "INFO",
                        "body": "request started",
                    }
                ),
                json.dumps(
                    {
                        "timestamp": "2026-04-30T10:00:01Z",
                        "trace_id": "trace-cycle",
                        "span_id": "span-b",
                        "parent_span_id": "span-a",
                        "service_name": "order",
                        "severity_text": "INFO",
                        "body": "order started",
                    }
                ),
                json.dumps(
                    {
                        "timestamp": "2026-04-30T10:00:02Z",
                        "trace_id": "trace-self",
                        "span_id": "span-self",
                        "parent_span_id": "span-self",
                        "service_name": "payment",
                        "severity_text": "INFO",
                        "body": "self parent",
                    }
                ),
            ]
        ),
        encoding="utf-8",
    )

    result = analyze_otel_jsonl(sample).to_dict()

    assert result["summary"]["span_topology_warnings"] == 2
    reasons = {
        row["reason"] for row in result["tables"]["span_topology_warnings"]
    }
    assert reasons == {"PARENT_CYCLE", "SELF_PARENT"}
    finding_codes = {finding["code"] for finding in result["metadata"]["findings"]}
    assert "OTEL_SPAN_TOPOLOGY_INCONSISTENT" in finding_codes


def test_otel_error_detection_prioritizes_severity_before_body_keywords(tmp_path) -> None:
    sample = tmp_path / "otel-severity-priority.jsonl"
    sample.write_text(
        "\n".join(
            [
                json.dumps(
                    {
                        "timestamp": "2026-04-30T10:00:00Z",
                        "trace_id": "trace-info",
                        "span_id": "span-info",
                        "service_name": "gateway",
                        "severity_text": "INFO",
                        "body": "error budget check passed",
                    }
                ),
                json.dumps(
                    {
                        "timestamp": "2026-04-30T10:00:01Z",
                        "trace_id": "trace-unspecified",
                        "span_id": "span-unspecified",
                        "service_name": "worker",
                        "body": "job failed before severity was attached",
                    }
                ),
            ]
        ),
        encoding="utf-8",
    )

    result = analyze_otel_jsonl(sample).to_dict()

    assert result["summary"]["failed_traces"] == 1
    assert result["tables"]["trace_service_paths"] == [
        {
            "trace_id": "trace-info",
            "service_path": "gateway",
            "service_count": 1,
            "record_count": 1,
            "has_error": False,
            "max_severity": "INFO",
        },
        {
            "trace_id": "trace-unspecified",
            "service_path": "worker",
            "service_count": 1,
            "record_count": 1,
            "has_error": True,
            "max_severity": "UNSPECIFIED",
        },
    ]


def test_otel_cli_writes_analysis_result_json(tmp_path) -> None:
    pytest.importorskip("typer")
    pytest.importorskip("rich")

    sample = Path(__file__).parents[3] / "examples/otel/sample-otel-logs.jsonl"
    output = tmp_path / "otel-result.json"

    completed = subprocess.run(
        [
            sys.executable,
            "-m",
            "archscope_engine.cli",
            "otel",
            "analyze",
            "--file",
            str(sample),
            "--out",
            str(output),
        ],
        check=True,
        capture_output=True,
        text=True,
    )

    payload = json.loads(output.read_text(encoding="utf-8"))
    assert completed.returncode == 0
    assert payload["type"] == "otel_logs"
