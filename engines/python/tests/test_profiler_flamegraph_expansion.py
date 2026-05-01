from collections import Counter
from pathlib import Path

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
