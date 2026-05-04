"""Regression tests for the multi-language thread-dump framework
(T-188 / T-189 / T-190 / T-191).
"""
from __future__ import annotations

import pytest

from archscope_engine.analyzers.multi_thread_analyzer import (
    analyze_multi_thread_dumps,
)
from archscope_engine.models.thread_snapshot import (
    StackFrame,
    ThreadDumpBundle,
    ThreadSnapshot,
    ThreadState,
)
from archscope_engine.parsers.thread_dump import (
    DEFAULT_REGISTRY,
    JavaJstackParserPlugin,
    MixedFormatError,
    ParserRegistry,
    UnknownFormatError,
)

# ---------------------------------------------------------------------------
# T-188: model behavior
# ---------------------------------------------------------------------------


def test_thread_state_coerces_aliases() -> None:
    assert ThreadState.coerce("RUNNING") is ThreadState.RUNNABLE
    assert ThreadState.coerce("Sleeping") is ThreadState.TIMED_WAITING
    assert ThreadState.coerce("chan receive") is ThreadState.CHANNEL_WAIT
    assert ThreadState.coerce(None) is ThreadState.UNKNOWN
    assert ThreadState.coerce("nonsense") is ThreadState.UNKNOWN


def test_thread_snapshot_signature_uses_top_frames() -> None:
    snapshot = ThreadSnapshot(
        snapshot_id="java::0::worker",
        thread_name="worker",
        thread_id="0xabc",
        state=ThreadState.RUNNABLE,
        category="RUNNABLE",
        stack_frames=[
            StackFrame(function="run", module="com.example.Worker", language="java"),
            StackFrame(function="exec", module="com.example.Pool", language="java"),
        ],
        language="java",
        source_format="java_jstack",
    )
    assert snapshot.stack_signature() == (
        "com.example.Worker.run | com.example.Pool.exec"
    )


# ---------------------------------------------------------------------------
# T-189 + T-190: registry + Java jstack plugin
# ---------------------------------------------------------------------------


_JSTACK_SAMPLE = """\
2025-05-04 10:00:00
Full thread dump OpenJDK 64-Bit Server VM (17.0.1+12 mixed mode):

"worker-1" #12 prio=5 os_prio=0 tid=0x00007f0001 nid=0x00ab runnable [0x00007f]
   java.lang.Thread.State: RUNNABLE
\tat com.example.Worker.run(Worker.java:42)
\tat com.example.Pool.exec(Pool.java:88)

"worker-2" #13 prio=5 os_prio=0 tid=0x00007f0002 nid=0x00ac waiting for monitor entry
   java.lang.Thread.State: BLOCKED (on object monitor)
\tat com.example.Worker.run(Worker.java:42)
\tat com.example.Pool.exec(Pool.java:88)
\t- waiting to lock <0x000000076ab62208> (a com.example.Pool)
"""


def test_java_jstack_plugin_detects_full_thread_header() -> None:
    plugin = JavaJstackParserPlugin()
    assert plugin.can_parse(_JSTACK_SAMPLE)


def test_java_jstack_plugin_detects_quoted_nid_only() -> None:
    plugin = JavaJstackParserPlugin()
    head = '"worker" #1 nid=0x00ab runnable\n   java.lang.Thread.State: RUNNABLE\n'
    assert plugin.can_parse(head)


def test_java_jstack_plugin_rejects_random_text() -> None:
    plugin = JavaJstackParserPlugin()
    assert not plugin.can_parse("nothing about JVM here")


def test_default_registry_picks_up_java_plugin() -> None:
    plugin = DEFAULT_REGISTRY.get("java_jstack")
    assert isinstance(plugin, JavaJstackParserPlugin)


def test_registry_parses_java_dump_into_bundle(tmp_path) -> None:
    dump = tmp_path / "td-1.txt"
    dump.write_text(_JSTACK_SAMPLE, encoding="utf-8")

    bundle = DEFAULT_REGISTRY.parse_one(dump, dump_index=0)

    assert bundle.source_format == "java_jstack"
    assert bundle.language == "java"
    assert bundle.dump_label == "td-1.txt"
    assert [snap.thread_name for snap in bundle.snapshots] == ["worker-1", "worker-2"]
    assert bundle.snapshots[0].state is ThreadState.RUNNABLE
    assert bundle.snapshots[1].state is ThreadState.BLOCKED
    first_frame = bundle.snapshots[0].stack_frames[0]
    assert first_frame.module == "com.example.Worker"
    assert first_frame.function == "run"
    assert first_frame.file == "Worker.java"
    assert first_frame.line == 42
    assert first_frame.language == "java"


def test_registry_rejects_unknown_format(tmp_path) -> None:
    junk = tmp_path / "garbage.txt"
    junk.write_text("This is not a thread dump.\n", encoding="utf-8")
    with pytest.raises(UnknownFormatError):
        DEFAULT_REGISTRY.parse_one(junk)


