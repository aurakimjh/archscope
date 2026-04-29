from __future__ import annotations

from collections import Counter
from pathlib import Path
from typing import Any

from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.profile_stack import ProfileStack
from archscope_engine.parsers.collapsed_parser import parse_collapsed_file_with_diagnostics


def analyze_collapsed_profile(
    path: Path,
    interval_ms: float,
    elapsed_sec: float | None = None,
    top_n: int = 20,
    profile_kind: str = "wall",
) -> AnalysisResult:
    parse_result = parse_collapsed_file_with_diagnostics(path)
    return build_collapsed_result(
        stacks=parse_result.stacks,
        source_file=path,
        interval_ms=interval_ms,
        elapsed_sec=elapsed_sec,
        top_n=top_n,
        profile_kind=profile_kind,
        diagnostics=parse_result.diagnostics,
    )


def build_collapsed_result(
    stacks: Counter[str],
    source_file: Path,
    interval_ms: float,
    elapsed_sec: float | None,
    top_n: int = 20,
    profile_kind: str = "wall",
    diagnostics: dict[str, Any] | None = None,
) -> AnalysisResult:
    total_samples = sum(stacks.values())
    interval_seconds = interval_ms / 1000
    estimated_seconds = total_samples * interval_seconds

    top_stacks = [
        _to_profile_stack(
            stack=stack,
            samples=samples,
            total_samples=total_samples,
            interval_seconds=interval_seconds,
            elapsed_sec=elapsed_sec,
        )
        for stack, samples in stacks.most_common(top_n)
    ]

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
                    "stack": item.stack,
                    "samples": item.samples,
                    "estimated_seconds": item.estimated_seconds,
                    "sample_ratio": item.sample_ratio,
                    "elapsed_ratio": item.elapsed_ratio,
                }
                for item in top_stacks
            ],
            "component_breakdown": _component_breakdown(stacks),
        },
        tables={
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
        },
        metadata={
            "parser": "async_profiler_collapsed",
            "schema_version": "0.1.0",
            **({"diagnostics": diagnostics} if diagnostics is not None else {}),
        },
    )


def _to_profile_stack(
    stack: str,
    samples: int,
    total_samples: int,
    interval_seconds: float,
    elapsed_sec: float | None,
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
        category=_classify_stack(stack),
    )


def _component_breakdown(stacks: Counter[str]) -> list[dict[str, int | str]]:
    components: Counter[str] = Counter()
    for stack, samples in stacks.items():
        components[_classify_stack(stack)] += samples
    return [
        {"component": component, "samples": samples}
        for component, samples in components.most_common()
    ]


def _classify_stack(stack: str) -> str:
    lowered = stack.lower()
    if "oracle.jdbc" in lowered:
        return "Oracle JDBC"
    if "socket" in lowered or "http" in lowered:
        return "HTTP / Network"
    if "springframework.batch" in lowered:
        return "Spring Batch"
    if "springframework" in lowered:
        return "Spring Framework"
    return "Application"
