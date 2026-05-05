from __future__ import annotations

from collections import Counter
from dataclasses import dataclass, field
from heapq import nlargest
from typing import Iterable, Iterator

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
    paths: Iterable[tuple[list[str], int]],
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
    return list(iter_leaf_paths(root))


def iter_leaf_paths(root: FlameNode) -> Iterator[tuple[list[str], int]]:
    """Yield non-overlapping stack paths.

    Collapsed stacks have zero exclusive samples on intermediate nodes, so this
    behaves like a leaf iterator. Jennifer CSV rows are inclusive; yielding
    positive exclusive samples for internal nodes avoids double-counting while
    preserving self-time attributed to an intermediate frame.
    """
    stack = [root]
    while stack:
        node = stack.pop()
        child_total = sum(child.samples for child in node.children)
        exclusive_samples = node.samples - child_total
        if node.path and exclusive_samples > 0:
            yield node.path, exclusive_samples
        if not node.children and node.path and node.samples > 0:
            if exclusive_samples <= 0:
                yield node.path, node.samples
        for child in reversed(node.children):
            stack.append(child)


def top_child_frames(root: FlameNode, limit: int = 10) -> list[dict[str, int | str | float]]:
    return [
        {
            "frame": child.name,
            "samples": child.samples,
            "ratio": child.ratio,
        }
        for child in nlargest(limit, root.children, key=lambda item: item.samples)
    ]


def top_stacks_from_tree(root: FlameNode, limit: int = 20) -> list[dict[str, int | str | float]]:
    leaves = nlargest(limit, iter_leaf_paths(root), key=lambda item: item[1])
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
    frozen_by_node_id: dict[int, FlameNode] = {}
    stack: list[tuple[_MutableFlameNode, bool]] = [(node, False)]
    while stack:
        current, visited = stack.pop()
        if not visited:
            stack.append((current, True))
            for child in current.children.values():
                stack.append((child, False))
            continue

        frozen = FlameNode(
            id=current.id,
            parent_id=current.parent_id,
            name=current.name,
            samples=current.samples,
            ratio=round(current.samples / total_samples * 100, 4)
            if total_samples
            else 0.0,
            category=current.category,
            color=current.color,
            path=current.path,
        )
        frozen.children = [
            frozen_by_node_id[id(child)]
            for child in sorted(
                current.children.values(),
                key=lambda item: item.samples,
                reverse=True,
            )
        ]
        frozen_by_node_id[id(current)] = frozen
    return frozen_by_node_id[id(node)]


def _node_id(path: list[str]) -> str:
    return "frame:" + "/".join(_slug(part) for part in path)


def _slug(value: str) -> str:
    return (
        value.replace("\\", "_")
        .replace("/", "_")
        .replace(";", "_")
        .replace(" ", "_")
    )[:160]
