from __future__ import annotations

from dataclasses import dataclass
import re
from typing import Literal

from archscope_engine.analyzers.flamegraph_builder import (
    build_flame_tree_from_paths,
    iter_leaf_paths,
    top_child_frames,
    top_stacks_from_tree,
)
from archscope_engine.models.flamegraph import FlameNode

FilterType = Literal["include_text", "exclude_text", "regex_include", "regex_exclude"]
MatchMode = Literal["anywhere", "ordered", "subtree"]
ViewMode = Literal["preserve_full_path", "reroot_at_match"]
MAX_REGEX_PATTERN_LENGTH = 500
NESTED_QUANTIFIER_PATTERN = re.compile(
    r"\((?:[^()\\]|\\.)*[+*](?:[^()\\]|\\.)*\)\s*(?:[+*]|\{\d+(?:,\d*)?\})"
)


@dataclass(frozen=True)
class DrilldownFilter:
    pattern: str
    filter_type: FilterType = "include_text"
    match_mode: MatchMode = "anywhere"
    view_mode: ViewMode = "preserve_full_path"
    case_sensitive: bool = False
    label: str | None = None

    @property
    def display_label(self) -> str:
        return self.label or f"{self.filter_type}:{self.pattern}"


@dataclass(frozen=True)
class DrilldownStage:
    index: int
    label: str
    breadcrumb: list[str]
    flamegraph: FlameNode
    metrics: dict[str, float | int | str | None]
    top_stacks: list[dict[str, int | str | float]]
    top_child_frames: list[dict[str, int | str | float]]
    filter: DrilldownFilter | None = None
    diagnostics: dict[str, str] | None = None

    def to_dict(self) -> dict[str, object]:
        return {
            "index": self.index,
            "label": self.label,
            "breadcrumb": self.breadcrumb,
            "filter": self.filter.__dict__ if self.filter is not None else None,
            "metrics": self.metrics,
            "flamegraph": self.flamegraph.to_dict(),
            "top_stacks": self.top_stacks,
            "top_child_frames": self.top_child_frames,
            "diagnostics": self.diagnostics,
        }


def create_root_stage(
    root: FlameNode,
    *,
    interval_ms: float,
    elapsed_sec: float | None,
    top_n: int = 20,
) -> DrilldownStage:
    return _stage(
        index=0,
        label="All",
        breadcrumb=["All"],
        root=root,
        filter_spec=None,
        parent_samples=None,
        total_samples=root.samples,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
    )


def apply_drilldown_filter(
    parent: DrilldownStage,
    filter_spec: DrilldownFilter,
    *,
    interval_ms: float,
    elapsed_sec: float | None,
    top_n: int = 20,
) -> DrilldownStage:
    compiled, error = _compile_filter(filter_spec)
    if error is not None:
        next_root = build_flame_tree_from_paths(
            (),
            root_name=filter_spec.display_label,
        )
        return _stage(
            index=parent.index + 1,
            label=filter_spec.display_label,
            breadcrumb=[*parent.breadcrumb, filter_spec.display_label],
            root=next_root,
            filter_spec=filter_spec,
            parent_samples=parent.flamegraph.samples,
            total_samples=parent.metrics["total_samples"],
            interval_ms=interval_ms,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
            diagnostics=error,
        )

    next_root = build_flame_tree_from_paths(
        _iter_filtered_paths(parent.flamegraph, filter_spec, compiled),
        root_name=filter_spec.display_label,
    )
    return _stage(
        index=parent.index + 1,
        label=filter_spec.display_label,
        breadcrumb=[*parent.breadcrumb, filter_spec.display_label],
        root=next_root,
        filter_spec=filter_spec,
        parent_samples=parent.flamegraph.samples,
        total_samples=parent.metrics["total_samples"],
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        diagnostics=None,
    )


def build_drilldown_stages(
    root: FlameNode,
    filters: list[DrilldownFilter],
    *,
    interval_ms: float,
    elapsed_sec: float | None,
    top_n: int = 20,
) -> list[DrilldownStage]:
    stages = [
        create_root_stage(
            root,
            interval_ms=interval_ms,
            elapsed_sec=elapsed_sec,
            top_n=top_n,
        )
    ]
    for filter_spec in filters:
        stages.append(
            apply_drilldown_filter(
                stages[-1],
                filter_spec,
                interval_ms=interval_ms,
                elapsed_sec=elapsed_sec,
                top_n=top_n,
            )
        )
    return stages


