from collections import Counter

from archscope_engine.analyzers.flamegraph_builder import build_flame_tree_from_collapsed
from archscope_engine.analyzers.profiler_analyzer import analyze_collapsed_profile
from archscope_engine.analyzers.profiler_timeline import (
    build_timeline_analysis,
    build_timeline_analysis_with_scope,
)


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


def test_timeline_analysis_base_method_reroots_and_uses_matched_denominator() -> None:
    tree = build_flame_tree_from_collapsed(
        Counter(
            {
                (
                    "org.springframework.boot.SpringApplication.run;"
                    "com.company.batch.OrderJob.execute;"
                    "com.company.Service.process"
                ): 6,
                (
                    "org.springframework.boot.SpringApplication.run;"
                    "com.company.batch.OrderJob.execute;"
                    "oracle.jdbc.Statement.executeQuery"
                ): 4,
                "scheduler.IdleLoop.run;java.lang.Thread.sleep": 90,
            }
        )
    )

    rows, scope = build_timeline_analysis_with_scope(
        tree,
        interval_ms=100,
        elapsed_sec=10,
        timeline_base_method="OrderJob.execute",
    )

    by_segment = {row["segment"]: row for row in rows}
    assert "STARTUP_FRAMEWORK" not in by_segment
    assert by_segment["INTERNAL_METHOD"]["samples"] == 6
    assert by_segment["INTERNAL_METHOD"]["stage_ratio"] == 60.0
    assert by_segment["INTERNAL_METHOD"]["total_ratio"] == 6.0
    assert by_segment["SQL_EXECUTION"]["samples"] == 4
    assert by_segment["SQL_EXECUTION"]["stage_ratio"] == 40.0
    assert by_segment["SQL_EXECUTION"]["total_ratio"] == 4.0
    assert scope["base_samples"] == 10
    assert scope["base_ratio_of_total"] == 10.0
    assert scope["view_mode"] == "reroot_at_base_frame"
    assert by_segment["INTERNAL_METHOD"]["top_stacks"] == [
        {
            "name": "com.company.batch.OrderJob.execute;com.company.Service.process",
            "samples": 6,
        }
    ]


def test_timeline_analysis_base_method_not_found_returns_warning() -> None:
    tree = build_flame_tree_from_collapsed(
        Counter({"com.company.OtherJob.execute;com.company.Service.process": 3})
    )

    rows, scope = build_timeline_analysis_with_scope(
        tree,
        interval_ms=100,
        elapsed_sec=None,
        timeline_base_method="MissingJob.execute",
    )

    assert rows == []
    assert scope["base_samples"] == 0
    assert scope["warnings"] == [
        {
            "code": "TIMELINE_BASE_METHOD_NOT_FOUND",
            "message": "No profiler stack matched the configured timeline base method.",
        }
    ]


def test_analyze_collapsed_profile_emits_timeline_scope_metadata(tmp_path) -> None:
    path = tmp_path / "wall.collapsed"
    path.write_text(
        "launcher.Main;com.company.batch.OrderJob.execute;com.company.Service.process 7\n"
        "launcher.Main;scheduler.IdleLoop.run;java.lang.Thread.sleep 13\n",
        encoding="utf-8",
    )

    result = analyze_collapsed_profile(
        path,
        interval_ms=100,
        elapsed_sec=2,
        top_n=5,
        timeline_base_method="OrderJob.execute",
    )

    assert result.metadata["timeline_scope"]["base_samples"] == 7
    assert result.metadata["timeline_scope"]["total_samples"] == 20
    assert result.metadata["timeline_scope"]["base_ratio_of_total"] == 35.0
    assert result.series["timeline_analysis"][0]["stage_ratio"] == 100.0
