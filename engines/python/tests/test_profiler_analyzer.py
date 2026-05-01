from pathlib import Path

from archscope_engine.analyzers.profiler_analyzer import (
    _drilldown_from_result,
    analyze_collapsed_profile,
)
from archscope_engine.analyzers.profiler_drilldown import DrilldownFilter
from archscope_engine.analyzers.profile_classification import (
    StackClassificationRule,
    classify_stack,
    load_packaged_stack_classification_rules,
    load_stack_classification_rules,
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


def test_profile_classification_uses_application_fallback() -> None:
    assert classify_stack("com.mycompany.internal.Worker") == "Application"
    assert classify_stack("com.mycompany.HttpClientFacade") == "Application"


def test_profile_classification_rule_order_prefers_specific_rules() -> None:
    stack = "org.springframework.batch.core.Job;org.springframework.Service"

    assert classify_stack(stack) == "Spring Batch"


def test_profile_classification_is_case_insensitive() -> None:
    assert classify_stack("JAVA.LANG.THREAD;COM.EXAMPLE.SERVICE") == "JVM"


def test_profile_classification_loads_packaged_rules() -> None:
    rules = load_packaged_stack_classification_rules()

    assert classify_stack("oracle.jdbc.driver.OracleDriver", rules) == "Oracle JDBC"


def test_profile_classification_loads_external_config(tmp_path) -> None:
    config = tmp_path / "runtime_rules.json"
    config.write_text(
        '[{"label": "Vendor Runtime", "contains": ["vendor.special"]}]',
        encoding="utf-8",
    )

    rules = load_stack_classification_rules(config)

    assert classify_stack("vendor.special.Handler", rules) == "Vendor Runtime"


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


def test_drilldown_from_result_does_not_mutate_source_result(tmp_path) -> None:
    profile = tmp_path / "wall.collapsed"
    profile.write_text(
        "com.example.Controller;com.example.Service;com.example.Dao 10\n"
        "com.example.Controller;com.example.Cache 5\n",
        encoding="utf-8",
    )
    result = analyze_collapsed_profile(profile, interval_ms=100, top_n=10)
    original = result.to_dict()

    drilled = _drilldown_from_result(
        result,
        [DrilldownFilter(pattern="Dao")],
        interval_ms=100,
        elapsed_sec=None,
        top_n=10,
    )

    assert result.to_dict() == original
    assert drilled is not result
    assert len(result.charts["drilldown_stages"]) == 1
    assert len(drilled.charts["drilldown_stages"]) == 2
    assert drilled.metadata["drilldown_current_stage"]["label"] == "include_text:Dao"
