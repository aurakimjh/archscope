from __future__ import annotations

from collections import Counter
from pathlib import Path
from typing import Any, cast

from archscope_engine.common.debug_log import DebugLogCollector
from archscope_engine.common.file_utils import detect_text_encoding
from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.flamegraph import FlameNode, flame_node_from_dict
from archscope_engine.models.profile_stack import ProfileStack
from archscope_engine.models.result_contracts import (
    ParserDiagnostics as ParserDiagnosticsContract,
    ProfilerCollapsedMetadata,
    ProfilerCollapsedSeries,
    ProfilerCollapsedSummary,
    ProfilerCollapsedTables,
)
from archscope_engine.analyzers.profile_classification import (
    DEFAULT_STACK_CLASSIFICATION_RULES,
    StackClassificationRule,
    classify_stack,
)
from archscope_engine.parsers.collapsed_parser import parse_collapsed_file_with_diagnostics
from archscope_engine.parsers.html_profiler_parser import parse_html_profiler
from archscope_engine.parsers.jennifer_csv_parser import parse_jennifer_flamegraph_csv
from archscope_engine.parsers.svg_flamegraph_parser import parse_svg_flamegraph
from archscope_engine.analyzers.flamegraph_builder import build_flame_tree_from_collapsed
from archscope_engine.analyzers.profiler_breakdown import build_execution_breakdown
from archscope_engine.analyzers.profiler_timeline import build_timeline_analysis
from archscope_engine.analyzers.profiler_drilldown import (
    DrilldownFilter,
    build_drilldown_stages,
    create_root_stage,
)


