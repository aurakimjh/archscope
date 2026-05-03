import json
from pathlib import Path

import pytest

from archscope_engine.exporters.html_exporter import write_html_report

pytest.importorskip("typer")
pytest.importorskip("rich")


def test_html_report_exporter_renders_analysis_result(tmp_path) -> None:
    source = Path(__file__).parents[3] / "examples/outputs/access-log-result.json"
    output = tmp_path / "access-report.html"

    write_html_report(source, output)

    html = output.read_text(encoding="utf-8")
    assert "ArchScope Analysis Report - access_log" in html
    assert "Summary" in html
    assert "top_urls_by_avg_response_time" in html


def test_html_report_exporter_renders_parser_debug_log(tmp_path) -> None:
    source = tmp_path / "debug.json"
    output = tmp_path / "debug.html"
    source.write_text(
        json.dumps(
            {
                "environment": {"archscope_version": "0.1.0"},
                "context": {
                    "analyzer_type": "access_log",
                    "parser": "nginx",
                    "source_file_name": "access.log",
                    "encoding_detected": "utf-8",
                },
                "redaction": {"enabled": True},
                "summary": {"verdict": "PARTIAL_SUCCESS", "skipped": 1},
                "errors_by_type": {
                    "INVALID_NUMBER": {
                        "count": 1,
                        "description": "bad response time",
                        "failed_pattern": "ACCESS_LOG",
                        "samples": [{"line_number": 2, "message": "bad"}],
                    }
                },
                "exceptions": [],
                "hints": ["Check response-time suffix."],
            }
        ),
        encoding="utf-8",
    )

    write_html_report(source, output)

    html = output.read_text(encoding="utf-8")
    assert "ArchScope Parser Debug Report - access_log" in html
    assert "INVALID_NUMBER" in html
