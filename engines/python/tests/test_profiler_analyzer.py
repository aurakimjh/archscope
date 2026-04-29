from pathlib import Path

from archscope_engine.analyzers.profiler_analyzer import analyze_collapsed_profile
from archscope_engine.analyzers.profile_classification import (
    StackClassificationRule,
    classify_stack,
)


def test_analyze_collapsed_merges_duplicate_stacks() -> None:
    sample = Path(__file__).parents[3] / "examples/profiler/sample-wall.collapsed"

    result = analyze_collapsed_profile(sample, interval_ms=100, elapsed_sec=1336.559)

    assert result.type == "profiler_collapsed"
    assert result.summary["total_samples"] == 32629
    assert result.summary["estimated_seconds"] == 3262.9


def test_analyze_collapsed_profile_includes_diagnostics_metadata(tmp_path) -> None:
    path = tmp_path / "profile.collapsed"
    path.write_text("frame1;frame2 10\nbad-line\n", encoding="utf-8")

    result = analyze_collapsed_profile(path, interval_ms=100)

    assert result.summary["total_samples"] == 10
    assert result.metadata["diagnostics"]["total_lines"] == 2
    assert result.metadata["diagnostics"]["parsed_records"] == 1
    assert result.metadata["diagnostics"]["skipped_lines"] == 1
    assert result.metadata["diagnostics"]["skipped_by_reason"] == {
        "MISSING_SAMPLE_COUNT": 1
    }


def test_profile_classification_supports_common_runtime_families() -> None:
    assert classify_stack("java.lang.Thread;com.example.Service") == "JVM"
    assert classify_stack("node:internal;node_modules/app/index.js") == "Node.js"
    assert classify_stack("python;site-packages/django/core") == "Python"
    assert classify_stack("runtime.goexit;main.handle /app/main.go:42") == "Go"
    assert classify_stack("System.Web;Microsoft.Data.SqlClient") == "ASP.NET / .NET"


def test_analyze_collapsed_profile_accepts_classification_rules(tmp_path) -> None:
    profile = tmp_path / "wall.collapsed"
    profile.write_text("vendor.special.Handler;com.example.Work 3\n", encoding="utf-8")

    result = analyze_collapsed_profile(
        profile,
        interval_ms=100,
        classification_rules=(
            StackClassificationRule("Vendor Runtime", ("vendor.special",)),
        ),
    )

    assert result.series["component_breakdown"] == [
        {"component": "Vendor Runtime", "samples": 3}
    ]
