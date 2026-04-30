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
