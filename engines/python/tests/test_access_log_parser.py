from pathlib import Path

from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
from archscope_engine.parsers.access_log_parser import (
    parse_access_log_with_diagnostics,
    parse_nginx_access_line,
)


def test_parse_nginx_access_line_with_response_time() -> None:
    line = (
        '127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] '
        '"GET /api/orders/1001 HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.123'
    )

    record = parse_nginx_access_line(line)

    assert record is not None
    assert record.method == "GET"
    assert record.uri == "/api/orders/1001"
    assert record.status == 200
    assert record.response_time_ms == 123.0


def test_analyze_access_log_sample() -> None:
    sample = Path(__file__).parents[3] / "examples/access-logs/sample-nginx-access.log"

    result = analyze_access_log(sample)

    assert result.type == "access_log"
    assert result.summary["total_requests"] == 6
    assert result.summary["error_rate"] == 33.33


def test_parse_access_log_reports_malformed_line_diagnostics(tmp_path) -> None:
    valid_line = (
        '127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] '
        '"GET /api/orders/1001 HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.123'
    )
    invalid_timestamp_line = (
        '127.0.0.1 - - [bad-time] '
        '"GET /api/orders/1002 HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.123'
    )
    invalid_number_line = (
        '127.0.0.1 - - [27/Apr/2026:10:00:02 +0900] '
        '"GET /api/orders/1003 HTTP/1.1" abc 1234 "-" "Mozilla/5.0" 0.123'
    )
    no_format_line = "not an nginx access log record"
    path = tmp_path / "access.log"
    path.write_text(
        "\n".join(
            [
                valid_line,
                "",
                invalid_timestamp_line,
                invalid_number_line,
                no_format_line,
            ]
        ),
        encoding="utf-8",
    )

    result = parse_access_log_with_diagnostics(path)

    assert len(result.records) == 1
    assert result.diagnostics == {
        "total_lines": 5,
        "parsed_records": 1,
        "skipped_lines": 3,
        "skipped_by_reason": {
            "INVALID_TIMESTAMP": 1,
            "INVALID_NUMBER": 1,
            "NO_FORMAT_MATCH": 1,
        },
        "samples": [
            {
                "line_number": 3,
                "reason": "INVALID_TIMESTAMP",
                "message": "Timestamp does not match nginx format.",
                "raw_preview": invalid_timestamp_line,
            },
            {
                "line_number": 4,
                "reason": "INVALID_NUMBER",
                "message": "Numeric field could not be parsed.",
                "raw_preview": invalid_number_line,
            },
            {
                "line_number": 5,
                "reason": "NO_FORMAT_MATCH",
                "message": "Line does not match nginx access log format.",
                "raw_preview": no_format_line,
            },
        ],
    }


def test_analyze_access_log_includes_diagnostics_metadata(tmp_path) -> None:
    valid_line = (
        '127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] '
        '"GET /api/orders/1001 HTTP/1.1" 200 1234 "-" "Mozilla/5.0" 0.123'
    )
    path = tmp_path / "access.log"
    path.write_text(f"{valid_line}\nmalformed\n", encoding="utf-8")

    result = analyze_access_log(path)

    assert result.summary["total_requests"] == 1
    assert result.metadata["diagnostics"]["total_lines"] == 2
    assert result.metadata["diagnostics"]["parsed_records"] == 1
    assert result.metadata["diagnostics"]["skipped_lines"] == 1
    assert result.metadata["diagnostics"]["skipped_by_reason"] == {
        "NO_FORMAT_MATCH": 1
    }
