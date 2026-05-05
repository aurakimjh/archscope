from __future__ import annotations

import json
import textwrap
from pathlib import Path

from archscope_engine.analyzers.lock_contention_analyzer import analyze_lock_contention
from archscope_engine.analyzers.multi_thread_analyzer import analyze_multi_thread_dumps
from archscope_engine.models.thread_snapshot import ThreadState
from archscope_engine.parsers.thread_dump import DEFAULT_REGISTRY
from archscope_engine.parsers.thread_dump.java_jstack import JavaJstackParserPlugin


_SINGLE_DUMP = textwrap.dedent(
    """\
    Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

    "main" #1 prio=5 os_prio=0 tid=0x0001 nid=0x0001 runnable
       java.lang.Thread.State: RUNNABLE
    \tat com.acme.Main.run(Main.java:10)
    """
)


def test_registry_expands_multiple_jstack_sections_from_one_file(tmp_path: Path) -> None:
    path = tmp_path / "multi-jstack.txt"
    path.write_text(
        "2026-05-05 10:00:00\n"
        + _SINGLE_DUMP
        + "\n2026-05-05 10:00:05\n"
        + _SINGLE_DUMP.replace('"main"', '"worker"'),
        encoding="utf-8",
    )

    bundles = DEFAULT_REGISTRY.parse_many([path])

    assert len(bundles) == 2
    assert [bundle.dump_index for bundle in bundles] == [0, 1]
    assert [bundle.dump_label for bundle in bundles] == [
        "multi-jstack.txt#1",
        "multi-jstack.txt#2",
    ]
    assert bundles[0].metadata["raw_timestamp"] == "2026-05-05 10:00:00"
    assert bundles[1].metadata["raw_timestamp"] == "2026-05-05 10:00:05"
    assert [bundle.snapshots[0].thread_name for bundle in bundles] == [
        "main",
        "worker",
    ]


def test_registry_detects_utf16_jstack_head(tmp_path: Path) -> None:
    path = tmp_path / "utf16-jstack.txt"
    path.write_text(_SINGLE_DUMP, encoding="utf-16")

    bundle = DEFAULT_REGISTRY.parse_one(path)

    assert bundle.source_format == "java_jstack"
    assert bundle.snapshots[0].thread_name == "main"


def test_java_jcmd_json_parser_routes_and_promotes_network_wait(tmp_path: Path) -> None:
    path = tmp_path / "jcmd.json"
    path.write_text(
        json.dumps(
            {
                "time": "2026-05-05T10:00:00Z",
                "threadDump": {
                    "threadContainers": [
                        {
                            "threads": [
                                {
                                    "name": "http-client",
                                    "tid": 7,
                                    "state": "RUNNABLE",
                                    "stack": [
                                        "java.net.SocketInputStream.socketRead0(Native Method)",
                                        "com.acme.HttpClient.call(HttpClient.java:22)",
                                    ],
                                }
                            ]
                        }
                    ]
                }
            }
        ),
        encoding="utf-8",
    )

    bundle = DEFAULT_REGISTRY.parse_one(path)

    assert bundle.source_format == "java_jcmd_json"
    assert bundle.language == "java"
    assert bundle.captured_at is not None
    assert bundle.snapshots[0].thread_name == "http-client"
    assert bundle.snapshots[0].state is ThreadState.NETWORK_WAIT


def test_java_jcmd_json_parser_reports_missing_thread_dump(tmp_path: Path) -> None:
    path = tmp_path / "not-jcmd.json"
    path.write_text(json.dumps({"threads": []}), encoding="utf-8")

    try:
        DEFAULT_REGISTRY.parse_one(path, format_override="java_jcmd_json")
    except ValueError as error:
        assert "missing required 'threadDump' object" in str(error)
    else:
        raise AssertionError("Expected missing threadDump to be rejected.")


