import json
import subprocess
import sys
from pathlib import Path

import pytest

from archscope_engine.analyzers.jfr_analyzer import analyze_jfr_print_json
from archscope_engine.parsers.jfr_parser import parse_jfr_print_json

pytest.importorskip("typer")
pytest.importorskip("rich")


def test_parse_jfr_print_json_extracts_events() -> None:
    sample = Path(__file__).parents[3] / "examples/jfr/sample-jfr-print.json"

    events = parse_jfr_print_json(sample)

    assert events[0]["event_type"] == "jdk.CPUTimeSample"
    assert events[0]["frames"] == ["com.example.checkout.PaymentService.authorize"]
    assert events[1]["duration_ms"] == 42.5


def test_analyze_jfr_print_json_builds_analysis_result() -> None:
    sample = Path(__file__).parents[3] / "examples/jfr/sample-jfr-print.json"

    result = analyze_jfr_print_json(sample)

    assert result.type == "jfr_recording"
    assert result.summary["event_count"] == 3
    assert result.summary["gc_pause_total_ms"] == 42.5
    assert result.summary["blocked_thread_events"] == 1
    assert result.tables["notable_events"][0]["evidence_ref"] == "jfr:event:1"
    assert result.metadata["schema_version"] == "0.1.0"
    assert result.metadata["poc"] is True


def test_jfr_cli_writes_analysis_result_json(tmp_path) -> None:
    sample = Path(__file__).parents[3] / "examples/jfr/sample-jfr-print.json"
    output = tmp_path / "jfr-result.json"

    completed = subprocess.run(
        [
            sys.executable,
            "-m",
            "archscope_engine.cli",
            "jfr",
            "analyze-json",
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
    assert payload["type"] == "jfr_recording"
    assert payload["summary"]["event_count"] == 3
