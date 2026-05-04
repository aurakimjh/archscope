"""Regression tests for Java jstack post-parse enrichment (T-194 / T-195)."""
from __future__ import annotations

from archscope_engine.models.thread_snapshot import StackFrame, ThreadState
from archscope_engine.parsers.thread_dump.java_jstack import (
    JavaJstackParserPlugin,
    _infer_java_state,
    _normalize_proxy_frame,
)


def _java_frame(module: str | None, function: str) -> StackFrame:
    return StackFrame(module=module, function=function, language="java")


# ---------------------------------------------------------------------------
# T-194 — proxy class normalization
# ---------------------------------------------------------------------------


def test_normalize_strips_cglib_enhancer_suffix() -> None:
    frame = _java_frame(
        module="com.example.MyService$$EnhancerByCGLIB$$abc123",
        function="businessMethod",
    )
    cleaned = _normalize_proxy_frame(frame)
    assert cleaned.module == "com.example.MyService"
    assert cleaned.function == "businessMethod"


def test_normalize_strips_fastclass_cglib_suffix() -> None:
    frame = _java_frame(
        module="com.example.Worker$$FastClassByCGLIB$$deadbeef",
        function="invoke",
    )
    cleaned = _normalize_proxy_frame(frame)
    assert cleaned.module == "com.example.Worker"


def test_normalize_collapses_jdk_proxy_digits() -> None:
    frame = _java_frame(module="com.sun.proxy.$$Proxy42", function="handle")
    cleaned = _normalize_proxy_frame(frame)
    assert cleaned.module == "com.sun.proxy.$$Proxy"


def test_normalize_collapses_generated_method_accessor_digits() -> None:
    frame = _java_frame(
        module="sun.reflect.GeneratedMethodAccessor1234",
        function="invoke",
    )
    cleaned = _normalize_proxy_frame(frame)
    assert cleaned.module == "sun.reflect.GeneratedMethodAccessor"


def test_normalize_leaves_plain_frames_unchanged() -> None:
    frame = _java_frame(module="com.example.Plain", function="run")
    assert _normalize_proxy_frame(frame) is frame


def test_normalize_skips_non_java_frames() -> None:
    frame = StackFrame(
        module="some$$EnhancerByCGLIB$$x",
        function="run",
        language="go",
    )
    # Only Java frames should be touched.
    assert _normalize_proxy_frame(frame) is frame


def test_two_proxy_variants_collapse_to_same_signature() -> None:
    a = _java_frame(
        module="com.example.PaymentService$$EnhancerByCGLIB$$aaaa1111",
        function="charge",
    )
    b = _java_frame(
        module="com.example.PaymentService$$EnhancerByCGLIB$$bbbb2222",
        function="charge",
    )
    assert _normalize_proxy_frame(a).module == _normalize_proxy_frame(b).module


# ---------------------------------------------------------------------------
# T-195 — JVM network/IO state inference
# ---------------------------------------------------------------------------


def test_infer_state_promotes_runnable_epoll_to_network_wait() -> None:
    frames = [
        _java_frame(module="sun.nio.ch.EPoll", function="epollWait"),
        _java_frame(module="sun.nio.ch.EPollSelectorImpl", function="doSelect"),
    ]
    assert (
        _infer_java_state(ThreadState.RUNNABLE, frames) is ThreadState.NETWORK_WAIT
    )


def test_infer_state_promotes_socket_read0_to_network_wait() -> None:
    frames = [
        _java_frame(module="java.net.SocketInputStream", function="socketRead0"),
    ]
    assert (
        _infer_java_state(ThreadState.RUNNABLE, frames) is ThreadState.NETWORK_WAIT
    )


def test_infer_state_promotes_file_input_stream_to_io_wait() -> None:
    frames = [
        _java_frame(module="java.io.FileInputStream", function="readBytes"),
        _java_frame(module="com.example.Loader", function="load"),
    ]
    assert _infer_java_state(ThreadState.RUNNABLE, frames) is ThreadState.IO_WAIT


def test_infer_state_keeps_blocked_threads_blocked() -> None:
    # A thread reported as BLOCKED with a socketRead0 top frame should stay
    # BLOCKED — the runtime knows better than our heuristic.
    frames = [
        _java_frame(module="java.net.SocketInputStream", function="socketRead0"),
    ]
    assert _infer_java_state(ThreadState.BLOCKED, frames) is ThreadState.BLOCKED


def test_infer_state_leaves_runnable_unrelated_top_frame_alone() -> None:
    frames = [_java_frame(module="com.example.Worker", function="compute")]
    assert _infer_java_state(ThreadState.RUNNABLE, frames) is ThreadState.RUNNABLE


# ---------------------------------------------------------------------------
# End-to-end: enrichment runs through the plugin
# ---------------------------------------------------------------------------


_DUMP_WITH_PROXY = """\
2025-05-04 10:00:00
Full thread dump OpenJDK 64-Bit Server VM (17.0.1+12 mixed mode):

"http-nio-8080-exec-1" #25 prio=5 os_prio=0 tid=0x00007f001 nid=0x00aa runnable [0x00007f]
   java.lang.Thread.State: RUNNABLE
\tat sun.nio.ch.EPoll.epollWait(EPoll.java:42)
\tat com.example.PaymentService$$EnhancerByCGLIB$$abc123.charge(MyService.java:88)
\tat com.example.PaymentController.handle(PaymentController.java:12)
"""


def test_plugin_applies_proxy_normalization_and_state_inference(tmp_path) -> None:
    dump = tmp_path / "td.txt"
    dump.write_text(_DUMP_WITH_PROXY, encoding="utf-8")
    bundle = JavaJstackParserPlugin().parse(dump)

    assert len(bundle.snapshots) == 1
    snapshot = bundle.snapshots[0]
    # T-195: epollWait promoted RUNNABLE → NETWORK_WAIT.
    assert snapshot.state is ThreadState.NETWORK_WAIT
    # T-194: CGLIB hash is stripped.
    proxy_frame = snapshot.stack_frames[1]
    assert proxy_frame.module == "com.example.PaymentService"
    assert proxy_frame.function == "charge"
