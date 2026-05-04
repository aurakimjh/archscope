"""Language-agnostic thread-dump record models (T-188).

The legacy ``ThreadDumpRecord`` (in :mod:`models.thread_dump`) stays the
canonical type for the Java single-dump path. The multi-language /
multi-dump pipeline added in Phase 5 (Java jstack, Go goroutine, Python
py-spy / faulthandler, Node.js diagnostic report, .NET dump) uses the
records defined here so the correlator does not need to know about any
particular runtime.

Three layers:

* :class:`StackFrame` — one stack frame, normalized across runtimes. The
  ``language`` field lets language-aware enrichment plugins (T-194/T-195
  for Java, etc.) opt in or out without inspecting frame text.
* :class:`ThreadSnapshot` — a single thread captured at a single moment.
* :class:`ThreadDumpBundle` — all snapshots from a single dump file,
  carrying provenance (dump index, captured timestamp, originating file,
  detected source format).
"""
from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime
from enum import Enum
from typing import Any


class ThreadState(str, Enum):
    """Normalized thread states across runtimes.

    Subclasses ``str`` so the enum values serialize naturally to JSON
    (``"RUNNABLE"``) and can be compared against parser strings without an
    explicit ``.value``.
    """

    RUNNABLE = "RUNNABLE"
    BLOCKED = "BLOCKED"
    WAITING = "WAITING"
    TIMED_WAITING = "TIMED_WAITING"
    NETWORK_WAIT = "NETWORK_WAIT"
    IO_WAIT = "IO_WAIT"
    LOCK_WAIT = "LOCK_WAIT"
    CHANNEL_WAIT = "CHANNEL_WAIT"
    DEAD = "DEAD"
    NEW = "NEW"
    UNKNOWN = "UNKNOWN"

    @classmethod
    def coerce(cls, value: str | None) -> "ThreadState":
        """Best-effort mapping of a raw runtime state string to the enum."""
        if value is None:
            return cls.UNKNOWN
        upper = value.strip().upper().replace("-", "_").replace(" ", "_")
        # Common aliases observed in the wild.
        aliases = {
            "RUNNING": cls.RUNNABLE,
            "RUNNABLE": cls.RUNNABLE,
            "ACTIVE": cls.RUNNABLE,
            "BLOCKED": cls.BLOCKED,
            "WAIT": cls.WAITING,
            "WAITING": cls.WAITING,
            "TIMED_WAITING": cls.TIMED_WAITING,
            "PARKED": cls.WAITING,
            "TERMINATED": cls.DEAD,
            "DEAD": cls.DEAD,
            "NEW": cls.NEW,
            "INITIALIZING": cls.NEW,
            "SLEEP": cls.TIMED_WAITING,
            "SLEEPING": cls.TIMED_WAITING,
            "IO_WAIT": cls.IO_WAIT,
            "IO": cls.IO_WAIT,
            "NETWORK_WAIT": cls.NETWORK_WAIT,
            "LOCK_WAIT": cls.LOCK_WAIT,
            "CHANNEL_WAIT": cls.CHANNEL_WAIT,
            "CHAN_RECEIVE": cls.CHANNEL_WAIT,
            "CHAN_SEND": cls.CHANNEL_WAIT,
            "SELECT": cls.CHANNEL_WAIT,
        }
        if upper in aliases:
            return aliases[upper]
        try:
            return cls(upper)
        except ValueError:
            return cls.UNKNOWN


@dataclass(frozen=True)
class StackFrame:
    """A single normalized stack frame.

    ``function`` is the only required field; the rest are best-effort
    captures of whatever the source format provided so language-specific
    enrichment plugins can decide how much detail is available.
    """

    function: str
    module: str | None = None
    file: str | None = None
    line: int | None = None
    language: str | None = None

    def to_dict(self) -> dict[str, Any]:
        return {
            "function": self.function,
            "module": self.module,
            "file": self.file,
            "line": self.line,
            "language": self.language,
        }

    def render(self) -> str:
        """Human-readable single-line rendering used for stack signatures."""
        parts: list[str] = []
        if self.module:
            parts.append(f"{self.module}.{self.function}")
        else:
            parts.append(self.function)
        if self.file:
            location = self.file
            if self.line is not None:
                location = f"{self.file}:{self.line}"
            parts.append(f"({location})")
        return " ".join(parts)


@dataclass
class ThreadSnapshot:
    """One thread captured in one dump."""

    snapshot_id: str
    thread_name: str
    thread_id: str | None
    state: ThreadState
    category: str | None
    stack_frames: list[StackFrame] = field(default_factory=list)
    lock_info: str | None = None
    metadata: dict[str, Any] = field(default_factory=dict)
    language: str | None = None
    source_format: str | None = None

    def stack_signature(self, depth: int = 5) -> str:
        """Compact representation of the top ``depth`` frames.

        Matches the convention used by the legacy single-dump analyzer
        (`" | "` separator) so existing dashboards keep grouping the same
        threads even when they switch to the multi-dump pipeline.
        """
        if not self.stack_frames:
            return "(no-stack)"
        return " | ".join(frame.render() for frame in self.stack_frames[:depth])

    def to_dict(self) -> dict[str, Any]:
        return {
            "snapshot_id": self.snapshot_id,
            "thread_name": self.thread_name,
            "thread_id": self.thread_id,
            "state": self.state.value,
            "category": self.category,
            "stack_frames": [frame.to_dict() for frame in self.stack_frames],
            "lock_info": self.lock_info,
            "metadata": dict(self.metadata),
            "language": self.language,
            "source_format": self.source_format,
        }


@dataclass
class ThreadDumpBundle:
    """All snapshots emitted from a single dump file."""

    snapshots: list[ThreadSnapshot]
    source_file: str
    source_format: str
    language: str
    dump_index: int = 0
    dump_label: str | None = None
    captured_at: datetime | None = None
    metadata: dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict[str, Any]:
        return {
            "source_file": self.source_file,
            "source_format": self.source_format,
            "language": self.language,
            "dump_index": self.dump_index,
            "dump_label": self.dump_label,
            "captured_at": self.captured_at.isoformat() if self.captured_at else None,
            "metadata": dict(self.metadata),
            "snapshots": [snapshot.to_dict() for snapshot in self.snapshots],
        }
