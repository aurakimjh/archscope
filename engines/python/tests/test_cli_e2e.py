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
