"""JSON exporter (canonical output format)."""
# [한글] json_exporter — AnalysisResult → JSON 파일.
# ensure_ascii=False, indent=2 로 사람이 읽기 좋은 UTF-8 출력.
# 끝에 "\n" 추가 (POSIX 텍스트 파일 관습).
# parity: Go engine-native internal/exporters/json 의 출력과 byte 일치.
from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from archscope_engine.models.analysis_result import AnalysisResult


def write_json_result(result: AnalysisResult | dict[str, Any], path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = result.to_dict() if isinstance(result, AnalysisResult) else result
    path.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2) + "\n",
        encoding="utf-8",
    )
