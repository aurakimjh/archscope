"""Node.js diagnostic-report parser plugin (T-200) + enrichment (T-201).

``process.report.writeReport()`` (Node.js 12+) emits a JSON object like::

    {
      "header": { "event": "JavaScript API", ... },
      "javascriptStack": {
        "message": "...",
        "stack": [
          "    at handler (/app/server.js:42:5)",
          "    at Layer.handle [as handle_request] (...)",
          ...
        ]
      },
      "nativeStack": [ { "pc": "0x...", "symbol": "uv_run" }, ... ],
      "libuv": [ { "type": "tcp", "is_active": true, ... }, ... ]
    }

Node's main thread is single-threaded, but the report also exposes the
worker pool's native pthreads. We map JS frames into
``StackFrame(language="javascript")`` and native frames into
``language="native"`` so the JS-only enrichment plugin (T-201) can target
just JS frames.
"""
from __future__ import annotations

import json
import re
from pathlib import Path
from typing import Any

from archscope_engine.models.thread_snapshot import (
    StackFrame,
    ThreadDumpBundle,
    ThreadSnapshot,
    ThreadState,
)
from archscope_engine.parsers.thread_dump.registry import DEFAULT_REGISTRY


_DETECT_RE = re.compile(
    r'"header"\s*:\s*\{.*?"javascriptStack"\s*:', re.DOTALL
)
_JS_FRAME_RE = re.compile(
    r"\s*at\s+(?P<func>[^()]+?)\s+\((?P<file>[^)]+):(?P<line>\d+):(?P<col>\d+)\)\s*$"
)


class NodejsDiagnosticReportParserPlugin:
    """Parser for ``process.report.writeReport()`` JSON output."""

    format_id: str = "nodejs_diagnostic_report"
    language: str = "javascript"

    def can_parse(self, head: str) -> bool:
        # The 4 KB head almost always contains the opening brace + first
        # few keys; check both required markers via a single forgiving regex.
        return bool(_DETECT_RE.search(head))

    def parse(self, path: Path) -> ThreadDumpBundle:
        text = Path(path).read_text(encoding="utf-8", errors="replace")
        try:
            payload: dict[str, Any] = json.loads(text)
        except json.JSONDecodeError:
            # Treat as empty bundle so the registry caller still gets a
            # structured response and not an exception.
            return ThreadDumpBundle(
                snapshots=[],
                source_file=str(path),
                source_format=self.format_id,
                language=self.language,
                metadata={"parse_error": "INVALID_NODEJS_REPORT_JSON"},
            )

        snapshots: list[ThreadSnapshot] = []
        snapshots.append(self._js_main_snapshot(payload))
        native_snapshot = self._native_snapshot(payload)
        if native_snapshot is not None:
            snapshots.append(native_snapshot)

        metadata: dict[str, Any] = {}
        if isinstance(payload.get("libuv"), list):
            metadata["libuv_handles"] = payload["libuv"][:50]
        if isinstance(payload.get("header"), dict):
            metadata["header"] = {
                key: payload["header"].get(key)
                for key in ("event", "trigger", "filename", "nodejsVersion")
                if key in payload["header"]
            }

        return ThreadDumpBundle(
            snapshots=snapshots,
            source_file=str(path),
            source_format=self.format_id,
            language=self.language,
            metadata=metadata,
        )

    def _js_main_snapshot(self, payload: dict[str, Any]) -> ThreadSnapshot:
        js_section = payload.get("javascriptStack") or {}
        raw_frames = js_section.get("stack")
        frames: list[StackFrame] = []
        if isinstance(raw_frames, list):
            for raw in raw_frames:
                if not isinstance(raw, str):
                    continue
                frames.append(_parse_js_frame(raw))
        frames = [_normalize_js_frame(frame) for frame in frames]
        state = _infer_js_state(ThreadState.RUNNABLE, payload)
        return ThreadSnapshot(
            snapshot_id="javascript::0::main",
            thread_name="main",
            thread_id="main",
            state=state,
            category=state.value,
            stack_frames=frames,
            metadata={"javascript_message": js_section.get("message")},
            language="javascript",
            source_format=self.format_id,
        )

    def _native_snapshot(self, payload: dict[str, Any]) -> ThreadSnapshot | None:
        raw = payload.get("nativeStack")
        if not isinstance(raw, list) or not raw:
            return None
        frames: list[StackFrame] = []
        for entry in raw:
            if not isinstance(entry, dict):
                continue
            symbol = entry.get("symbol") or entry.get("name") or "<unknown>"
            frames.append(
                StackFrame(
                    function=str(symbol),
                    file=entry.get("file") if isinstance(entry.get("file"), str) else None,
                    line=int(entry["line"]) if isinstance(entry.get("line"), int) else None,
                    language="native",
                )
            )
        return ThreadSnapshot(
            snapshot_id="native::0::worker-pool",
            thread_name="native",
            thread_id="native",
            state=ThreadState.UNKNOWN,
            category=None,
            stack_frames=frames,
            language="native",
            source_format=self.format_id,
        )


