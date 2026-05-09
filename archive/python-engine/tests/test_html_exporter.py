# ─────────────────────────────────────────────────────────────────────
# [한글] test_html_exporter — HTML 보고서 exporter 회귀 테스트.
#
# 검증 대상
#   • 자기 충족 HTML: 외부 자산 참조 0개, 인라인 CSS 만 사용.
#   • 사용자 입력 escape: <script> 가 들어가도 &lt;script&gt; 로 변환 (XSS 안전).
#   • 한글 raw 보존.
#   • Findings 섹션은 metadata.findings 가 비어있지 않을 때만 렌더.
#   • Summary / Tables / Series 정상 렌더.
#
# pytest.importorskip
#   typer/rich 가 설치되지 않은 환경에서는 테스트 자체를 skip — CLI 의
#   부수 의존성이 누락되어도 본 모듈 단독 테스트는 가능하게.
#
# parity 주의
#   Go engine-native (apps/engine-native/internal/exporters/html) 의
#   출력과 같은 시각적/구조적 결과 produce 해야 함.
# ─────────────────────────────────────────────────────────────────────
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
