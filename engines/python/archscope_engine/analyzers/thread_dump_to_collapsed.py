"""Thread-dump → collapsed-format batch converter (T-216).

Inspired by aitop's `convertToCollapsedFormat` (Java jstack only). The
new pipeline reuses the multi-language parser registry from Phase 5, so
hundreds of dumps from any combination of Java/Go/Python/Node.js/.NET
can be folded into a single FlameGraph-compatible collapsed file.

For each :class:`ThreadSnapshot` we emit one collapsed line of the form
``frameN-1;frameN-2;...;frame0 1`` (root frame first, leaf last) with a
sample count of 1 per snapshot. Identical stacks across snapshots are
aggregated by the standard collapsed-format convention so the resulting
file feeds straight into ``profiler analyze-collapsed``.

Per-language enrichment (T-194 onward) already runs inside each parser
plugin, so AOP-proxy normalization and per-runtime state inference are
applied automatically before stacks are flattened — no JVM-specific
post-processing here.
"""
from __future__ import annotations

from collections import Counter
from pathlib import Path
from typing import Iterable

from archscope_engine.models.thread_snapshot import (
    StackFrame,
    ThreadDumpBundle,
    ThreadSnapshot,
)
from archscope_engine.parsers.thread_dump import DEFAULT_REGISTRY


def convert_thread_dumps_to_collapsed(
    inputs: Iterable[Path],
    *,
    format_override: str | None = None,
    include_thread_name: bool = True,
) -> Counter[str]:
    """Parse the given dump files and return aggregated collapsed stacks.

    The keys of the returned counter are ``"frame_root;...;frame_leaf"``
    strings ready to be written as ``f"{stack} {count}\n"`` lines.

    ``include_thread_name`` prepends the thread name as the deepest
    "synthetic" frame (mirroring aitop's behavior) so multiple threads
    that happen to share the same stack stay distinguishable in the
    flamegraph. Pass ``False`` to merge them aggressively.
    """
    bundles = DEFAULT_REGISTRY.parse_many(
        list(inputs), format_override=format_override
    )
    return _collapse_bundles(bundles, include_thread_name=include_thread_name)


def write_collapsed_file(
    inputs: Iterable[Path],
    output: Path,
    *,
    format_override: str | None = None,
    include_thread_name: bool = True,
) -> tuple[Path, int]:
    """Convert and write the result to ``output``. Returns (path, line_count)."""
    counter = convert_thread_dumps_to_collapsed(
        inputs,
        format_override=format_override,
        include_thread_name=include_thread_name,
    )
    output.parent.mkdir(parents=True, exist_ok=True)
    with output.open("w", encoding="utf-8") as handle:
        for stack, count in counter.most_common():
            handle.write(f"{stack} {count}\n")
    return output, len(counter)


def _collapse_bundles(
    bundles: list[ThreadDumpBundle],
    *,
    include_thread_name: bool,
) -> Counter[str]:
    counter: Counter[str] = Counter()
    for bundle in bundles:
        for snapshot in bundle.snapshots:
            stack = _collapse_snapshot(snapshot, include_thread_name=include_thread_name)
            if not stack:
                continue
            counter[stack] += 1
    return counter


def _collapse_snapshot(
    snapshot: ThreadSnapshot,
    *,
    include_thread_name: bool,
) -> str:
    frames = list(snapshot.stack_frames)
    if not frames:
        return ""
    # Collapsed convention: root at the left, leaf at the right.
    # Most runtime dumps print top-of-stack first, so reverse to get
    # caller-first ordering.
    rendered = [_render_frame(frame) for frame in reversed(frames)]
    if include_thread_name:
        rendered.insert(0, _sanitize(snapshot.thread_name))
    return ";".join(rendered)


def _render_frame(frame: StackFrame) -> str:
    if frame.module:
        text = f"{frame.module}.{frame.function}"
    else:
        text = frame.function
    return _sanitize(text)


def _sanitize(text: str) -> str:
    """Make sure no semicolons or newlines leak into the collapsed line."""
    return text.replace(";", "_").replace("\n", " ").replace("\r", " ").strip()
