from __future__ import annotations

from collections import Counter
from dataclasses import dataclass, field

from archscope_engine.models.flamegraph import FlameNode


@dataclass
class _MutableFlameNode:
    id: str
    parent_id: str | None
    name: str
    samples: int = 0
    category: str | None = None
    color: str | None = None
    path: list[str] = field(default_factory=list)
    children: dict[str, "_MutableFlameNode"] = field(default_factory=dict)


def build_flame_tree_from_collapsed(stacks: Counter[str]) -> FlameNode:
    root = _MutableFlameNode(
        id="root",
        parent_id=None,
        name="All",
        samples=sum(stacks.values()),
        path=[],
    )

    for stack, samples in stacks.items():
        if samples <= 0:
            continue
        current = root
        path: list[str] = []
        for frame in [part for part in stack.split(";") if part]:
            path.append(frame)
            child = current.children.get(frame)
            if child is None:
                child = _MutableFlameNode(
                    id=_node_id(path),
                    parent_id=current.id,
                    name=frame,
                    path=list(path),
                )
                current.children[frame] = child
            child.samples += samples
            current = child

    return _freeze_node(root, total_samples=max(root.samples, 1))


def build_flame_tree_from_paths(
    paths: list[tuple[list[str], int]],
    *,
    root_name: str = "All",
) -> FlameNode:
    stacks: Counter[str] = Counter()
    for path, samples in paths:
        if path and samples > 0:
            stacks[";".join(path)] += samples
    root = build_flame_tree_from_collapsed(stacks)
    root.name = root_name
    return root


def extract_leaf_paths(root: FlameNode) -> list[tuple[list[str], int]]:
    leaves: list[tuple[list[str], int]] = []

    def visit(node: FlameNode) -> None:
        if not node.children and node.path:
            leaves.append((node.path, node.samples))
            return
        for child in node.children:
            visit(child)

    visit(root)
    return leaves


def top_child_frames(root: FlameNode, limit: int = 10) -> list[dict[str, int | str | float]]:
    return [
        {
            "frame": child.name,
            "samples": child.samples,
            "ratio": child.ratio,
        }
        for child in sorted(root.children, key=lambda item: item.samples, reverse=True)[:limit]
    ]


def top_stacks_from_tree(root: FlameNode, limit: int = 20) -> list[dict[str, int | str | float]]:
    leaves = sorted(extract_leaf_paths(root), key=lambda item: item[1], reverse=True)[:limit]
    total = max(root.samples, 1)
    return [
        {
            "stack": ";".join(path),
            "samples": samples,
            "sample_ratio": round(samples / total * 100, 2),
        }
        for path, samples in leaves
    ]


def _freeze_node(node: _MutableFlameNode, total_samples: int) -> FlameNode:
    frozen = FlameNode(
        id=node.id,
        parent_id=node.parent_id,
        name=node.name,
        samples=node.samples,
        ratio=round(node.samples / total_samples * 100, 4) if total_samples else 0.0,
        category=node.category,
        color=node.color,
        path=node.path,
    )
    frozen.children = [
        _freeze_node(child, total_samples)
        for child in sorted(node.children.values(), key=lambda item: item.samples, reverse=True)
    ]
    return frozen


def _node_id(path: list[str]) -> str:
    return "frame:" + "/".join(_slug(part) for part in path)


def _slug(value: str) -> str:
    return (
        value.replace("\\", "_")
        .replace("/", "_")
        .replace(";", "_")
        .replace(" ", "_")
    )[:160]