def _stage(
    *,
    index: int,
    label: str,
    breadcrumb: list[str],
    root: FlameNode,
    filter_spec: DrilldownFilter | None,
    parent_samples: int | None,
    total_samples: int,
    interval_ms: float,
    elapsed_sec: float | None,
    top_n: int,
    diagnostics: dict[str, str] | None = None,
) -> DrilldownStage:
    interval_seconds = interval_ms / 1000
    estimated_seconds = root.samples * interval_seconds
    return DrilldownStage(
        index=index,
        label=label,
        breadcrumb=breadcrumb,
        filter=filter_spec,
        flamegraph=root,
        metrics={
            "total_samples": total_samples,
            "matched_samples": root.samples,
            "estimated_seconds": round(estimated_seconds, 3),
            "total_ratio": round(root.samples / total_samples * 100, 4)
            if total_samples
            else 0.0,
            "parent_stage_ratio": round(root.samples / parent_samples * 100, 4)
            if parent_samples
            else 100.0,
            "elapsed_ratio": round(estimated_seconds / elapsed_sec * 100, 4)
            if elapsed_sec and elapsed_sec > 0
            else None,
        },
        top_stacks=top_stacks_from_tree(root, top_n),
        top_child_frames=top_child_frames(root, top_n),
        diagnostics=diagnostics,
    )


def _iter_filtered_paths(
    root: FlameNode,
    filter_spec: DrilldownFilter,
    compiled: re.Pattern[str] | None,
):
    for path, samples in iter_leaf_paths(root):
        matched_index = _matched_index(path, filter_spec, compiled)
        include = matched_index is not None
        if filter_spec.filter_type in {"exclude_text", "regex_exclude"}:
            include = not include
        if not include:
            continue

        if (
            filter_spec.view_mode == "reroot_at_match"
            and matched_index is not None
            and filter_spec.filter_type in {"include_text", "regex_include"}
        ):
            next_path = path[matched_index:]
        else:
            next_path = path
        yield next_path, samples


def _matched_index(
    path: list[str],
    filter_spec: DrilldownFilter,
    compiled: re.Pattern[str] | None,
) -> int | None:
    if filter_spec.match_mode == "ordered":
        return _ordered_match_index(path, filter_spec, compiled)
    if filter_spec.match_mode == "subtree":
        return _first_matching_frame(path, filter_spec, compiled)
    return _first_matching_frame(path, filter_spec, compiled)


def _ordered_match_index(
    path: list[str],
    filter_spec: DrilldownFilter,
    compiled: re.Pattern[str] | None,
) -> int | None:
    terms = [term.strip() for term in re.split(r"[>;]", filter_spec.pattern) if term.strip()]
    if not terms:
        return None
    start_index: int | None = None
    term_index = 0
    for frame_index, frame in enumerate(path):
        if _frame_matches(frame, terms[term_index], filter_spec, compiled):
            if start_index is None:
                start_index = frame_index
            term_index += 1
            if term_index == len(terms):
                return start_index
    return None


def _first_matching_frame(
    path: list[str],
    filter_spec: DrilldownFilter,
    compiled: re.Pattern[str] | None,
) -> int | None:
    for index, frame in enumerate(path):
        if _frame_matches(frame, filter_spec.pattern, filter_spec, compiled):
            return index
    return None


def _frame_matches(
    frame: str,
    pattern: str,
    filter_spec: DrilldownFilter,
    compiled: re.Pattern[str] | None,
) -> bool:
    if filter_spec.filter_type in {"regex_include", "regex_exclude"}:
        pattern_to_use = compiled
        if filter_spec.match_mode == "ordered":
            pattern_to_use, error = _compile_regex(pattern, filter_spec)
            if error is not None:
                return False
        return pattern_to_use.search(frame) is not None if pattern_to_use is not None else False
    if filter_spec.case_sensitive:
        return pattern in frame
    return pattern.casefold() in frame.casefold()


def _compile_filter(
    filter_spec: DrilldownFilter,
) -> tuple[re.Pattern[str] | None, dict[str, str] | None]:
    if filter_spec.filter_type not in {"regex_include", "regex_exclude"}:
        return None, None
    if filter_spec.match_mode == "ordered":
        return None, None
    return _compile_regex(filter_spec.pattern, filter_spec)


def _compile_regex(
    pattern: str,
    filter_spec: DrilldownFilter,
) -> tuple[re.Pattern[str] | None, dict[str, str] | None]:
    if _is_unsafe_regex_pattern(pattern):
        return None, {
            "reason": "UNSAFE_REGEX",
            "message": (
                "Regex pattern is too long or contains a nested quantifier pattern "
                "that is disabled for parser safety."
            ),
        }
    flags = 0 if filter_spec.case_sensitive else re.IGNORECASE
    try:
        return re.compile(pattern, flags), None
    except re.error as exc:
        return None, {
            "reason": "INVALID_REGEX",
            "message": f"Invalid regex pattern: {exc}",
        }


def _is_unsafe_regex_pattern(pattern: str) -> bool:
    if len(pattern) > MAX_REGEX_PATTERN_LENGTH:
        return True
    return NESTED_QUANTIFIER_PATTERN.search(pattern) is not None
