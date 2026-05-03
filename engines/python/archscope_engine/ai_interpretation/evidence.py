from __future__ import annotations

from dataclasses import dataclass
import re
from typing import Any, Iterable

from archscope_engine.models.analysis_result import AnalysisResult

EVIDENCE_REF_PATTERN = re.compile(
    r"^(?P<source_type>[a-z][a-z0-9_]*):"
    r"(?P<entity_type>[a-z][a-z0-9_]*):"
    r"(?P<identifier>[A-Za-z0-9_.-]+)$"
)

ALLOWED_EVIDENCE_NAMESPACES: dict[str, set[str]] = {
    "access_log": {"record", "finding"},
    "profiler": {"stack", "frame", "finding"},
    "jfr": {"event"},
    "otel": {"log", "span", "event"},
    "timeline": {"event", "correlation"},
}

TEXT_EVIDENCE_FIELDS = (
    "raw_line",
    "raw_block",
    "raw_preview",
    "body_preview",
    "message",
    "summary",
    "stack",
)


@dataclass(frozen=True)
class EvidenceRef:
    source_type: str
    entity_type: str
    identifier: str

    @property
    def value(self) -> str:
        return f"{self.source_type}:{self.entity_type}:{self.identifier}"


@dataclass(frozen=True)
class EvidenceItem:
    ref: str
    source_type: str
    entity_type: str
    identifier: str
    text: str
    source_file: str | None = None
    location: str | None = None
    payload: dict[str, Any] | None = None


class EvidenceRegistry:
    def __init__(self, items: Iterable[EvidenceItem] = ()) -> None:
        self._items: dict[str, EvidenceItem] = {}
        for item in items:
            self.add(item)

    def add(self, item: EvidenceItem) -> None:
        parsed = parse_evidence_ref(item.ref)
        if parsed is None:
            raise ValueError(f"Invalid evidence_ref grammar: {item.ref}")
        if not is_allowed_namespace(parsed):
            raise ValueError(f"Unsupported evidence_ref namespace: {item.ref}")
        self._items[item.ref] = item

    def get(self, ref: str) -> EvidenceItem | None:
        return self._items.get(ref)

    def contains(self, ref: str) -> bool:
        return ref in self._items

    def refs(self) -> set[str]:
        return set(self._items)

    def items(self) -> list[EvidenceItem]:
        return list(self._items.values())


def parse_evidence_ref(value: str) -> EvidenceRef | None:
    if not isinstance(value, str):
        return None
    match = EVIDENCE_REF_PATTERN.match(value.strip())
    if not match:
        return None
    return EvidenceRef(
        source_type=match.group("source_type"),
        entity_type=match.group("entity_type"),
        identifier=match.group("identifier"),
    )


def is_allowed_namespace(ref: EvidenceRef) -> bool:
    return ref.entity_type in ALLOWED_EVIDENCE_NAMESPACES.get(ref.source_type, set())


def collect_evidence(result: AnalysisResult | dict[str, Any]) -> EvidenceRegistry:
    payload = result.to_dict() if isinstance(result, AnalysisResult) else result
    source_files = payload.get("source_files", [])
    default_source_file = (
        source_files[0] if source_files and isinstance(source_files[0], str) else None
    )
    items = _collect_from_value(payload, default_source_file)
    return EvidenceRegistry(items)


def _collect_from_value(value: Any, default_source_file: str | None) -> list[EvidenceItem]:
    if isinstance(value, list):
        items: list[EvidenceItem] = []
        for child in value:
            items.extend(_collect_from_value(child, default_source_file))
        return items

    if not isinstance(value, dict):
        return []

    items = []
    evidence_ref = value.get("evidence_ref")
    if isinstance(evidence_ref, str):
        item = _item_from_payload(evidence_ref, value, default_source_file)
        if item is not None:
            items.append(item)

    for child in value.values():
        items.extend(_collect_from_value(child, default_source_file))

    return items


def _item_from_payload(
    evidence_ref: str,
    payload: dict[str, Any],
    default_source_file: str | None,
) -> EvidenceItem | None:
    parsed = parse_evidence_ref(evidence_ref)
    if parsed is None or not is_allowed_namespace(parsed):
        return None

    text = _evidence_text(payload)
    if not text:
        text = evidence_ref

    source_file = payload.get("source_file")
    if not isinstance(source_file, str):
        source_file = default_source_file

    location = payload.get("line_number") or payload.get("row_id") or payload.get("time")
    return EvidenceItem(
        ref=evidence_ref,
        source_type=parsed.source_type,
        entity_type=parsed.entity_type,
        identifier=parsed.identifier,
        text=text,
        source_file=source_file,
        location=str(location) if location is not None else None,
        payload=dict(payload),
    )


def _evidence_text(payload: dict[str, Any]) -> str:
    parts: list[str] = []
    for field in TEXT_EVIDENCE_FIELDS:
        value = payload.get(field)
        if isinstance(value, str) and value.strip():
            parts.append(value.strip())
    frames = payload.get("frames")
    if isinstance(frames, list):
        frame_text = ";".join(frame for frame in frames if isinstance(frame, str))
        if frame_text:
            parts.append(frame_text)
    return "\n".join(parts)
