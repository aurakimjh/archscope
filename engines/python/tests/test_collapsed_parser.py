from pathlib import Path

from archscope_engine.analyzers.profiler_analyzer import analyze_collapsed_profile
from archscope_engine.parsers.collapsed_parser import parse_collapsed_line


def test_parse_collapsed_line() -> None:
    stack, samples = parse_collapsed_line("frame1;frame2;frame3 123")

    assert stack == "frame1;frame2;frame3"
    assert samples == 123


def test_analyze_collapsed_merges_duplicate_stacks() -> None:
    sample = Path(__file__).parents[3] / "examples/profiler/sample-wall.collapsed"

    result = analyze_collapsed_profile(sample, interval_ms=100, elapsed_sec=1336.559)

    assert result.type == "profiler_collapsed"
    assert result.summary["total_samples"] == 32629
    assert result.summary["estimated_seconds"] == 3262.9
