from __future__ import annotations

from pathlib import Path

from archscope_engine.models.thread_dump import ThreadDumpRecord


def parse_thread_dump(path: Path) -> list[ThreadDumpRecord]:
    raise NotImplementedError("Thread dump parsing is planned for a later phase.")
