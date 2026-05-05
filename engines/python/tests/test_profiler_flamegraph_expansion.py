from collections import Counter
from pathlib import Path

from archscope_engine.analyzers import profiler_breakdown
from archscope_engine.analyzers.flamegraph_builder import build_flame_tree_from_collapsed
from archscope_engine.analyzers.profiler_breakdown import (
    build_execution_breakdown,
    classify_execution_stack,
)
from archscope_engine.analyzers.profiler_drilldown import (
    DrilldownFilter,
    apply_drilldown_filter,
    create_root_stage,
)
from archscope_engine.parsers.jennifer_csv_parser import parse_jennifer_flamegraph_csv


def test_jennifer_csv_parser_reconstructs_tree_with_virtual_root() -> None:
    sample = Path(__file__).parents[3] / "examples/profiler/sample-jennifer-flame.csv"

    result = parse_jennifer_flamegraph_csv(sample)

    assert result.root.name == "All"
    assert result.root.samples == 115
    assert [child.name for child in result.root.children] == [
        "com.example.checkout.CheckoutController.checkout",
        "java.lang.Thread.sleep",
    ]
    checkout = result.root.children[0]
    assert checkout.children[0].name == "com.example.checkout.CheckoutService.placeOrder"
    assert result.diagnostics["parsed_records"] == 7


def test_jennifer_csv_parser_reports_duplicate_key(tmp_path) -> None:
    path = tmp_path / "jennifer.csv"
    path.write_text(
        "key,parent_key,method_name,ratio,sample_count\n"
        "root,,Root,100,10\n"
        "root,,Duplicate,100,5\n",
        encoding="utf-8",
    )

    result = parse_jennifer_flamegraph_csv(path)

    assert result.root.samples == 10
    assert result.diagnostics["skipped_by_reason"] == {"DUPLICATE_KEY": 1}
    assert result.diagnostics["error_count"] == 1


def test_jennifer_csv_parser_reports_missing_parent_as_warning(tmp_path) -> None:
    path = tmp_path / "jennifer.csv"
    path.write_text(
        "key,parent_key,method_name,ratio,sample_count\n"
        "child,missing,Child,100,5\n",
        encoding="utf-8",
    )

    result = parse_jennifer_flamegraph_csv(path)

    assert result.root.name == "Child"
    assert result.diagnostics["warning_count"] == 1
    assert result.diagnostics["warnings"][0]["reason"] == "MISSING_PARENT"


def test_jennifer_csv_parser_breaks_parent_cycles(tmp_path) -> None:
    path = tmp_path / "jennifer.csv"
    path.write_text(
        "key,parent_key,method_name,ratio,sample_count\n"
        "a,b,A,50,5\n"
        "b,a,B,50,5\n",
        encoding="utf-8",
    )

    result = parse_jennifer_flamegraph_csv(path)

    assert result.root.name == "All"
    assert {child.name for child in result.root.children} == {"A", "B"}
    assert result.diagnostics["errors"][0]["reason"] == "PARENT_CYCLE"


def test_jennifer_csv_parser_missing_required_columns(tmp_path) -> None:
    path = tmp_path / "jennifer.csv"
    path.write_text("key,parent_key,method_name\nroot,,Root\n", encoding="utf-8")

    result = parse_jennifer_flamegraph_csv(path)

    assert result.root.samples == 0
    assert result.diagnostics["errors"][0]["reason"] == "MISSING_REQUIRED_COLUMNS"


def test_jennifer_csv_parser_cp949_fallback(tmp_path) -> None:
    path = tmp_path / "jennifer.csv"
    path.write_bytes(
        "key,parent_key,method_name,ratio,sample_count\n"
        "root,,서비스,100,7\n".encode("cp949")
    )

    result = parse_jennifer_flamegraph_csv(path)

    assert result.root.name == "서비스"
    assert result.root.samples == 7
    assert result.diagnostics["warning_count"] >= 1


def test_collapsed_stack_to_flame_tree_conversion() -> None:
    tree = build_flame_tree_from_collapsed(
        Counter(
            {
                "root;service;dao": 5,
                "root;service;http": 3,
            }
        )
    )

    assert tree.samples == 8
    assert tree.children[0].name == "root"
    assert tree.children[0].samples == 8
    assert tree.children[0].children[0].name == "service"
    assert tree.children[0].children[0].samples == 8


