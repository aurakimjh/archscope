import json
import subprocess
import sys
from pathlib import Path

import pytest

from archscope_engine.analyzers.runtime_analyzer import (
    analyze_dotnet_exception_iis,
    analyze_go_panic,
    analyze_nodejs_stack,
    analyze_python_traceback,
)

pytest.importorskip("typer")
pytest.importorskip("rich")


def test_nodejs_stack_analyzer_parses_error_blocks() -> None:
    sample = Path(__file__).parents[3] / "examples/runtime/sample-nodejs-stack.txt"

    result = analyze_nodejs_stack(sample).to_dict()

    assert result["type"] == "nodejs_stack"
    assert result["summary"]["total_records"] == 2
    assert result["series"]["record_type_distribution"][0]["record_type"] == "TypeError"


def test_python_traceback_analyzer_parses_tracebacks() -> None:
    sample = Path(__file__).parents[3] / "examples/runtime/sample-python-traceback.txt"

    result = analyze_python_traceback(sample).to_dict()

    assert result["type"] == "python_traceback"
    assert result["summary"]["total_records"] == 2
    assert result["summary"]["top_record_type"] == "TimeoutError"


def test_go_panic_analyzer_parses_panic_and_goroutines() -> None:
    sample = Path(__file__).parents[3] / "examples/runtime/sample-go-panic.txt"

    result = analyze_go_panic(sample).to_dict()

    assert result["type"] == "go_panic"
    assert result["summary"]["total_records"] == 3
    assert result["series"]["record_type_distribution"][0]["record_type"] == "goroutine"


def test_dotnet_analyzer_parses_exception_and_iis_records() -> None:
    sample = Path(__file__).parents[3] / "examples/runtime/sample-dotnet-iis.txt"

    result = analyze_dotnet_exception_iis(sample).to_dict()

    assert result["type"] == "dotnet_exception_iis"
    assert result["summary"]["total_exceptions"] == 1
    assert result["summary"]["iis_requests"] == 3
    assert result["summary"]["iis_error_requests"] == 1


def test_runtime_clis_write_analysis_result_json(tmp_path) -> None:
    repo_root = Path(__file__).parents[3]
    cases = [
        ("nodejs", repo_root / "examples/runtime/sample-nodejs-stack.txt", "nodejs_stack"),
        (
            "python-traceback",
            repo_root / "examples/runtime/sample-python-traceback.txt",
            "python_traceback",
        ),
        ("go-panic", repo_root / "examples/runtime/sample-go-panic.txt", "go_panic"),
        (
            "dotnet",
            repo_root / "examples/runtime/sample-dotnet-iis.txt",
            "dotnet_exception_iis",
        ),
    ]

    for command, sample, result_type in cases:
        output = tmp_path / f"{result_type}.json"
        completed = subprocess.run(
            [
                sys.executable,
                "-m",
                "archscope_engine.cli",
                command,
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
        assert payload["type"] == result_type
