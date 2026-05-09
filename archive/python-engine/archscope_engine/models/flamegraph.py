"""Flame graph tree node."""
# [한글] flamegraph.FlameNode — flame graph 트리 노드 (가변).
# 필드: id (frame:slug 경로), parent_id, name (frame 이름),
# samples (inclusive 카운트), ratio (전체 대비 %), category, color,
# children (자식 노드 리스트), path (root → 자기까지 frame 이름들),
# metadata (분석기별 자유 페이로드 — diff 분석기가 a/b/delta 저장).
# JSON 직렬화시 키 이름은 camelCase (parentId 등) 로 frontend 호환.
# parity: Go internal/models 의 FlameNode 와 JSON 키 byte 일치.
from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class FlameNode:
    id: str
    parent_id: str | None
    name: str
    samples: int
    ratio: float
    category: str | None = None
    color: str | None = None
    children: list["FlameNode"] = field(default_factory=list)
    path: list[str] = field(default_factory=list)
    # Free-form per-node payload — populated by specialized analyzers
    # (e.g. profiler_diff stores ``{"a": int, "b": int, "delta": int,
    # "delta_ratio": float}``). Default trees leave it None to avoid
    # bloating the JSON wire format.
    metadata: dict[str, Any] | None = None

    def to_dict(self) -> dict[str, Any]:
        root = _node_to_dict_shallow(self)
        stack: list[tuple[FlameNode, dict[str, Any]]] = [(self, root)]
        while stack:
            node, payload = stack.pop()
            children_payload: list[dict[str, Any]] = []
            payload["children"] = children_payload
            for child in node.children:
                child_payload = _node_to_dict_shallow(child)
                children_payload.append(child_payload)
                stack.append((child, child_payload))
        return root


def flame_node_from_dict(value: dict[str, Any]) -> FlameNode:
    root = _node_from_dict_shallow(value)
    stack: list[tuple[FlameNode, dict[str, Any]]] = [(root, value)]
    while stack:
        node, payload = stack.pop()
        for child_value in payload.get("children", []):
            if not isinstance(child_value, dict):
                continue
            child = _node_from_dict_shallow(child_value)
            node.children.append(child)
            stack.append((child, child_value))
    return root


def _node_to_dict_shallow(node: FlameNode) -> dict[str, Any]:
    payload: dict[str, Any] = {
        "id": node.id,
        "parentId": node.parent_id,
        "name": node.name,
        "samples": node.samples,
        "ratio": node.ratio,
        "category": node.category,
        "color": node.color,
        "children": [],
        "path": node.path,
    }
    if node.metadata is not None:
        payload["metadata"] = node.metadata
    return payload


def _node_from_dict_shallow(value: dict[str, Any]) -> FlameNode:
    metadata = value.get("metadata")
    return FlameNode(
        id=str(value["id"]),
        parent_id=value.get("parentId"),
        name=str(value["name"]),
        samples=int(value["samples"]),
        ratio=float(value.get("ratio", 0.0)),
        category=value.get("category"),
        color=value.get("color"),
        path=[str(part) for part in value.get("path", [])],
        metadata=metadata if isinstance(metadata, dict) else None,
    )