def test_multi_stage_drilldown_include_then_exclude() -> None:
    tree = build_flame_tree_from_collapsed(
        Counter(
            {
                "root;service;oracle.jdbc.executeQuery": 5,
                "root;service;RestTemplate.exchange": 3,
                "root;background;Thread.sleep": 2,
            }
        )
    )
    stage0 = create_root_stage(tree, interval_ms=100, elapsed_sec=10)
    stage1 = apply_drilldown_filter(
        stage0,
        DrilldownFilter(pattern="service"),
        interval_ms=100,
        elapsed_sec=10,
    )
    stage2 = apply_drilldown_filter(
        stage1,
        DrilldownFilter(pattern="RestTemplate", filter_type="exclude_text"),
        interval_ms=100,
        elapsed_sec=10,
    )

    assert stage0.metrics["matched_samples"] == 10
    assert stage1.metrics["matched_samples"] == 8
    assert stage1.breadcrumb == ["All", "include_text:service"]
    assert stage2.metrics["matched_samples"] == 5
    assert stage2.metrics["parent_stage_ratio"] == 62.5


def test_drilldown_regex_filter_rejects_nested_quantifier_pattern() -> None:
    tree = build_flame_tree_from_collapsed(
        Counter(
            {
                "root;service;aaaaaaaaaaaaaaaaaaaa": 5,
                "root;service;bbbbbbbbbbbbbbbbbbbb": 3,
            }
        )
    )
    stage0 = create_root_stage(tree, interval_ms=100, elapsed_sec=None)

    stage1 = apply_drilldown_filter(
        stage0,
        DrilldownFilter(pattern="(a+)+b", filter_type="regex_include"),
        interval_ms=100,
        elapsed_sec=None,
    )

    assert stage1.metrics["matched_samples"] == 0
    assert stage1.diagnostics == {
        "reason": "UNSAFE_REGEX",
        "message": (
            "Regex pattern is too long or contains a nested quantifier pattern "
            "that is disabled for parser safety."
        ),
    }


def test_drilldown_regex_filter_reports_invalid_regex() -> None:
    tree = build_flame_tree_from_collapsed(Counter({"root;service;dao": 5}))
    stage0 = create_root_stage(tree, interval_ms=100, elapsed_sec=None)

    stage1 = apply_drilldown_filter(
        stage0,
        DrilldownFilter(pattern="[", filter_type="regex_include"),
        interval_ms=100,
        elapsed_sec=None,
    )

    assert stage1.metrics["matched_samples"] == 0
    assert stage1.diagnostics is not None
    assert stage1.diagnostics["reason"] == "INVALID_REGEX"


def test_drilldown_text_filter_supports_case_sensitive_mode() -> None:
    tree = build_flame_tree_from_collapsed(Counter({"Root;Service;Dao": 5}))
    stage0 = create_root_stage(tree, interval_ms=100, elapsed_sec=None)

    insensitive = apply_drilldown_filter(
        stage0,
        DrilldownFilter(pattern="service"),
        interval_ms=100,
        elapsed_sec=None,
    )
    sensitive = apply_drilldown_filter(
        stage0,
        DrilldownFilter(pattern="service", case_sensitive=True),
        interval_ms=100,
        elapsed_sec=None,
    )

    assert insensitive.metrics["matched_samples"] == 5
    assert sensitive.metrics["matched_samples"] == 0


def test_ordered_match_and_reroot_mode() -> None:
    tree = build_flame_tree_from_collapsed(
        Counter(
            {
                "root;controller;service;dao": 5,
                "root;controller;adapter;dao": 3,
            }
        )
    )
    stage0 = create_root_stage(tree, interval_ms=100, elapsed_sec=None)

    stage1 = apply_drilldown_filter(
        stage0,
        DrilldownFilter(
            pattern="controller>service>dao",
            match_mode="ordered",
            view_mode="reroot_at_match",
        ),
        interval_ms=100,
        elapsed_sec=None,
    )

    assert stage1.metrics["matched_samples"] == 5
    assert stage1.flamegraph.children[0].name == "controller"
    assert stage1.flamegraph.children[0].children[0].name == "service"


