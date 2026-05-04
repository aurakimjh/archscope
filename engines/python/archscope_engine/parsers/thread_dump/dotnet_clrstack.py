""".NET ``dotnet-dump analyze`` clrstack parser plugin (T-202).

The clrstack output looks like::

    OS Thread Id: 0x1a4 (1)
            Child SP               IP Call Site
    000000A1B2C3D4E0 00007FF812345678 System.Threading.Monitor.Enter(System.Object)
    000000A1B2C3D4F8 00007FF812345abc MyApp.Service.Process(MyApp.Request)
    000000A1B2C3D510 00007FF812345def MyApp.Program.Main(System.String[])

    OS Thread Id: 0x1a5 (2)
            Child SP               IP Call Site
    ...

We capture each block as a :class:`ThreadSnapshot` with
``language="dotnet"``, normalize obvious async state-machine wrappers
(``MoveNext()``, lifted ``<…>g__…|…``) so signatures collapse, and
record the sync-block-table marker (when present) under ``metadata`` for
downstream lock analysis.
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


_OS_THREAD_HEADER_RE = re.compile(
    r"^OS Thread Id:\s+(?P<tid>0x[0-9a-fA-F]+)(?:\s+\((?P<managed>\d+)\))?\s*$"
)
_FRAME_HEADER_RE = re.compile(r"^\s*Child SP\s+IP\s+Call Site\s*$")
_FRAME_LINE_RE = re.compile(
    r"^(?P<sp>[0-9a-fA-F]+)\s+(?P<ip>[0-9a-fA-F]+)\s+(?P<call>.+)$"
)
_SYNC_BLOCK_HEADER_RE = re.compile(r"^Sync Block Owner Info:\s*$")


class DotnetClrstackParserPlugin:
    """Parser for ``dotnet-dump analyze`` clrstack-style listings."""

    format_id: str = "dotnet_clrstack"
    language: str = "dotnet"

    def can_parse(self, head: str) -> bool:
        return bool(re.search(r"^OS Thread Id:\s+0x", head, re.MULTILINE))

    def parse(self, path: Path) -> ThreadDumpBundle:
        text = Path(path).read_text(encoding="utf-8", errors="replace")
        snapshots: list[ThreadSnapshot] = []
        sync_block_evidence: list[str] = []

        # Split into thread blocks by `OS Thread Id:` headers.
        lines = text.splitlines()
        index = 0
        thread_index = 0
        while index < len(lines):
            line = lines[index].rstrip("\r")
            sync_match = _SYNC_BLOCK_HEADER_RE.match(line)
            if sync_match:
                # Capture the rest of the sync-block table as evidence —
                # we don't deeply parse it but expose it under metadata so
                # the multi-dump correlator can correlate lock owners.
                buffer: list[str] = []
                index += 1
                while index < len(lines):
                    sub = lines[index].rstrip("\r")
                    if _OS_THREAD_HEADER_RE.match(sub) or sub.strip() == "":
                        break
                    buffer.append(sub)
                    index += 1
                if buffer:
                    sync_block_evidence.extend(buffer)
                continue
            header_match = _OS_THREAD_HEADER_RE.match(line)
            if not header_match:
                index += 1
                continue

            tid = header_match.group("tid")
            managed_id = header_match.group("managed")
            index += 1
            # Optional `Child SP IP Call Site` header line — skip if present.
            if index < len(lines) and _FRAME_HEADER_RE.match(
                lines[index].rstrip("\r")
            ):
                index += 1
            frames: list[StackFrame] = []
            while index < len(lines):
                sub = lines[index].rstrip("\r")
                if _OS_THREAD_HEADER_RE.match(sub):
                    break
                if _SYNC_BLOCK_HEADER_RE.match(sub):
                    break
                frame_match = _FRAME_LINE_RE.match(sub.strip())
                if frame_match:
                    frames.append(_parse_call_site(frame_match.group("call").strip()))
                index += 1

            frames = [_normalize_dotnet_frame(frame) for frame in frames]
            state = _infer_dotnet_state(ThreadState.RUNNABLE, frames)
            snapshots.append(
                ThreadSnapshot(
                    snapshot_id=f"dotnet::{thread_index}::{tid}",
                    thread_name=managed_id or tid,
                    thread_id=tid,
                    state=state,
                    category=state.value,
                    stack_frames=frames,
                    metadata={
                        "managed_id": managed_id,
                    },
                    language="dotnet",
                    source_format=self.format_id,
                )
            )
            thread_index += 1

        bundle_metadata: dict[str, object] = {}
        if sync_block_evidence:
            bundle_metadata["sync_block_owner_info"] = sync_block_evidence

        return ThreadDumpBundle(
            snapshots=snapshots,
            source_file=str(path),
            source_format=self.format_id,
            language=self.language,
            metadata=bundle_metadata,
        )


def _parse_call_site(call: str) -> StackFrame:
    # `MyApp.Service.Process(MyApp.Request)` → module=MyApp.Service, function=Process
    open_paren = call.find("(")
    qualified = call[:open_paren] if open_paren > 0 else call
    if "." in qualified:
        module, _, function = qualified.rpartition(".")
    else:
        module, function = None, qualified
    return StackFrame(
        function=function or call,
        module=module,
        language="dotnet",
    )


# ---------------------------------------------------------------------------
# Light cleanup of async state-machine synthetic names (T-202).
# ---------------------------------------------------------------------------

# C# async state machine: `<MethodName>d__7.MoveNext` → `MethodName.MoveNext`.
_ASYNC_STATE_MACHINE_RE = re.compile(r"<(?P<name>[^>]+)>d__\d+(?P<rest>.*)")
# Local-function synthesis: `<Foo>g__Bar|3_0` → `Foo.Bar`.
_LOCAL_FUNCTION_RE = re.compile(r"<(?P<outer>[^>]+)>g__(?P<inner>[A-Za-z_]\w*)\|\d+_\d+")


def _normalize_dotnet_frame(frame: StackFrame) -> StackFrame:
    if frame.language != "dotnet":
        return frame
    new_function = frame.function
    new_module = frame.module

    if new_module:
        local_match = _LOCAL_FUNCTION_RE.search(new_module)
        if local_match:
            new_module = new_module.replace(
                local_match.group(0),
                f"{local_match.group('outer')}.{local_match.group('inner')}",
            )
        async_match = _ASYNC_STATE_MACHINE_RE.search(new_module)
        if async_match:
            new_module = new_module.replace(
                async_match.group(0),
                f"{async_match.group('name')}{async_match.group('rest')}",
            )

    if new_function == frame.function and new_module == frame.module:
        return frame
    return replace(frame, function=new_function, module=new_module)


_DOTNET_NETWORK_FUNCTIONS = re.compile(
    r"\b(?:Socket\.Receive|Socket\.Send|Socket\.Accept|Socket\.Connect|"
    r"NetworkStream\.Read|NetworkStream\.Write|"
    r"HttpClient\.Send|HttpRequestMessage\.SendAsync)",
    re.IGNORECASE,
)
_DOTNET_LOCK_FUNCTIONS = re.compile(
    r"\b(?:Monitor\.Enter|Monitor\.Wait|SpinLock\.Enter|"
    r"SemaphoreSlim\.WaitAsync|ReaderWriterLockSlim\.Enter)",
    re.IGNORECASE,
)
_DOTNET_IO_FUNCTIONS = re.compile(
    r"\b(?:File\.Read|FileStream\.Read|StreamReader\.Read|"
    r"BufferedStream\.Read)",
    re.IGNORECASE,
)


def _infer_dotnet_state(state: ThreadState, frames: list[StackFrame]) -> ThreadState:
    if not frames:
        return state
    top = frames[0]
    qualified = (
        f"{top.module}.{top.function}" if top.module else top.function
    )
    if _DOTNET_NETWORK_FUNCTIONS.search(qualified):
        return ThreadState.NETWORK_WAIT
    if _DOTNET_LOCK_FUNCTIONS.search(qualified):
        return ThreadState.LOCK_WAIT
    if _DOTNET_IO_FUNCTIONS.search(qualified):
        return ThreadState.IO_WAIT
    return state


DEFAULT_REGISTRY.register(DotnetClrstackParserPlugin())
