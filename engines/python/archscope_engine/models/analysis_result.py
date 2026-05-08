"""Top-level analyzer response container."""
# ─────────────────────────────────────────────────────────────────────
# [한글] analysis_result — 모든 분석기의 공통 응답 컨테이너.
#
# 필드 의미
#   type         : 분석 종류 라벨 ("access_log", "thread_dump_multi",
#                  "gc_log", "profiler_collapsed" 등).
#   source_files : 입력 파일 경로 (str) 의 리스트.
#   summary      : 요약 카운터/지표 (분석기별 자유로운 dict).
#   series       : 시계열/분포 (시각화용).
#   tables       : 표 데이터 (top_n cap 적용된 상태).
#   charts       : 별도 차트 데이터 (선택적, 거의 미사용).
#   metadata     : parser 이름, schema_version, diagnostics, findings.
#   created_at   : 응답 생성 시간 (ISO8601 UTC).
#
# parity 주의사항: Go engine-native 의 internal/models/analysis_result.go
# 와 동일한 필드 이름. JSON serialize 결과가 byte 일치하기 위해
# field 순서도 유지.
# ─────────────────────────────────────────────────────────────────────
from __future__ import annotations

from dataclasses import asdict, dataclass, field
from datetime import datetime, timezone
from typing import Any


@dataclass(frozen=True)
class AnalysisResult:
    type: str
    source_files: list[str]
    summary: dict[str, Any] = field(default_factory=dict)
    series: dict[str, Any] = field(default_factory=dict)
    tables: dict[str, Any] = field(default_factory=dict)
    charts: dict[str, Any] = field(default_factory=dict)
    metadata: dict[str, Any] = field(default_factory=dict)
    created_at: str = field(
        default_factory=lambda: datetime.now(timezone.utc).isoformat()
    )

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)
