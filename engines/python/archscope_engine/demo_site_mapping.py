from __future__ import annotations

import json
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass(frozen=True)
class AnalyzerTypeMapping:
    analyzer_type: str
    command: tuple[str, ...] | None
    input_option: str | None
    format_overrides: dict[str, "AnalyzerTypeMapping"] = field(default_factory=dict)
    note: str | None = None


def load_analyzer_type_mappings(anchor: Path) -> dict[str, AnalyzerTypeMapping]:
    mapping_path = find_analyzer_type_mapping(anchor)
    payload = json.loads(mapping_path.read_text(encoding="utf-8"))
    mappings = payload.get("mappings")
    if not isinstance(mappings, dict):
        raise ValueError(f"Invalid analyzer type mapping file: {mapping_path}")
    return {
        analyzer_type: _mapping_from_payload(analyzer_type, value)
        for analyzer_type, value in mappings.items()
        if isinstance(analyzer_type, str) and isinstance(value, dict)
    }


def find_analyzer_type_mapping(anchor: Path) -> Path:
    candidates = []
    if anchor.is_file():
        candidates.append(anchor.parent / "analyzer_type_mapping.json")
        candidates.extend(parent / "analyzer_type_mapping.json" for parent in anchor.parents)
    else:
        candidates.append(anchor / "analyzer_type_mapping.json")
        candidates.extend(parent / "analyzer_type_mapping.json" for parent in anchor.parents)
    for candidate in candidates:
        if candidate.exists():
            return candidate
    raise FileNotFoundError(
        "demo-site analyzer_type_mapping.json was not found near the manifest root."
    )


def command_for_mapping(
    mapping: AnalyzerTypeMapping,
    *,
    file_format: str | None = None,
) -> tuple[str, ...] | None:
    if file_format and file_format in mapping.format_overrides:
        return mapping.format_overrides[file_format].command
    return mapping.command


def input_option_for_mapping(
    mapping: AnalyzerTypeMapping,
    *,
    file_format: str | None = None,
) -> str | None:
    if file_format and file_format in mapping.format_overrides:
        return mapping.format_overrides[file_format].input_option
    return mapping.input_option


def _mapping_from_payload(
    analyzer_type: str,
    payload: dict[str, Any],
) -> AnalyzerTypeMapping:
    command = payload.get("command")
    input_option = payload.get("input_option")
    overrides = payload.get("format_overrides")
    return AnalyzerTypeMapping(
        analyzer_type=analyzer_type,
        command=tuple(command) if _string_list(command) else None,
        input_option=input_option if isinstance(input_option, str) else None,
        format_overrides={
            name: _mapping_from_payload(analyzer_type, value)
            for name, value in overrides.items()
            if isinstance(name, str) and isinstance(value, dict)
        }
        if isinstance(overrides, dict)
        else {},
        note=payload.get("note") if isinstance(payload.get("note"), str) else None,
    )


def _string_list(value: Any) -> bool:
    return isinstance(value, list) and all(isinstance(item, str) for item in value)