def analyze_collapsed_profile(
    path: Path,
    interval_ms: float,
    elapsed_sec: float | None = None,
    top_n: int = 20,
    profile_kind: str = "wall",
    classification_rules: tuple[
        StackClassificationRule, ...
    ] = DEFAULT_STACK_CLASSIFICATION_RULES,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    parse_result = parse_collapsed_file_with_diagnostics(path, debug_log=debug_log)
    return build_collapsed_result(
        stacks=parse_result.stacks,
        source_file=path,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        profile_kind=profile_kind,
        diagnostics=parse_result.diagnostics,
        classification_rules=classification_rules,
    )


def analyze_flamegraph_svg_profile(
    path: Path,
    interval_ms: float = 100,
    elapsed_sec: float | None = None,
    top_n: int = 20,
    profile_kind: str = "wall",
    classification_rules: tuple[
        StackClassificationRule, ...
    ] = DEFAULT_STACK_CLASSIFICATION_RULES,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    """Analyze a FlameGraph.pl/async-profiler-style SVG flamegraph file.

    The SVG is converted to collapsed-format stacks and then fed through
    the existing collapsed pipeline so drill-down/breakdown remain
    available without duplicating analyzer logic.
    """
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    parse_result = parse_svg_flamegraph(path)
    diagnostics = dict(parse_result.diagnostics)
    result = build_collapsed_result(
        stacks=parse_result.stacks,
        source_file=path,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        profile_kind=profile_kind,
        diagnostics=diagnostics,
        classification_rules=classification_rules,
    )
    result.metadata["parser"] = "flamegraph_svg"
    return result


def analyze_flamegraph_html_profile(
    path: Path,
    interval_ms: float = 100,
    elapsed_sec: float | None = None,
    top_n: int = 20,
    profile_kind: str = "wall",
    classification_rules: tuple[
        StackClassificationRule, ...
    ] = DEFAULT_STACK_CLASSIFICATION_RULES,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    """Analyze an HTML-wrapped flamegraph (inline SVG or async-profiler JS)."""
    if debug_log is not None:
        debug_log.encoding_detected = detect_text_encoding(path)
    parse_result = parse_html_profiler(path)
    diagnostics = dict(parse_result.diagnostics)
    diagnostics["detected_format"] = parse_result.detected_format
    result = build_collapsed_result(
        stacks=parse_result.stacks,
        source_file=path,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        profile_kind=profile_kind,
        diagnostics=diagnostics,
        classification_rules=classification_rules,
    )
    result.metadata["parser"] = "flamegraph_html"
    result.metadata["detected_html_format"] = parse_result.detected_format
    return result


def analyze_jennifer_csv_profile(
    path: Path,
    interval_ms: float = 100,
    elapsed_sec: float | None = None,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    if debug_log is not None:
        debug_log.encoding_detected = "utf-8-sig"
    parse_result = parse_jennifer_flamegraph_csv(path, debug_log=debug_log)
    root = parse_result.root
    return _build_flamegraph_result(
        root=root,
        source_file=path,
        parser="jennifer_flamegraph_csv",
        profile_kind="wall",
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        diagnostics=parse_result.diagnostics,
    )


def drilldown_collapsed_profile(
    path: Path,
    interval_ms: float,
    filters: list[DrilldownFilter],
    elapsed_sec: float | None = None,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    result = analyze_collapsed_profile(
        path=path,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        debug_log=debug_log,
    )
    return _drilldown_from_result(result, filters, interval_ms, elapsed_sec, top_n)


def drilldown_jennifer_csv_profile(
    path: Path,
    filters: list[DrilldownFilter],
    interval_ms: float = 100,
    elapsed_sec: float | None = None,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    result = analyze_jennifer_csv_profile(
        path=path,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        debug_log=debug_log,
    )
    return _drilldown_from_result(result, filters, interval_ms, elapsed_sec, top_n)


def breakdown_collapsed_profile(
    path: Path,
    interval_ms: float,
    filters: list[DrilldownFilter] | None = None,
    elapsed_sec: float | None = None,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    filters = filters or []
    return drilldown_collapsed_profile(
        path=path,
        interval_ms=interval_ms,
        filters=filters,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        debug_log=debug_log,
    )


def breakdown_jennifer_csv_profile(
    path: Path,
    filters: list[DrilldownFilter] | None = None,
    interval_ms: float = 100,
    elapsed_sec: float | None = None,
    top_n: int = 20,
    debug_log: DebugLogCollector | None = None,
) -> AnalysisResult:
    return drilldown_jennifer_csv_profile(
        path=path,
        filters=filters or [],
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        debug_log=debug_log,
    )


def build_collapsed_result(
    stacks: Counter[str],
    source_file: Path,
    interval_ms: float,
    elapsed_sec: float | None,
    top_n: int = 20,
    profile_kind: str = "wall",
    diagnostics: dict[str, Any] | None = None,
    classification_rules: tuple[
        StackClassificationRule, ...
    ] = DEFAULT_STACK_CLASSIFICATION_RULES,
) -> AnalysisResult:
    total_samples = sum(stacks.values())
    interval_seconds = interval_ms / 1000
    estimated_seconds = total_samples * interval_seconds
    stack_classification_cache: dict[str, str] = {}

    top_stacks = [
        _to_profile_stack(
            stack=stack,
            samples=samples,
            total_samples=total_samples,
            interval_seconds=interval_seconds,
            elapsed_sec=elapsed_sec,
            classification_rules=classification_rules,
            classification_cache=stack_classification_cache,
        )
        for stack, samples in stacks.most_common(top_n)
    ]

    summary: ProfilerCollapsedSummary = {
        "profile_kind": profile_kind,
        "total_samples": total_samples,
        "interval_ms": interval_ms,
        "estimated_seconds": round(estimated_seconds, 3),
        "elapsed_seconds": elapsed_sec,
    }
    series: ProfilerCollapsedSeries = {
        "top_stacks": [
            {
                "stack": item.stack,
                "samples": item.samples,
                "estimated_seconds": item.estimated_seconds,
                "sample_ratio": item.sample_ratio,
                "elapsed_ratio": item.elapsed_ratio,
            }
            for item in top_stacks
        ],
        "component_breakdown": _component_breakdown(
            stacks,
            classification_rules,
            stack_classification_cache,
        ),
    }
    tables: ProfilerCollapsedTables = {
        "top_stacks": [
            {
                "stack": item.stack,
                "samples": item.samples,
                "estimated_seconds": item.estimated_seconds,
                "sample_ratio": item.sample_ratio,
                "elapsed_ratio": item.elapsed_ratio,
                "frames": item.frames,
            }
            for item in top_stacks
        ]
    }
    flamegraph = build_flame_tree_from_collapsed(stacks)
    root_stage = create_root_stage(
        flamegraph,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
    )
    series["execution_breakdown"] = build_execution_breakdown(
        flamegraph,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
    )
    series["timeline_analysis"] = build_timeline_analysis(
        flamegraph,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
    )
    tables["top_child_frames"] = root_stage.top_child_frames
    tables["timeline_analysis"] = series["timeline_analysis"]
    charts = {
        "flamegraph": flamegraph.to_dict(),
        "drilldown_stages": [root_stage.to_dict()],
    }
    metadata: ProfilerCollapsedMetadata = {
        "parser": "async_profiler_collapsed",
        "schema_version": "0.1.0",
        "diagnostics": cast(
            ParserDiagnosticsContract,
            diagnostics if diagnostics is not None else _default_diagnostics(stacks),
        ),
    }

    return AnalysisResult(
        type="profiler_collapsed",
        source_files=[str(source_file)],
        summary=summary,
        series=series,
        tables=tables,
        charts=charts,
        metadata=metadata,
    )


def _build_flamegraph_result(
    *,
    root: FlameNode,
    source_file: Path,
    parser: str,
    profile_kind: str,
    interval_ms: float,
    elapsed_sec: float | None,
    top_n: int,
    diagnostics: dict[str, Any],
) -> AnalysisResult:
    total_samples = root.samples
    estimated_seconds = total_samples * (interval_ms / 1000)
    root_stage = create_root_stage(
        root,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
    )
    return AnalysisResult(
        type="profiler_collapsed",
        source_files=[str(source_file)],
        summary={
            "profile_kind": profile_kind,
            "total_samples": total_samples,
            "interval_ms": interval_ms,
            "estimated_seconds": round(estimated_seconds, 3),
            "elapsed_seconds": elapsed_sec,
        },
        series={
            "top_stacks": [
                {
                    "stack": item["stack"],
                    "samples": item["samples"],
                    "estimated_seconds": round(item["samples"] * (interval_ms / 1000), 3),
                    "sample_ratio": item["sample_ratio"],
                    "elapsed_ratio": (
                        round(item["samples"] * (interval_ms / 1000) / elapsed_sec * 100, 2)
                        if elapsed_sec and elapsed_sec > 0
                        else None
                    ),
                }
                for item in root_stage.top_stacks
            ],
            "component_breakdown": [],
            "execution_breakdown": build_execution_breakdown(
                root,
                interval_ms=interval_ms,
                elapsed_sec=elapsed_sec,
                top_n=top_n,
            ),
            "timeline_analysis": build_timeline_analysis(
                root,
                interval_ms=interval_ms,
                elapsed_sec=elapsed_sec,
                top_n=top_n,
            ),
        },
        tables={
            "top_stacks": [
                {
                    "stack": item["stack"],
                    "samples": item["samples"],
                    "estimated_seconds": round(item["samples"] * (interval_ms / 1000), 3),
                    "sample_ratio": item["sample_ratio"],
                    "elapsed_ratio": (
                        round(item["samples"] * (interval_ms / 1000) / elapsed_sec * 100, 2)
                        if elapsed_sec and elapsed_sec > 0
                        else None
                    ),
                    "frames": str(item["stack"]).split(";"),
                }
                for item in root_stage.top_stacks
            ],
            "top_child_frames": root_stage.top_child_frames,
            "timeline_analysis": build_timeline_analysis(
                root,
                interval_ms=interval_ms,
                elapsed_sec=elapsed_sec,
                top_n=top_n,
            ),
        },
        charts={
            "flamegraph": root.to_dict(),
            "drilldown_stages": [root_stage.to_dict()],
        },
        metadata={
            "parser": parser,
            "schema_version": "0.1.0",
            "diagnostics": diagnostics,
        },
    )


def _drilldown_from_result(
    result: AnalysisResult,
    filters: list[DrilldownFilter],
    interval_ms: float,
    elapsed_sec: float | None,
    top_n: int,
) -> AnalysisResult:
    root = flame_node_from_dict(result.charts["flamegraph"])
    stages = build_drilldown_stages(
        root,
        filters,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
    )
    current = stages[-1]
    charts = {
        **result.charts,
        "drilldown_stages": [stage.to_dict() for stage in stages],
        "flamegraph": current.flamegraph.to_dict(),
    }
    series = {
        **result.series,
        "execution_breakdown": build_execution_breakdown(
            current.flamegraph,
            interval_ms=interval_ms,
            elapsed_sec=elapsed_sec,
            total_samples=stages[0].flamegraph.samples,
            parent_samples=stages[-2].flamegraph.samples if len(stages) > 1 else None,
            top_n=top_n,
        ),
        "timeline_analysis": build_timeline_analysis(
            current.flamegraph,
            interval_ms=interval_ms,
            elapsed_sec=elapsed_sec,
            total_samples=stages[0].flamegraph.samples,
            top_n=top_n,
        ),
    }
    tables = {
        **result.tables,
        "top_stacks": [
            {
                "stack": item["stack"],
                "samples": item["samples"],
                "estimated_seconds": round(item["samples"] * (interval_ms / 1000), 3),
                "sample_ratio": item["sample_ratio"],
                "elapsed_ratio": (
                    round(item["samples"] * (interval_ms / 1000) / elapsed_sec * 100, 2)
                    if elapsed_sec and elapsed_sec > 0
                    else None
                ),
                "frames": str(item["stack"]).split(";"),
            }
            for item in current.top_stacks
        ],
        "top_child_frames": current.top_child_frames,
        "timeline_analysis": series["timeline_analysis"],
    }
    metadata = {
        **result.metadata,
        "drilldown_current_stage": current.to_dict(),
    }
    return AnalysisResult(
        type=result.type,
        source_files=list(result.source_files),
        summary=dict(result.summary),
        series=series,
        tables=tables,
        charts=charts,
        metadata=metadata,
        created_at=result.created_at,
    )


def _to_profile_stack(
    stack: str,
    samples: int,
    total_samples: int,
    interval_seconds: float,
    elapsed_sec: float | None,
    classification_rules: tuple[StackClassificationRule, ...],
    classification_cache: dict[str, str],
) -> ProfileStack:
    estimated_seconds = samples * interval_seconds
    sample_ratio = (samples / total_samples * 100) if total_samples else 0.0
    elapsed_ratio = (
        (estimated_seconds / elapsed_sec * 100)
        if elapsed_sec and elapsed_sec > 0
        else None
    )
    return ProfileStack(
        stack=stack,
        frames=stack.split(";"),
        samples=samples,
        estimated_seconds=round(estimated_seconds, 3),
        sample_ratio=round(sample_ratio, 2),
        elapsed_ratio=round(elapsed_ratio, 2) if elapsed_ratio is not None else None,
        category=_classify_stack_cached(
            stack,
            classification_rules,
            classification_cache,
        ),
    )


def _component_breakdown(
    stacks: Counter[str],
    classification_rules: tuple[StackClassificationRule, ...],
    classification_cache: dict[str, str] | None = None,
) -> list[dict[str, int | str]]:
    components: Counter[str] = Counter()
    cache = classification_cache if classification_cache is not None else {}
    for stack, samples in stacks.items():
        components[_classify_stack_cached(stack, classification_rules, cache)] += samples
    return [
        {"component": component, "samples": samples}
        for component, samples in components.most_common()
    ]


def _classify_stack_cached(
    stack: str,
    classification_rules: tuple[StackClassificationRule, ...],
    classification_cache: dict[str, str],
) -> str:
    cached = classification_cache.get(stack)
    if cached is not None:
        return cached
    classified = classify_stack(stack, classification_rules)
    classification_cache[stack] = classified
    return classified


def _default_diagnostics(stacks: Counter[str]) -> dict[str, Any]:
    parsed_records = len(stacks)
    return {
        "source_file": None,
        "format": "async_profiler_collapsed",
        "total_lines": parsed_records,
        "parsed_records": parsed_records,
        "skipped_lines": 0,
        "skipped_by_reason": {},
        "samples": [],
        "warning_count": 0,
        "error_count": 0,
        "warnings": [],
        "errors": [],
    }
