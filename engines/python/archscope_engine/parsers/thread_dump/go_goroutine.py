"""Go goroutine-dump parser plugin (T-196) + Go-only enrichment (T-197).

Handles the dump format produced by ``runtime/debug.Stack()``,
``runtime.Stack`` (panic / SIGQUIT), and ``GODEBUG=schedtrace`` style
output. Each goroutine block looks like::

    goroutine 17 [chan receive, 5 minutes]:
    main.worker(0xc0000a8000)
        /app/main.go:88 +0x55
    created by main.start
        /app/main.go:10 +0x33

State inference (T-197) promotes Go runtime states that the multi-dump
correlator should not treat as "running": ``netpoll``/``netpollBlock``
→ NETWORK_WAIT, ``selectgo`` / chan ops → CHANNEL_WAIT,
``semacquire`` / mutex → LOCK_WAIT. Framework cleanup strips wrapper
methods that obscure the real call site.
"""
from __future__ import annotations

import re
from dataclasses import replace
from pathlib import Path

from archscope_engine.models.thread_snapshot import (
    StackFrame,
    ThreadDumpBundle,
    ThreadSnapshot,
    ThreadState,
)
from archscope_engine.parsers.thread_dump.registry import DEFAULT_REGISTRY

_GOROUTINE_HEADER_RE = re.compile(
    r"^goroutine\s+(?P<id>\d+)\s+\[(?P<status>[^\]]+)\]:\s*$"
)
_FRAME_LOCATION_RE = re.compile(
    r"^\t(?P<file>[^:]+\.go):(?P<line>\d+)(?:\s+\+0x[0-9a-fA-F]+)?\s*$"
)


class GoGoroutineParserPlugin:
    """Parse goroutine dumps from runtime.Stack / panic / debug.Stack."""

    format_id: str = "go_goroutine"
    language: str = "go"

    def can_parse(self, head: str) -> bool:
        return bool(re.search(r"^goroutine\s+\d+\s+\[\w", head, re.MULTILINE))

    def parse(self, path: Path) -> ThreadDumpBundle:
        text = Path(path).read_text(encoding="utf-8", errors="replace")
        snapshots: list[ThreadSnapshot] = []
        for index, block in enumerate(_split_goroutine_blocks(text)):
            snapshot = _parse_block(block, index)
            if snapshot is not None:
                snapshots.append(snapshot)
        return ThreadDumpBundle(
            snapshots=snapshots,
            source_file=str(path),
            source_format=self.format_id,
            language=self.language,
        )


def _split_goroutine_blocks(text: str) -> list[list[str]]:
    blocks: list[list[str]] = []
    current: list[str] = []
    for line in text.splitlines():
        if _GOROUTINE_HEADER_RE.match(line.rstrip("\r")):
            if current:
                blocks.append(current)
            current = [line]
        elif current:
            current.append(line)
    if current:
        blocks.append(current)
    return blocks


def _parse_block(block: list[str], index: int) -> ThreadSnapshot | None:
    header = _GOROUTINE_HEADER_RE.match(block[0].rstrip("\r"))
    if not header:
        return None
    goroutine_id = header.group("id")
    status = header.group("status")
    state_str, _, duration = status.partition(",")
    state_str = state_str.strip()
    duration = duration.strip() or None
    state = ThreadState.coerce(state_str)

    frames: list[StackFrame] = []
    body = block[1:]
    iterator = iter(range(len(body)))
    for index_in_body in iterator:
        line = body[index_in_body].rstrip("\r")
        if not line.strip():
            continue
        if line.startswith("created by "):
            # Add the "created by" parent frame as the *bottom* of the stack
            # for context, then break — anything after this is duplicated.
            parent_text = line[len("created by ") :].strip()
            location = _next_location(body, index_in_body + 1)
            frames.append(
                _build_frame(
                    function_text=parent_text,
                    file=location[0] if location else None,
                    line_no=location[1] if location else None,
                )
            )
            break
        if line.startswith("\t"):
            # Stray location without a preceding function call line — skip.
            continue
        location = _next_location(body, index_in_body + 1)
        frames.append(
            _build_frame(
                function_text=line.strip(),
                file=location[0] if location else None,
                line_no=location[1] if location else None,
            )
        )
        # Fast-forward iterator past the location line we just consumed.
        if location is not None:
            try:
                next(iterator)
            except StopIteration:
                break

    frames = [_normalize_go_frame(frame) for frame in frames]
    state = _infer_go_state(state, frames)

    metadata: dict[str, object] = {"raw_status": status}
    if duration:
        metadata["duration"] = duration

    return ThreadSnapshot(
        snapshot_id=f"go::{index}::goroutine-{goroutine_id}",
        thread_name=f"goroutine-{goroutine_id}",
        thread_id=goroutine_id,
        state=state,
        category=state.value,
        stack_frames=frames,
        lock_info=None,
        metadata=metadata,
        language="go",
        source_format="go_goroutine",
    )