def test_reroot_mode_merges_duplicate_paths_after_filter() -> None:
    tree = build_flame_tree_from_collapsed(
        Counter(
            {
                "root;a;service;dao": 5,
                "root;b;service;dao": 7,
            }
        )
    )
    stage0 = create_root_stage(tree, interval_ms=100, elapsed_sec=None)

    stage1 = apply_drilldown_filter(
        stage0,
        DrilldownFilter(pattern="service", view_mode="reroot_at_match"),
        interval_ms=100,
        elapsed_sec=None,
    )

    assert stage1.metrics["matched_samples"] == 12
    assert stage1.flamegraph.children[0].name == "service"
    assert stage1.flamegraph.children[0].samples == 12


def test_execution_breakdown_classifies_required_categories() -> None:
    cases = {
        "SQL_DATABASE": ["com.example.Service", "oracle.jdbc.OracleStatement.executeQuery"],
        "EXTERNAL_API_HTTP": ["com.example.Service", "RestTemplate.exchange"],
        "NETWORK_IO_WAIT": ["java.net.SocketInputStream.socketRead"],
        "CONNECTION_POOL_WAIT": ["com.zaxxer.hikari.pool.HikariPool.getConnection"],
        "UNKNOWN": ["native_unknown_frame"],
    }

    for expected, frames in cases.items():
        assert classify_execution_stack(frames).primary_category == expected


def test_execution_breakdown_tracks_wait_reason_for_external_network_wait() -> None:
    tree = build_flame_tree_from_collapsed(
        Counter(
            {
                "com.example.Service;RestTemplate.exchange;SocketInputStream.socketRead": 7,
            }
        )
    )

    rows = build_execution_breakdown(tree, interval_ms=100, elapsed_sec=10)

    assert rows[0]["category"] == "EXTERNAL_API_HTTP"
    assert rows[0]["wait_reason"] == "NETWORK_IO_WAIT"
    assert rows[0]["samples"] == 7


def test_execution_breakdown_classifies_pool_wait_with_locksupport() -> None:
    classification = classify_execution_stack(
        [
            "com.zaxxer.hikari.pool.HikariPool.getConnection",
            "java.util.concurrent.locks.LockSupport.park",
        ]
    )

    assert classification.primary_category == "CONNECTION_POOL_WAIT"
    assert "LOCK_SYNCHRONIZATION_WAIT" in classification.matched_categories


def test_execution_breakdown_tracks_sql_network_wait() -> None:
    classification = classify_execution_stack(
        [
            "oracle.jdbc.driver.T4CPreparedStatement.executeQuery",
            "oracle.jdbc.driver.T4CMAREngine.unmarshalUB1",
            "java.net.SocketInputStream.socketRead",
        ]
    )

    assert classification.primary_category == "SQL_DATABASE"
    assert classification.wait_reason == "NETWORK_IO_WAIT"


def test_flamegraph_builder_handles_deep_stack_without_recursion_error() -> None:
    stack = ";".join(f"frame{i}" for i in range(1500))
    tree = build_flame_tree_from_collapsed(Counter({stack: 1}))

    payload = tree.to_dict()

    assert payload["samples"] == 1
    assert payload["children"][0]["name"] == "frame0"


def test_execution_stack_classification_reuses_cache(monkeypatch) -> None:
    profiler_breakdown.clear_execution_stack_classification_cache()
    calls = 0
    original = profiler_breakdown._classify_execution_stack_uncached

    def counted(frames: tuple[str, ...]):
        nonlocal calls
        calls += 1
        return original(frames)

    monkeypatch.setattr(
        profiler_breakdown,
        "_classify_execution_stack_uncached",
        counted,
    )

    frames = ("com.example.Service", "RestTemplate.exchange")

    assert profiler_breakdown.classify_execution_stack(frames).primary_category == (
        "EXTERNAL_API_HTTP"
    )
    assert profiler_breakdown.classify_execution_stack(frames).primary_category == (
        "EXTERNAL_API_HTTP"
    )
    assert calls == 1
    profiler_breakdown.clear_execution_stack_classification_cache()