def test_jstack_virtual_carrier_pinning_surfaces_in_multi_analyzer(tmp_path: Path) -> None:
    path = tmp_path / "virtual.txt"
    path.write_text(
        textwrap.dedent(
            """\
            Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

            "ForkJoinPool-1-worker-1" #20 prio=5 os_prio=0 tid=0x0020 nid=0x0020 runnable
               java.lang.Thread.State: RUNNABLE
               Carrying virtual thread #123
            \tat java.lang.VirtualThread.parkOnCarrierThread(VirtualThread.java:680)
            \tat com.acme.BlockingService.call(BlockingService.java:77)
            """
        ),
        encoding="utf-8",
    )
    bundle = JavaJstackParserPlugin().parse(path)
    result = analyze_multi_thread_dumps([bundle])

    assert result.summary["virtual_thread_carrier_pinning"] == 1
    row = result.tables["virtual_thread_carrier_pinning"][0]
    assert row["thread_name"] == "ForkJoinPool-1-worker-1"
    assert row["candidate_method"] == "com.acme.BlockingService.call (BlockingService.java:77)"
    assert "VIRTUAL_THREAD_CARRIER_PINNING" in [
        finding["code"] for finding in result.metadata["findings"]
    ]


def test_jstack_smr_native_and_class_histogram_tables(tmp_path: Path) -> None:
    path = tmp_path / "metadata.txt"
    path.write_text(
        textwrap.dedent(
            """\
            Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

            "native-reader" #1 prio=5 os_prio=0 tid=0x0001 nid=0x0001 runnable
               java.lang.Thread.State: RUNNABLE
            \tat java.io.FileInputStream.readBytes(Native Method)
            \tat com.acme.Reader.read(Reader.java:12)

            Threads class SMR info:
              unresolved zombie thread 0x000000001234abcd

             num     #instances         #bytes  class name
            -------------------------------------------------------
               1:           100           2400  java.lang.String
               2:            10           1600  com.acme.Order
            Total           110           4000
            """
        ),
        encoding="utf-8",
    )
    bundles = DEFAULT_REGISTRY.parse_many([path])
    result = analyze_multi_thread_dumps(bundles)

    assert result.summary["smr_unresolved_threads"] == 1
    assert result.tables["smr_unresolved_threads"][0]["line"] == (
        "unresolved zombie thread 0x000000001234abcd"
    )
    assert result.tables["native_method_threads"][0]["native_method"] == (
        "java.io.FileInputStream.readBytes(Native Method)"
    )
    assert result.tables["class_histogram_top_classes"][0]["class_name"] == (
        "java.lang.String"
    )
    assert "SMR_UNRESOLVED_THREAD" in [
        finding["code"] for finding in result.metadata["findings"]
    ]


def test_jstack_class_histogram_limit_is_configurable(tmp_path: Path) -> None:
    path = tmp_path / "histogram-limit.txt"
    path.write_text(
        textwrap.dedent(
            """\
            Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

            "main" #1 tid=0x0001 nid=0x0001 runnable
               java.lang.Thread.State: RUNNABLE
            \tat com.acme.Main.run(Main.java:1)

             num     #instances         #bytes  class name
               1:           100           3000  com.acme.A
               2:           100           2000  com.acme.B
               3:           100           1000  com.acme.C
            Total           300           6000
            """
        ),
        encoding="utf-8",
    )

    bundles = DEFAULT_REGISTRY.parse_many(
        [path],
        parser_options={"class_histogram_limit": 2},
    )
    histogram = bundles[0].metadata["class_histogram"]

    assert histogram["row_limit"] == 2
    assert histogram["total_rows"] == 3
    assert histogram["truncated"] is True
    assert [row["class_name"] for row in histogram["classes"]] == [
        "com.acme.A",
        "com.acme.B",
    ]


def test_object_wait_is_not_reported_as_lock_contention(tmp_path: Path) -> None:
    path = tmp_path / "object-wait.txt"
    path.write_text(
        textwrap.dedent(
            """\
            Full thread dump OpenJDK 64-Bit Server VM (21.0.1 mixed mode):

            "obj-waiter" #4 prio=5 os_prio=0 tid=0x0004 nid=0x0004 in Object.wait
               java.lang.Thread.State: TIMED_WAITING (on object monitor)
            \tat java.lang.Object.wait(Native Method)
            \t- waiting on <0x00000007dddddddd> (a java.lang.Object)
            """
        ),
        encoding="utf-8",
    )
    bundle = JavaJstackParserPlugin().parse(path)
    waiter = bundle.snapshots[0]

    assert waiter.lock_waiting is not None
    assert waiter.lock_waiting.wait_mode == "object_wait"
    assert waiter.metadata["monitor_wait_mode"] == "object_wait"

    result = analyze_lock_contention(bundle)
    assert result.summary["threads_waiting_on_lock"] == 1
    assert result.summary["contended_locks"] == 0
    assert result.metadata["findings"] == []
