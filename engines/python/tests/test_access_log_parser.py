from pathlib import Path

from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
from archscope_engine.parsers.access_log_parser import parse_nginx_access_line


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
