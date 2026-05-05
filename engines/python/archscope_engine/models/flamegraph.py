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
    return {
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


def _node_from_dict_shallow(value: dict[str, Any]) -> FlameNode:
    return FlameNode(
        id=str(value["id"]),
        parent_id=value.get("parentId"),
        name=str(value["name"]),
        samples=int(value["samples"]),
        ratio=float(value.get("ratio", 0.0)),
        category=value.get("category"),
        color=value.get("color"),
        path=[str(part) for part in value.get("path", [])],
    )