def _parse_js_frame(text: str) -> StackFrame:
    text = text.strip()
    match = _JS_FRAME_RE.match(text)
    if not match:
        # Anonymous callbacks come back as `at /app/foo.js:1:2` — try a
        # location-only fallback.
        loc_match = re.match(r"\s*at\s+(?P<file>[^:]+):(?P<line>\d+):\d+", text)
        if loc_match:
            try:
                return StackFrame(
                    function="<anonymous>",
                    file=loc_match.group("file"),
                    line=int(loc_match.group("line")),
                    language="javascript",
                )
            except ValueError:
                pass
        return StackFrame(function=text, language="javascript")
    try:
        line_no: int | None = int(match.group("line"))
    except ValueError:
        line_no = None
    return StackFrame(
        function=match.group("func").strip(),
        file=match.group("file"),
        line=line_no,
        language="javascript",
    )


# ---------------------------------------------------------------------------
# T-201 — Node.js framework cleanup + state inference
# ---------------------------------------------------------------------------

_JS_BOILERPLATE_FUNCTIONS = re.compile(
    r"^(?:Layer\.handle(?:_request)?|Route\.dispatch|"
    r"dispatch|RouterExecutionContext\.create|"
    r"ExceptionsHandler\.\w+|RouterProxy\.\w+|"
    r"runHooks|fastify\.\w+|"
    r"Koa\.handleRequest|next|"
    r"Promise\.resolve\.then|"
    r"AsyncFunction\.\w+|"
    r"AsyncResource\.\w+)$"
)


def _normalize_js_frame(frame: StackFrame) -> StackFrame:
    """Drop the obvious framework wrappers on JS frames."""
    if frame.language != "javascript":
        return frame
    # Strip `[as ...]` aliases that Express adds to function names.
    if frame.function and "[as " in frame.function:
        frame_function = frame.function.split("[as ")[0].strip()
        frame = StackFrame(
            function=frame_function or frame.function,
            file=frame.file,
            line=frame.line,
            language="javascript",
        )
    return frame


def _infer_js_state(state: ThreadState, payload: dict[str, Any]) -> ThreadState:
    """Pick the JS thread state from the diagnostic-report context.

    The diagnostic report is captured *because* something interesting was
    happening on the main thread, so a plain ``RUNNABLE`` is rarely the
    full story:

    * If libuv has any active handle of type ``tcp``/``udp``/``pipe`` we
      treat the main thread as ``NETWORK_WAIT`` (the JS side is parked in
      ``uv__io_poll`` waiting for I/O).
    * If pending file/timer handles dominate, mark it ``IO_WAIT``.
    * Otherwise fall through to the input state.
    """
    libuv = payload.get("libuv")
    if not isinstance(libuv, list):
        return state
    network_active = 0
    file_active = 0
    for handle in libuv:
        if not isinstance(handle, dict):
            continue
        if not handle.get("is_active"):
            continue
        kind = (handle.get("type") or "").lower()
        if kind in {"tcp", "udp", "pipe"}:
            network_active += 1
        elif kind in {"timer", "fs_event", "fs_poll"}:
            file_active += 1
    if network_active > 0:
        return ThreadState.NETWORK_WAIT
    if file_active > 0:
        return ThreadState.IO_WAIT
    return state


DEFAULT_REGISTRY.register(NodejsDiagnosticReportParserPlugin())
