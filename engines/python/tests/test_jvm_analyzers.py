from pathlib import Path

from archscope_engine.analyzers.exception_analyzer import analyze_exception_stack
from archscope_engine.analyzers.gc_log_analyzer import analyze_gc_log, build_gc_log_result
from archscope_engine.analyzers.thread_dump_analyzer import analyze_thread_dump
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.parsers.exception_parser import parse_exception_stack
from archscope_engine.parsers.gc_log_parser import parse_gc_log
from archscope_engine.parsers.thread_dump_parser import parse_thread_dump


def test_gc_log_parser_hotspot_unified_sample() -> None:
    sample = Path(__file__).parents[3] / "examples/gc-logs/sample-hotspot-gc.log"
    diagnostics = ParserDiagnostics()

    events = parse_gc_log(sample, diagnostics=diagnostics)

    assert len(events) == 2
    assert events[0].gc_type == "Pause Young"
    assert events[0].cause == "G1 Evacuation Pause"
    assert events[1].gc_type == "Pause Full"
    assert events[1].pause_ms == 190.120
    assert diagnostics.parsed_records == 2


def test_gc_log_analyzer_returns_analysis_result() -> None:
    sample = Path(__file__).parents[3] / "examples/gc-logs/sample-hotspot-gc.log"

    result = analyze_gc_log(sample)

    assert result.type == "gc_log"
    assert result.summary["total_events"] == 2
    assert result.summary["full_gc_count"] == 1
    assert result.metadata["diagnostics"]["parsed_records"] == 2
    assert {finding["code"] for finding in result.metadata["findings"]} == {
        "LONG_GC_PAUSE",
        "FULL_GC_PRESENT",
    }


def test_gc_log_result_builder_accepts_event_iterable() -> None:
    sample = Path(__file__).parents[3] / "examples/gc-logs/sample-hotspot-gc.log"
    events = parse_gc_log(sample)

    result = build_gc_log_result(
        (event for event in events),
        source_file=sample,
        diagnostics={"parsed_records": len(events)},
    )

    assert result.summary["total_events"] == 2
    assert len(result.series["pause_timeline"]) == 2
    assert len(result.tables["events"]) == 2


def test_thread_dump_parser_and_analyzer(tmp_path) -> None:
    path = tmp_path / "thread-dump.txt"
    path.write_text(
        '"http-nio-8080-exec-1" #31 tid=0x1 nid=0x101 waiting for monitor entry\n'
        "   java.lang.Thread.State: BLOCKED (on object monitor)\n"
        "        at com.example.OrderService.find(OrderService.java:42)\n"
        "        - waiting to lock <0x00000001>\n\n"
        '"HikariPool-1 housekeeper" #32 tid=0x2 nid=0x102 waiting on condition\n'
        "   java.lang.Thread.State: TIMED_WAITING (parking)\n"
        "        at jdk.internal.misc.Unsafe.park(Native Method)\n",
        encoding="utf-8",
    )

    records = parse_thread_dump(path)
    result = analyze_thread_dump(path)

    assert len(records) == 2
    assert records[0].category == "BLOCKED"
    assert result.type == "thread_dump"
    assert result.summary["blocked_threads"] == 1
    assert result.summary["waiting_threads"] == 1
    assert result.metadata["findings"][0]["code"] == "BLOCKED_THREADS_PRESENT"


def test_exception_parser_and_analyzer(tmp_path) -> None:
    path = tmp_path / "exception.log"
    path.write_text(
        "2026-04-27T10:00:00+09:00 java.lang.IllegalStateException: failed\n"
        "    at com.example.OrderController.create(OrderController.java:15)\n"
        "Caused by: java.sql.SQLTimeoutException: timeout\n"
        "    at oracle.jdbc.driver.T4CPreparedStatement.executeQuery(T4CPreparedStatement.java:1)\n"
        "java.lang.IllegalStateException: failed\n"
        "    at com.example.OrderController.create(OrderController.java:15)\n",
        encoding="utf-8",
    )

    records = parse_exception_stack(path)
    result = analyze_exception_stack(path)

    assert len(records) == 2
    assert records[0].root_cause == "java.sql.SQLTimeoutException"
    assert result.type == "exception_stack"
    assert result.summary["total_exceptions"] == 2
    assert result.summary["unique_signatures"] == 1
    assert {finding["code"] for finding in result.metadata["findings"]} == {
        "REPEATED_EXCEPTION_SIGNATURE",
        "ROOT_CAUSE_PRESENT",
    }
