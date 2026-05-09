# ─────────────────────────────────────────────────────────────────────
# [한글] test_access_log_parser — access_log_parser 단위 회귀 테스트.
#
# 검증 대상
#   • parse_nginx_access_line : nginx-combined-with-response-time 라인의
#     method/uri/status/response_time_ms (sec → ms 변환) 정확도.
#   • parse_access_log_with_diagnostics — diagnostics 의 모든 필드:
#       - source_file / format("nginx") / warning_count / error_count.
#       - errors == samples (참조 일치).
#       - skipped_by_reason 카운트:
#           INVALID_TIMESTAMP / INVALID_NUMBER / NO_FORMAT_MATCH.
#       - samples 배열은 line_number / reason / message / raw_preview
#         4 필드를 정확히 그 순서로 보존.
#   • common(log_format="common") 분기: bytes "-" → 0, response_time
#     필드 없을 때 0.0 으로 fallback.
#   • strict=True : 첫 NO_FORMAT_MATCH 라인에서 ValueError("NO_FORMAT_MATCH").
#
# fixture 정책
#   nginx_line() 헬퍼로 정상 라인 1 종 + 비정상 timestamp / number /
#   format 케이스를 같은 파일에 묶어 multi-error 분기까지 확인.
#
# parity 주의 (Python ↔ Go 비교 가능한 부분)
#   Go 측 internal/parsers/accesslog/parser_test.go 와 입력 라인 / skip
#   reason 코드 / sample 필드명 / strict mode 메시지 byte 단위 동일.
# ─────────────────────────────────────────────────────────────────────
import pytest

from archscope_engine.parsers.access_log_parser import (
    parse_access_log_with_diagnostics,
    parse_nginx_access_line,
)


def nginx_line(
    *,
    timestamp: str = "27/Apr/2026:10:00:01 +0900",
    uri: str = "/api/orders/1001",
    status: str = "200",
    response_time_sec: str = "0.123",
) -> str:
    return (
        f"127.0.0.1 - - [{timestamp}] "
        f'"GET {uri} HTTP/1.1" {status} 1234 "-" "Mozilla/5.0" {response_time_sec}'
    )


def test_parse_nginx_access_line_with_response_time() -> None:
    line = nginx_line()

    record = parse_nginx_access_line(line)

    assert record is not None
    assert record.method == "GET"
    assert record.uri == "/api/orders/1001"
    assert record.status == 200
    assert record.response_time_ms == 123.0


def test_parse_access_log_reports_malformed_line_diagnostics(tmp_path) -> None:
    valid_line = nginx_line()
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
    assert result.diagnostics["source_file"] == str(path)
    assert result.diagnostics["format"] == "nginx"
    assert result.diagnostics["warning_count"] == 0
    assert result.diagnostics["error_count"] == 3
    assert result.diagnostics["errors"] == result.diagnostics["samples"]
    assert {
        key: result.diagnostics[key]
        for key in (
            "total_lines",
            "parsed_records",
            "skipped_lines",
            "skipped_by_reason",
            "samples",
        )
    } == {
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
                "message": "Line does not match supported access log formats.",
                "raw_preview": no_format_line,
            },
        ],
    }


def test_parse_common_access_log_without_response_time(tmp_path) -> None:
    path = tmp_path / "common.log"
    path.write_text(
        '127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] '
        '"GET /health HTTP/1.1" 204 -\n',
        encoding="utf-8",
    )

    result = parse_access_log_with_diagnostics(path, log_format="common")

    assert len(result.records) == 1
    assert result.records[0].uri == "/health"
    assert result.records[0].status == 204
    assert result.records[0].bytes_sent == 0
    assert result.records[0].response_time_ms == 0.0


def test_parse_access_log_strict_mode_fails_fast(tmp_path) -> None:
    path = tmp_path / "access.log"
    path.write_text("bad access log\n" + nginx_line(), encoding="utf-8")

    with pytest.raises(ValueError, match="NO_FORMAT_MATCH"):
        parse_access_log_with_diagnostics(path, strict=True)
