import json
import subprocess
import sys
from pathlib import Path

import pytest

pytest.importorskip("typer")
pytest.importorskip("rich")


def test_access_log_cli_writes_analysis_result_json(tmp_path) -> None:
    sample = Path(__file__).parents[3] / "examples/access-logs/sample-nginx-access.log"
    output = tmp_path / "access-log-result.json"

    completed = subprocess.run(
        [
            sys.executable,
            "-m",
            "archscope_engine.cli",
            "access-log",
            "analyze",
            "--file",
            str(sample),
            "--format",
            "nginx",
            "--out",
            str(output),
        ],
        check=True,
        capture_output=True,
        text=True,
    )

    payload = json.loads(output.read_text(encoding="utf-8"))
    assert completed.returncode == 0
    assert payload["type"] == "access_log"
    assert payload["summary"]["total_requests"] == 6


def test_profiler_cli_writes_analysis_result_json(tmp_path) -> None:
    sample = Path(__file__).parents[3] / "examples/profiler/sample-wall.collapsed"
    output = tmp_path / "profiler-result.json"

    completed = subprocess.run(
        [
            sys.executable,
            "-m",
            "archscope_engine.cli",
            "profiler",
            "analyze-collapsed",
            "--wall",
            str(sample),
            "--wall-interval-ms",
            "100",
            "--elapsed-sec",
            "1336.559",
            "--out",
            str(output),
        ],
        check=True,
        capture_output=True,
        text=True,
    )

    payload = json.loads(output.read_text(encoding="utf-8"))
    assert completed.returncode == 0
    assert payload["type"] == "profiler_collapsed"
    assert payload["summary"]["total_samples"] == 32629


def test_profiler_jennifer_csv_cli_writes_analysis_result_json(tmp_path) -> None:
    sample = Path(__file__).parents[3] / "examples/profiler/sample-jennifer-flame.csv"
    output = tmp_path / "jennifer-result.json"

    completed = subprocess.run(
        [
            sys.executable,
            "-m",
            "archscope_engine.cli",
            "profiler",
            "analyze-jennifer-csv",
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
    assert payload["type"] == "profiler_collapsed"
    assert payload["metadata"]["parser"] == "jennifer_flamegraph_csv"
    assert payload["charts"]["flamegraph"]["name"] == "All"


def test_profiler_drilldown_and_breakdown_cli_write_json(tmp_path) -> None:
    sample = Path(__file__).parents[3] / "examples/profiler/sample-wall.collapsed"
    drilldown_output = tmp_path / "drilldown-result.json"
    breakdown_output = tmp_path / "breakdown-result.json"

    subprocess.run(
        [
            sys.executable,
            "-m",
            "archscope_engine.cli",
            "profiler",
            "drilldown",
            "--wall",
            str(sample),
            "--filter",
            "org.springframework",
            "--out",
            str(drilldown_output),
        ],
        check=True,
        capture_output=True,
        text=True,
    )
    subprocess.run(
        [
            sys.executable,
            "-m",
            "archscope_engine.cli",
            "profiler",
            "breakdown",
            "--wall",
            str(sample),
            "--filter",
            "org.springframework",
            "--out",
            str(breakdown_output),
        ],
        check=True,
        capture_output=True,
        text=True,
    )

    drilldown_payload = json.loads(drilldown_output.read_text(encoding="utf-8"))
    breakdown_payload = json.loads(breakdown_output.read_text(encoding="utf-8"))
    assert len(drilldown_payload["charts"]["drilldown_stages"]) == 2
    assert "execution_breakdown" in breakdown_payload["series"]


def test_jvm_analyzer_clis_write_analysis_result_json(tmp_path) -> None:
    repo_root = Path(__file__).parents[3]
    cases = [
        (
            ["gc-log", "analyze"],
            repo_root / "examples/gc-logs/sample-hotspot-gc.log",
            "gc_log",
            "total_events",
        ),
        (
            ["thread-dump", "analyze"],
            repo_root / "examples/thread-dumps/sample-java-thread-dump.txt",
            "thread_dump",
            "total_threads",
        ),
        (
            ["exception", "analyze"],
            repo_root / "examples/exceptions/sample-java-exception.txt",
            "exception_stack",
            "total_exceptions",
        ),
    ]

    for command, sample, result_type, summary_key in cases:
        output = tmp_path / f"{result_type}.json"
        completed = subprocess.run(
            [
                sys.executable,
                "-m",
                "archscope_engine.cli",
                *command,
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
        assert payload["type"] == result_type
        assert payload["summary"][summary_key] > 0


def test_access_log_cli_writes_debug_log_for_malformed_records(tmp_path) -> None:
    sample = tmp_path / "access.log"
    output = tmp_path / "access-result.json"
    debug_dir = tmp_path / "debug"
    sample.write_text(
        "\n".join(
            [
                '127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] '
                '"GET /ok HTTP/1.1" 200 123 "-" "curl/8.1" 0.001',
                '10.0.0.1 - admin [27/Apr/2026:10:00:01 +0900] '
                '"GET /api/order/12345?token=abcd HTTP/2" 200 512 "-" "curl/8.1" rt=0.002',
                '127.0.0.1 - - [27/Apr/2026:10:00:02 +0900] '
                '"GET /after HTTP/1.1" 200 123 "-" "curl/8.1" 0.001',
            ]
        ),
        encoding="utf-8",
    )

    subprocess.run(
        [
            sys.executable,
            "-m",
            "archscope_engine.cli",
            "access-log",
            "analyze",
            "--file",
            str(sample),
            "--format",
            "nginx",
            "--out",
            str(output),
            "--debug-log-dir",
            str(debug_dir),
        ],
        check=True,
        capture_output=True,
        text=True,
    )

    debug_logs = list(debug_dir.glob("archscope-debug-access_log-*.json"))
    assert len(debug_logs) == 1
    payload = json.loads(debug_logs[0].read_text(encoding="utf-8"))
    target = payload["errors_by_type"]["INVALID_NUMBER"]["samples"][0]["raw_context"]["target"]
    assert payload["summary"]["verdict"] == "PARTIAL_SUCCESS"
    assert "abcd" not in target
    assert "<TOKEN len=4>" in target


def test_jfr_cli_writes_debug_log_on_fatal_parser_error(tmp_path) -> None:
    sample = tmp_path / "bad-jfr.json"
    output = tmp_path / "jfr-result.json"
    debug_dir = tmp_path / "debug"
    sample.write_text('{"recording": ', encoding="utf-8")

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
            "--debug-log-dir",
            str(debug_dir),
        ],
        capture_output=True,
        text=True,
    )

    assert completed.returncode != 0
    debug_logs = list(debug_dir.glob("archscope-debug-jfr_recording-*.json"))
    assert len(debug_logs) == 1
    payload = json.loads(debug_logs[0].read_text(encoding="utf-8"))
    assert payload["summary"]["verdict"] == "FATAL_ERROR"
    assert payload["exceptions"][0]["exception_type"] == "JSONDecodeError"
