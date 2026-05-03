import json
from pathlib import Path

from archscope_engine.analyzers.access_log_analyzer import analyze_access_log
from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.redaction import redact_text


def nginx_line(
    *,
    uri: str = "/api/orders/1001",
    status: str = "200",
    response_time_sec: str = "0.123",
) -> str:
    return (
        "127.0.0.1 - - [27/Apr/2026:10:00:01 +0900] "
        f'"GET {uri} HTTP/1.1" {status} 1234 "-" "Mozilla/5.0" {response_time_sec}'
    )


def test_clean_analysis_does_not_write_debug_log_without_force(tmp_path) -> None:
    path = tmp_path / "access.log"
    path.write_text(nginx_line(), encoding="utf-8")
    collector = _collector(path, tmp_path, force=False)

    result = analyze_access_log(path, debug_log=collector)
    debug_path = collector.write(diagnostics=result.metadata["diagnostics"])

    assert debug_path is None


def test_force_writes_clean_debug_log(tmp_path) -> None:
    path = tmp_path / "access.log"
    path.write_text(nginx_line(), encoding="utf-8")
    collector = _collector(path, tmp_path, force=True)

    result = analyze_access_log(path, debug_log=collector)
    debug_path = collector.write(diagnostics=result.metadata["diagnostics"])

    assert debug_path is not None
    payload = json.loads(debug_path.read_text(encoding="utf-8"))
    assert payload["summary"]["verdict"] == "CLEAN"
    assert payload["summary"]["skipped"] == 0


def test_debug_log_redacts_sensitive_values_but_preserves_shapes(tmp_path) -> None:
    bad_line = (
        '10.0.0.1 - admin [27/Apr/2026:10:00:01 +0900] '
        '"GET /api/order/12345?token=abcd HTTP/2" 200 512 "-" "curl/8.1" rt=0.002'
    )
    path = tmp_path / "access.log"
    path.write_text("\n".join([nginx_line(), bad_line, nginx_line(uri="/after")]), encoding="utf-8")
    collector = _collector(path, tmp_path, force=False)

    result = analyze_access_log(path, debug_log=collector)
    debug_path = collector.write(diagnostics=result.metadata["diagnostics"])

    assert debug_path is not None
    payload = json.loads(debug_path.read_text(encoding="utf-8"))
    sample = payload["errors_by_type"]["INVALID_NUMBER"]["samples"][0]
    target = sample["raw_context"]["target"]
    assert "10.0.0.1" not in target
    assert "abcd" not in target
    assert "<IPV4>" in target
    assert "<TOKEN len=4>" in target
    assert sample["field_shapes"]["request_shape"] == "METHOD PATH_WITH_QUERY PROTOCOL"
    assert sample["field_shapes"]["query_keys"] == ["token"]


def test_debug_log_limits_samples_per_error_type(tmp_path) -> None:
    path = tmp_path / "access.log"
    path.write_text("\n".join(f"bad line {index}" for index in range(6)), encoding="utf-8")
    collector = _collector(path, tmp_path, force=False)

    result = analyze_access_log(path, debug_log=collector)
    debug_path = collector.write(diagnostics=result.metadata["diagnostics"])

    assert debug_path is not None
    payload = json.loads(debug_path.read_text(encoding="utf-8"))
    assert payload["errors_by_type"]["NO_FORMAT_MATCH"]["count"] == 6
    assert len(payload["errors_by_type"]["NO_FORMAT_MATCH"]["samples"]) == 5
    assert payload["summary"]["verdict"] == "MAJORITY_FAILED"


def test_redaction_masks_common_secret_shapes() -> None:
    result = redact_text(
        "Authorization: Bearer abcdef user@example.com /Users/acme/prod/access.log"
    )

    assert "abcdef" not in result.text
    assert "user@example.com" not in result.text
    assert "/Users/acme/prod" not in result.text
    assert "<TOKEN len=6>" in result.text
    assert "<EMAIL>" in result.text
    assert "<PATH>/access.log" in result.text


def test_debug_log_size_cap_keeps_valid_json(tmp_path) -> None:
    path = tmp_path / "access.log"
    path.write_text("bad", encoding="utf-8")
    collector = _collector(path, tmp_path, force=True)
    collector.add_exception(
        phase="analysis",
        exception=ValueError("x" * 2_000_000),
    )

    debug_path = collector.write()

    assert debug_path is not None
    assert debug_path.stat().st_size <= 1_000_000
    payload = json.loads(debug_path.read_text(encoding="utf-8"))
    assert payload["truncated"] is True


def _collector(path: Path, debug_dir: Path, *, force: bool) -> DebugLogCollector:
    return DebugLogCollector(
        analyzer_type="access_log",
        source_file=path,
        parser="nginx_access_log",
        parser_options={"format": "nginx"},
        debug_log_dir=debug_dir,
        force_write=force,
    )
