# ─────────────────────────────────────────────────────────────────────
# [한글] exporters 패키지 — AnalysisResult → 외부 포맷 변환.
#
# 모듈 별 책임
#   - json_exporter      : JSON (canonical, frontend / API).
#   - html_exporter      : self-contained HTML 리포트.
#   - csv_exporter       : 표 데이터를 CSV 로.
#   - pptx_exporter      : python-pptx 기반 임원 리포트.
#   - pprof_exporter     : Google pprof 형식 (profiler 만).
#   - report_diff        : 두 AnalysisResult 비교 리포트.
#
# parity 주의: Go engine-native 의 internal/exporters 와 동일 출력.
# ─────────────────────────────────────────────────────────────────────
from archscope_engine.exporters.json_exporter import write_json_result

__all__ = ["write_json_result"]