def test_registry_format_override_skips_detection(tmp_path) -> None:
    junk = tmp_path / "headerless.txt"
    junk.write_text(_JSTACK_SAMPLE.split("Full thread dump")[1], encoding="utf-8")
    bundle = DEFAULT_REGISTRY.parse_one(junk, format_override="java_jstack")
    assert [snap.thread_name for snap in bundle.snapshots] == ["worker-1", "worker-2"]


def test_registry_rejects_mixed_formats(tmp_path) -> None:
    fake_format_registry = ParserRegistry()
    fake_format_registry.register(JavaJstackParserPlugin())

    class FakePlugin:
        format_id = "fake"
        language = "fake"

        def can_parse(self, head: str) -> bool:
            return head.startswith("FAKE_DUMP")

        def parse(self, path):
            return ThreadDumpBundle(
                snapshots=[],
                source_file=str(path),
                source_format=self.format_id,
                language=self.language,
            )

    fake_format_registry.register(FakePlugin())

    java_dump = tmp_path / "j.txt"
    java_dump.write_text(_JSTACK_SAMPLE, encoding="utf-8")
    fake_dump = tmp_path / "fake.txt"
    fake_dump.write_text("FAKE_DUMP\n", encoding="utf-8")

    with pytest.raises(MixedFormatError):
        fake_format_registry.parse_many([java_dump, fake_dump])


# ---------------------------------------------------------------------------
# T-191: multi-dump correlator
# ---------------------------------------------------------------------------


def _bundle(dump_index: int, snapshots: list[ThreadSnapshot]) -> ThreadDumpBundle:
    return ThreadDumpBundle(
        snapshots=snapshots,
        source_file=f"dump-{dump_index}.txt",
        source_format="java_jstack",
        language="java",
        dump_index=dump_index,
        dump_label=f"dump-{dump_index}",
    )


def _java_snapshot(name: str, state: ThreadState, stack: list[str]) -> ThreadSnapshot:
    return ThreadSnapshot(
        snapshot_id=f"java::{name}",
        thread_name=name,
        thread_id=None,
        state=state,
        category=state.value,
        stack_frames=[StackFrame(function=frame, language="java") for frame in stack],
        language="java",
        source_format="java_jstack",
    )


def test_multi_analyzer_emits_long_running_for_three_consecutive_runnable() -> None:
    bundles = [
        _bundle(
            i,
            [_java_snapshot("worker-1", ThreadState.RUNNABLE, ["compute", "loop"])],
        )
        for i in range(3)
    ]

    result = analyze_multi_thread_dumps(bundles)

    findings = result.metadata["findings"]
    codes = [f["code"] for f in findings]
    assert "LONG_RUNNING_THREAD" in codes
    assert result.summary["long_running_threads"] == 1
    assert result.summary["total_dumps"] == 3
    assert result.summary["unique_threads"] == 1
    assert result.summary["languages_detected"] == ["java"]


def test_multi_analyzer_emits_persistent_blocked_for_three_consecutive_blocked() -> None:
    bundles = [
        _bundle(i, [_java_snapshot("blocker", ThreadState.BLOCKED, ["lock"])])
        for i in range(3)
    ]

    result = analyze_multi_thread_dumps(bundles)

    codes = [f["code"] for f in result.metadata["findings"]]
    assert "PERSISTENT_BLOCKED_THREAD" in codes
    assert result.summary["persistent_blocked_threads"] == 1


def test_multi_analyzer_skips_when_run_breaks() -> None:
    bundles = [
        _bundle(
            0,
            [_java_snapshot("worker", ThreadState.RUNNABLE, ["compute"])],
        ),
        _bundle(
            1,
            [_java_snapshot("worker", ThreadState.WAITING, ["park"])],
        ),
        _bundle(
            2,
            [_java_snapshot("worker", ThreadState.RUNNABLE, ["compute"])],
        ),
    ]

    result = analyze_multi_thread_dumps(bundles)

    assert result.summary["long_running_threads"] == 0
    assert result.summary["persistent_blocked_threads"] == 0


def test_multi_analyzer_state_distribution_records_each_dump() -> None:
    bundles = [
        _bundle(
            0,
            [
                _java_snapshot("a", ThreadState.RUNNABLE, ["x"]),
                _java_snapshot("b", ThreadState.BLOCKED, ["y"]),
            ],
        ),
        _bundle(
            1,
            [_java_snapshot("a", ThreadState.RUNNABLE, ["x"])],
        ),
    ]
    result = analyze_multi_thread_dumps(bundles)
    distribution = result.series["state_distribution_per_dump"]
    assert distribution[0]["counts"]["RUNNABLE"] == 1
    assert distribution[0]["counts"]["BLOCKED"] == 1
    assert distribution[1]["counts"]["RUNNABLE"] == 1


def test_multi_analyzer_requires_at_least_one_bundle() -> None:
    with pytest.raises(ValueError):
        analyze_multi_thread_dumps([])
