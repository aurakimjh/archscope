from datetime import datetime
from pathlib import Path

from archscope_engine.analyzers.access_log_analyzer import analyze_access_log


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


def test_analyze_access_log_sample() -> None:
    sample = Path(__file__).parents[3] / "examples/access-logs/sample-nginx-access.log"

    result = analyze_access_log(sample)

    assert result.type == "access_log"
    assert result.summary["total_requests"] == 6
    assert result.summary["error_rate"] == 33.33


def test_analyze_access_log_includes_diagnostics_metadata(tmp_path) -> None:
    valid_line = nginx_line()
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


def test_analyze_access_log_respects_max_lines(tmp_path) -> None:
    path = tmp_path / "access.log"
    path.write_text(
        "\n".join(
            [
                nginx_line(uri="/one"),
                nginx_line(uri="/two"),
                nginx_line(uri="/three"),
            ]
        ),
        encoding="utf-8",
    )

    result = analyze_access_log(path, max_lines=2)

    assert result.summary["total_requests"] == 2
    assert result.metadata["diagnostics"]["total_lines"] == 2
    assert result.metadata["analysis_options"]["max_lines"] == 2


def test_analyze_access_log_filters_by_time_range(tmp_path) -> None:
    path = tmp_path / "access.log"
    path.write_text(
        "\n".join(
            [
                nginx_line(timestamp="27/Apr/2026:10:00:00 +0900", uri="/early"),
                nginx_line(timestamp="27/Apr/2026:10:01:00 +0900", uri="/included"),
                nginx_line(timestamp="27/Apr/2026:10:02:00 +0900", uri="/late"),
            ]
        ),
        encoding="utf-8",
    )

    result = analyze_access_log(
        path,
        start_time=datetime.fromisoformat("2026-04-27T10:01:00+09:00"),
        end_time=datetime.fromisoformat("2026-04-27T10:01:00+09:00"),
    )

    assert result.summary["total_requests"] == 1
    assert result.series["top_urls_by_count"] == [{"uri": "/included", "count": 1}]
    assert result.metadata["diagnostics"]["total_lines"] == 3
    assert result.metadata["diagnostics"]["parsed_records"] == 1


def test_analyze_access_log_reports_status_and_slow_url_findings(tmp_path) -> None:
    path = tmp_path / "access.log"
    path.write_text(
        "\n".join(
            [
                nginx_line(uri="/fast", status="200", response_time_sec="0.100"),
                nginx_line(uri="/slow-error", status="500", response_time_sec="2.500"),
            ]
        ),
        encoding="utf-8",
    )

    result = analyze_access_log(path)

    finding_codes = {finding["code"] for finding in result.metadata["findings"]}
    assert finding_codes == {
        "HIGH_ERROR_RATE",
        "SERVER_ERRORS_PRESENT",
        "SLOW_URL_AVERAGE",
    }


def test_analyze_access_log_reports_non_standard_http_status(tmp_path) -> None:
    path = tmp_path / "access.log"
    path.write_text(nginx_line(status="999"), encoding="utf-8")

    result = analyze_access_log(path)

    finding = next(
        item
        for item in result.metadata["findings"]
        if item["code"] == "NON_STANDARD_HTTP_STATUS"
    )
    assert finding["evidence"] == {"statuses": "999", "count": 1}


def test_analyze_access_log_handles_more_records_than_percentile_sample_limit(tmp_path) -> None:
    path = tmp_path / "access.log"
    path.write_text(
        "\n".join(
            nginx_line(
                timestamp=f"27/Apr/2026:10:{index % 60:02d}:00 +0900",
                uri=f"/item/{index}",
                response_time_sec=f"{index / 1000:.3f}",
            )
            for index in range(1, 10_050)
        ),
        encoding="utf-8",
    )

    result = analyze_access_log(path)

    assert result.summary["total_requests"] == 10_049
    assert result.summary["p95_response_ms"] > 0
