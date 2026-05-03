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