def _next_location(body: list[str], start: int) -> tuple[str, int] | None:
    if start >= len(body):
        return None
    match = _FRAME_LOCATION_RE.match(body[start].rstrip("\r"))
    if not match:
        return None
    try:
        line_no = int(match.group("line"))
    except ValueError:
        return None
    return (match.group("file"), line_no)


def _build_frame(function_text: str, file: str | None, line_no: int | None) -> StackFrame:
    function = function_text
    module: str | None = None
    open_paren = function_text.find("(")
    if open_paren > 0:
        function = function_text[:open_paren]
    if "." in function:
        # Go reports "package.Type.Method" or "package.func"; the module is
        # everything up to the last dot.
        module, _, function = function.rpartition(".")
    return StackFrame(
        function=function,
        module=module,
        file=file,
        line=line_no,
        language="go",
    )


# ---------------------------------------------------------------------------
# T-197 — Go framework cleanup + state inference
# ---------------------------------------------------------------------------

# Framework wrappers whose presence at the top of the stack obscures the
# real handler. We collapse closures like `gin.HandlerFunc.func1` into the
# parent receiver so signatures group consistently.
_GO_WRAPPER_PATTERNS = (
    (re.compile(r"^(gin\.HandlerFunc)\.func\d+"), r"\1"),
    (re.compile(r"^(gin\.\(\*Engine\))\.handleHTTPRequest"), r"\1.handleHTTPRequest"),
    (re.compile(r"^(echo\.\(\*Echo\))\.ServeHTTP"), r"\1.ServeHTTP"),
    (re.compile(r"^(chi\.\(\*Mux\))\.routeHTTP"), r"\1.routeHTTP"),
    (re.compile(r"^(fiber\.\(\*App\))\.Handler"), r"\1.Handler"),
    # Anonymous closure suffix `.func1.func2` → strip trailing `.func\d+`
    # chains to keep the receiver visible.
    (re.compile(r"(.+?)(?:\.func\d+)+$"), r"\1"),
)


def _normalize_go_frame(frame: StackFrame) -> StackFrame:
    if frame.language != "go":
        return frame
    qualified = (
        f"{frame.module}.{frame.function}" if frame.module else frame.function
    )
    new_qualified = qualified
    for pattern, replacement in _GO_WRAPPER_PATTERNS:
        new_qualified = pattern.sub(replacement, new_qualified)
    if new_qualified == qualified:
        return frame
    if "." in new_qualified:
        new_module, _, new_function = new_qualified.rpartition(".")
    else:
        new_module, new_function = None, new_qualified
    return replace(frame, function=new_function, module=new_module)


_GO_NETWORK_PATTERNS = re.compile(
    r"(?:netpollblock|runtime\.netpoll|net\.\(\*netFD\)\.Read|"
    r"net\.\(\*conn\)\.Read|net\.\(\*TCPConn\)\.Read|net\.\(\*UDPConn\)\.Read|"
    r"http\.\(\*persistConn\)\.readResponse)",
    re.IGNORECASE,
)
_GO_LOCK_PATTERNS = re.compile(
    r"(?:semacquire|sync\.\(\*Mutex\)\.Lock|sync\.\(\*RWMutex\)\.Lock|"
    r"sync\.runtime_Semacquire)",
    re.IGNORECASE,
)
_GO_CHANNEL_PATTERNS = re.compile(
    r"(?:gopark|chanrecv|chansend|runtime\.selectgo|runtime\.chansend|"
    r"runtime\.chanrecv)",
    re.IGNORECASE,
)
_GO_IO_PATTERNS = re.compile(
    r"(?:os\.\(\*File\)\.Read|bufio\.\(\*Reader\)\.Read|"
    r"io\.copyBuffer|io\.\(\*pipe\)\.read)",
    re.IGNORECASE,
)


def _infer_go_state(state: ThreadState, frames: list[StackFrame]) -> ThreadState:
    """Promote Go runtime states based on the top stack frame."""
    if not frames:
        return state
    top = frames[0]
    qualified = (
        f"{top.module}.{top.function}" if top.module else top.function
    )
    if _GO_NETWORK_PATTERNS.search(qualified):
        return ThreadState.NETWORK_WAIT
    if _GO_LOCK_PATTERNS.search(qualified):
        return ThreadState.LOCK_WAIT
    if _GO_CHANNEL_PATTERNS.search(qualified):
        return ThreadState.CHANNEL_WAIT
    if _GO_IO_PATTERNS.search(qualified):
        return ThreadState.IO_WAIT
    return state


# Auto-register so importing the subpackage picks up Go support.
DEFAULT_REGISTRY.register(GoGoroutineParserPlugin())
