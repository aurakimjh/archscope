import json
import subprocess
import sys
from pathlib import Path

import pytest

from archscope_engine.analyzers.otel_analyzer import analyze_otel_jsonl

pytest.importorskip("typer")
pytest.importorskip("rich")


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


def test_otel_cli_writes_analysis_result_json(tmp_path) -> None:
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
