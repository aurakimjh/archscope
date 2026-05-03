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
        return {
            "id": self.id,
            "parentId": self.parent_id,
            "name": self.name,
            "samples": self.samples,
            "ratio": self.ratio,
            "category": self.category,
            "color": self.color,
            "children": [child.to_dict() for child in self.children],
            "path": self.path,
        }


def flame_node_from_dict(value: dict[str, Any]) -> FlameNode:
    return FlameNode(
        id=str(value["id"]),
        parent_id=value.get("parentId"),
        name=str(value["name"]),
        samples=int(value["samples"]),
        ratio=float(value.get("ratio", 0.0)),
        category=value.get("category"),
        color=value.get("color"),
        path=[str(part) for part in value.get("path", [])],
        children=[
            flame_node_from_dict(child)
            for child in value.get("children", [])
            if isinstance(child, dict)
        ],
    )
