# ─────────────────────────────────────────────────────────────────────
# [한글] test_json_exporter — JSON exporter 회귀 테스트.
#
# 검증 대상
#   • write_json_result: 부모 디렉토리 자동 생성 (mkdir -p) + 2-space
#     indent + trailing newline.
#   • AnalysisResult round-trip: write → read → 같은 데이터.
#   • UTF-8 raw 출력 (ensure_ascii=False 동치) — 한글이 escape 없이 보존.
#
# parity 주의
#   Go engine-native (apps/engine-native/internal/exporters/json) 와
#   byte 단위 일치. 같은 입력에 대한 출력 파일이 두 엔진에서 같아야 함.
# ─────────────────────────────────────────────────────────────────────
import json

from archscope_engine.exporters.json_exporter import write_json_result
from archscope_engine.models.analysis_result import AnalysisResult


def test_write_json_result_round_trips_analysis_result(tmp_path) -> None:
    output_path = tmp_path / "nested" / "result.json"
    result = AnalysisResult(
        type="access_log",
        source_files=["sample.log"],
        summary={"total_requests": 1, "avg_response_ms": 123.4},
        series={
            "requests_per_minute": [
                {"time": "2026-04-29T10:00:00+0900", "value": 1}
            ]
        },
        tables={"sample_records": [{"uri": "/api/orders", "status": 200}]},
        metadata={"parser": "nginx_combined_with_response_time", "label": "한글"},
    )

    write_json_result(result, output_path)

    payload = json.loads(output_path.read_text(encoding="utf-8"))
    assert payload == result.to_dict()
    assert output_path.read_text(encoding="utf-8").endswith("\n")


def test_write_json_result_round_trips_plain_dict(tmp_path) -> None:
    output_path = tmp_path / "result.json"
    payload = {"type": "custom", "metadata": {"schema_version": "0.1.0"}}

    write_json_result(payload, output_path)

    assert json.loads(output_path.read_text(encoding="utf-8")) == payload
