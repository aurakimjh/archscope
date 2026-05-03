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
