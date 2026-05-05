from collections import Counter

from archscope_engine.analyzers.flamegraph_builder import build_flame_tree_from_collapsed
from archscope_engine.analyzers.profiler_analyzer import analyze_collapsed_profile
from archscope_engine.analyzers.profiler_timeline import build_timeline_analysis


def test_timeline_analysis_splits_sql_and_db_network_wait() -> None:
    tree = build_flame_tree_from_collapsed(
        Counter(
            {
                "com.example.Job;oracle.jdbc.Statement.executeQuery": 5,
                "com.example.Job;oracle.jdbc.T4CMAREngine;SocketInputStream.socketRead": 7,
            }
        )
    )

    rows = build_timeline_analysis(tree, interval_ms=100, elapsed_sec=10)

    by_segment = {row["segment"]: row for row in rows}
    assert by_segment["SQL_EXECUTION"]["samples"] == 5
    assert by_segment["DB_NETWORK_WAIT"]["samples"] == 7
    assert by_segment["DB_NETWORK_WAIT"]["estimated_seconds"] == 0.7


def test_timeline_analysis_splits_external_call_and_network_wait() -> None:
    tree = build_flame_tree_from_collapsed(
        Counter(
            {
                "com.example.Job;RestTemplate.exchange": 3,
                "com.example.Job;RestTemplate.exchange;SocketInputStream.socketRead": 4,
            }
        )
    )

    rows = build_timeline_analysis(tree, interval_ms=50, elapsed_sec=None)

    by_segment = {row["segment"]: row for row in rows}
    assert by_segment["EXTERNAL_CALL"]["samples"] == 3
    assert by_segment["EXTERNAL_NETWORK_WAIT"]["samples"] == 4
    assert "RestTemplate.exchange" in by_segment["EXTERNAL_NETWORK_WAIT"]["method_chains"][0]["chain"]


def test_timeline_analysis_result_is_emitted_from_profiler(tmp_path) -> None:
    path = tmp_path / "wall.collapsed"
    path.write_text(
        "com.example.BatchApplication.main;com.example.Job.execute 2\n"
        "com.example.Job.execute;com.example.Service.process 6\n"
        "com.example.Job.execute;oracle.jdbc.Statement.executeQuery 4\n",
        encoding="utf-8",
    )

    result = analyze_collapsed_profile(path, interval_ms=100, elapsed_sec=2, top_n=5)

    rows = result.series["timeline_analysis"]
    assert [row["segment"] for row in rows] == [
        "STARTUP_FRAMEWORK",
        "INTERNAL_METHOD",
        "SQL_EXECUTION",
    ]
    assert result.tables["timeline_analysis"] == rows
