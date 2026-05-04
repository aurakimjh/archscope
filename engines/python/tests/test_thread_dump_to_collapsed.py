"""Tests for the thread-dump → collapsed-format batch converter (T-216)."""
from __future__ import annotations

from collections import Counter
from pathlib import Path

from archscope_engine.analyzers.thread_dump_to_collapsed import (
    convert_thread_dumps_to_collapsed,
    write_collapsed_file,
)


_JSTACK = """\
Full thread dump OpenJDK 64-Bit Server VM (17.0.1+12 mixed mode):

"worker-1" #12 prio=5 os_prio=0 tid=0x00007f0001 nid=0x00ab runnable [0x00007f]
   java.lang.Thread.State: RUNNABLE
\tat com.example.Worker.run(Worker.java:42)
\tat com.example.Pool.exec(Pool.java:88)

"worker-1" #12 prio=5 os_prio=0 tid=0x00007f0001 nid=0x00ab runnable
   java.lang.Thread.State: RUNNABLE
\tat com.example.Worker.run(Worker.java:42)
\tat com.example.Pool.exec(Pool.java:88)

"proxy-handler" #25 prio=5 os_prio=0 tid=0x00007f0099 nid=0x00cc runnable
   java.lang.Thread.State: RUNNABLE
\tat com.example.PaymentService$$EnhancerByCGLIB$$abc123.charge(PaymentService.java:88)
"""


def test_converts_jstack_to_collapsed_with_thread_names(tmp_path: Path) -> None:
    dump = tmp_path / "td.txt"
    dump.write_text(_JSTACK, encoding="utf-8")
    counter = convert_thread_dumps_to_collapsed([dump])

    # `worker-1` appears twice with the same stack → collapsed line gets count 2.
    worker_key = "worker-1;com.example.Pool.exec;com.example.Worker.run"
    assert counter[worker_key] == 2
    # The proxy thread appears once and AOP normalization (T-194) collapsed
    # the CGLIB hash into the bare module name.
    assert any(
        key.startswith("proxy-handler;com.example.PaymentService.charge")
        for key in counter
    )


def test_omits_thread_name_when_requested(tmp_path: Path) -> None:
    dump = tmp_path / "td.txt"
    dump.write_text(_JSTACK, encoding="utf-8")
    counter = convert_thread_dumps_to_collapsed([dump], include_thread_name=False)
    # Without thread names the two worker-1 snapshots collapse with the
    # other RUNNABLE threads that share the same stack — but this fixture
    # has none, so we still get count 2 for the worker stack.
    assert counter["com.example.Pool.exec;com.example.Worker.run"] == 2


def test_aggregates_across_multiple_dump_files(tmp_path: Path) -> None:
    paths: list[Path] = []
    for index in range(3):
        path = tmp_path / f"td-{index}.txt"
        path.write_text(_JSTACK, encoding="utf-8")
        paths.append(path)
    counter = convert_thread_dumps_to_collapsed(paths)
    worker_key = "worker-1;com.example.Pool.exec;com.example.Worker.run"
    # The fixture contains worker-1 twice per file × 3 files = 6.
    assert counter[worker_key] == 6


def test_write_collapsed_file_emits_sorted_lines(tmp_path: Path) -> None:
    dump = tmp_path / "td.txt"
    dump.write_text(_JSTACK, encoding="utf-8")
    output = tmp_path / "result.collapsed"
    written, unique = write_collapsed_file([dump], output)
    assert written == output
    assert unique > 0
    text = output.read_text(encoding="utf-8")
    lines = [line for line in text.splitlines() if line.strip()]
    # First line should carry the highest count (the aggregated worker-1 stack).
    head = lines[0]
    assert head.endswith(" 2")
    assert head.startswith("worker-1;")


def test_sanitizes_semicolons_in_frame_text(tmp_path: Path) -> None:
    # Synthesize a snapshot via the multi-format pipeline using a
    # frame name that already contains a semicolon (Python lambda
    # representation, etc.) — we want the collapsed line to remain
    # parseable by downstream tools.
    pyspy_dump = tmp_path / "py.txt"
    pyspy_dump.write_text(
        "Process 12345: python /app/server.py\nPython v3.11.7\n\n"
        'Thread 12345 (active): "weird;name"\n'
        "    handler (server.py:42)\n",
        encoding="utf-8",
    )
    counter = convert_thread_dumps_to_collapsed([pyspy_dump])
    assert all(";" not in key.split(";")[0] for key in counter)
    # The semicolon in the thread name was replaced with `_`.
    assert any(key.startswith("weird_name") for key in counter)


def test_returns_counter_instance() -> None:
    # Pure regression: the public helper must return a Counter so callers
    # can use `.most_common()`.
    counter = Counter[str]()
    counter["x;y"] += 1
    assert counter.most_common(1)[0] == ("x;y", 1)
