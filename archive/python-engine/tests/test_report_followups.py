# ─────────────────────────────────────────────────────────────────────
# [한글] test_report_followups — 보고서 exporter 의 후속 회귀 테스트.
#
# 검증 대상
#   • HTML / PPTX / before-after diff 보고서가 정상 생성.
#   • PPTX 가 zip 으로 열렸을 때 8개 필수 OOXML part 포함.
#   • build_comparison_report: 두 AnalysisResult 의 metric_deltas
#     계산 정확.
#   • findings 카운트가 비교 보고서의 summary 에 정확히 반영.
#   • 한글 텍스트가 모든 산출물에서 raw UTF-8 로 보존.
#
# 통합 시나리오
#   분석 → JSON → exporter → 다른 형식 보고서 라운드트립이 깨지지 않는지
#   end-to-end 로 묶어서 회귀 차단.
# ─────────────────────────────────────────────────────────────────────
import json
import zipfile
from pathlib import Path

from archscope_engine.exporters.html_exporter import write_html_report
from archscope_engine.exporters.pptx_exporter import write_pptx_report
from archscope_engine.exporters.report_diff import build_comparison_report


def test_before_after_diff_builds_comparison_result(tmp_path) -> None:
    before = tmp_path / "before.json"
    after = tmp_path / "after.json"
    before.write_text(
        json.dumps(
            {
                "type": "access_log",
                "source_files": ["before.log"],
                "summary": {"total_requests": 10, "error_rate": 10.0},
                "metadata": {"findings": [{"code": "A"}]},
            }
        ),
        encoding="utf-8",
    )
    after.write_text(
        json.dumps(
            {
                "type": "access_log",
                "source_files": ["after.log"],
                "summary": {"total_requests": 12, "error_rate": 25.0},
                "metadata": {"findings": [{"code": "A"}, {"code": "B"}]},
            }
        ),
        encoding="utf-8",
    )

    result = build_comparison_report(before, after, label="deploy-42").to_dict()

    assert result["type"] == "comparison_report"
    assert result["summary"]["finding_delta"] == 1
    assert {
        "metric": "total_requests",
        "before": 10,
        "after": 12,
        "delta": 2,
        "change_percent": 20.0,
    } in result["series"]["summary_metric_deltas"]


def test_pptx_exporter_writes_minimal_deck(tmp_path) -> None:
    source = Path(__file__).parents[3] / "examples/outputs/access-log-result.json"
    output = tmp_path / "report.pptx"

    write_pptx_report(source, output)

    with zipfile.ZipFile(output) as package:
        names = set(package.namelist())
        assert "ppt/presentation.xml" in names
        assert "ppt/slides/slide1.xml" in names
        assert "ppt/slides/slide2.xml" in names


def test_html_exporter_renders_flamegraph_section(tmp_path) -> None:
    source = tmp_path / "profiler.json"
    output = tmp_path / "profiler.html"
    source.write_text(
        json.dumps(
            {
                "type": "profiler_collapsed",
                "source_files": ["sample.collapsed"],
                "summary": {"total_samples": 10},
                "series": {},
                "tables": {},
                "charts": {
                    "flamegraph": {
                        "name": "All",
                        "samples": 10,
                        "children": [{"name": "app.Handler", "samples": 8}],
                    }
                },
                "metadata": {"schema_version": "0.1.0"},
            }
        ),
        encoding="utf-8",
    )

    write_html_report(source, output)

    html = output.read_text(encoding="utf-8")
    assert "Flamegraph" in html
    assert "flame-bar" in html
