"""Tests for the Go goroutine-dump parser plugin (T-196 / T-197)."""
from __future__ import annotations

from archscope_engine.models.thread_snapshot import StackFrame, ThreadState
from archscope_engine.parsers.thread_dump.go_goroutine import (
    GoGoroutineParserPlugin,
    _infer_go_state,
    _normalize_go_frame,
)


_GO_DUMP = """\
goroutine 1 [running]:
main.main()
\t/app/main.go:42 +0x1a
created by main.start
\t/app/main.go:10 +0x33

goroutine 5 [chan receive, 5 minutes]:
main.worker(0xc0000a8000)
\t/app/main.go:88 +0x55

goroutine 17 [select]:
runtime.selectgo(0xc0000b0000, 0xc0000a0000)
\t/usr/local/go/src/runtime/select.go:328 +0x49b
main.background()
\t/app/main.go:120 +0xab

goroutine 24 [semacquire, 1 minutes]:
sync.runtime_Semacquire(0xc0000c0000)
\t/usr/local/go/src/runtime/sema.go:62 +0x42
sync.(*Mutex).Lock(0xc0000c0000)
\t/usr/local/go/src/sync/mutex.go:81 +0x90
"""


def test_can_parse_recognizes_goroutine_header() -> None:
    plugin = GoGoroutineParserPlugin()
    assert plugin.can_parse("goroutine 1 [running]:\n")
    assert plugin.can_parse("goroutine 99 [chan receive, 5 minutes]:")


def test_can_parse_rejects_non_goroutine_text() -> None:
    plugin = GoGoroutineParserPlugin()
    assert not plugin.can_parse('"worker" #1 nid=0x00ab\n')
    assert not plugin.can_parse("just a regular log line\n")


def test_parse_extracts_thread_count_and_states(tmp_path) -> None:
    dump = tmp_path / "go.dump"
    dump.write_text(_GO_DUMP, encoding="utf-8")

    bundle = GoGoroutineParserPlugin().parse(dump)

    assert bundle.language == "go"
    assert bundle.source_format == "go_goroutine"
    assert [snap.thread_name for snap in bundle.snapshots] == [
        "goroutine-1",
        "goroutine-5",
        "goroutine-17",
        "goroutine-24",
    ]
    # `[chan receive, 5 minutes]` → CHANNEL_WAIT via ThreadState.coerce()
    assert bundle.snapshots[1].state is ThreadState.CHANNEL_WAIT
    # Duration carried into metadata.
    assert bundle.snapshots[1].metadata["duration"] == "5 minutes"


def test_parse_extracts_frames_with_file_and_line(tmp_path) -> None:
    dump = tmp_path / "go.dump"
    dump.write_text(_GO_DUMP, encoding="utf-8")

    bundle = GoGoroutineParserPlugin().parse(dump)

    main_main = bundle.snapshots[0].stack_frames[0]
    assert main_main.module == "main"
    assert main_main.function == "main"
    assert main_main.file == "/app/main.go"
    assert main_main.line == 42
    assert main_main.language == "go"


def test_state_inference_promotes_goparked_to_channel_wait() -> None:
    frames = [
        StackFrame(module="runtime", function="gopark", language="go"),
    ]
    assert (
        _infer_go_state(ThreadState.RUNNABLE, frames) is ThreadState.CHANNEL_WAIT
    )


def test_state_inference_promotes_netpoll_to_network_wait() -> None:
    frames = [
        StackFrame(module="runtime", function="netpollblock", language="go"),
    ]
    assert (
        _infer_go_state(ThreadState.RUNNABLE, frames) is ThreadState.NETWORK_WAIT
    )


def test_state_inference_promotes_mutex_lock_to_lock_wait() -> None:
    frames = [
        StackFrame(
            module="sync.(*Mutex)",
            function="Lock",
            language="go",
        ),
    ]
    assert _infer_go_state(ThreadState.RUNNABLE, frames) is ThreadState.LOCK_WAIT


def test_normalize_collapses_anonymous_closure_chain() -> None:
    frame = StackFrame(
        module="github.com/myapp/handler",
        function="ServeHTTP.func1.func2",
        language="go",
    )
    cleaned = _normalize_go_frame(frame)
    assert cleaned.function == "ServeHTTP"


def test_normalize_collapses_gin_handler_func() -> None:
    frame = StackFrame(
        module="github.com/gin-gonic/gin",
        function="HandlerFunc.func1",
        language="go",
    )
    cleaned = _normalize_go_frame(frame)
    # `gin.HandlerFunc.func1` → `gin.HandlerFunc`
    assert cleaned.function == "HandlerFunc"


def test_full_parse_records_select_and_mutex_states(tmp_path) -> None:
    dump = tmp_path / "go.dump"
    dump.write_text(_GO_DUMP, encoding="utf-8")

    bundle = GoGoroutineParserPlugin().parse(dump)
    states = {snap.thread_name: snap.state for snap in bundle.snapshots}

    # goroutine 17 is `[select]` and the top frame is `runtime.selectgo`.
    assert states["goroutine-17"] is ThreadState.CHANNEL_WAIT
    # goroutine 24 is `[semacquire, 1 minutes]` with a sync.Mutex.Lock top frame.
    assert states["goroutine-24"] is ThreadState.LOCK_WAIT
