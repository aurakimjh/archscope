from __future__ import annotations

from pathlib import Path

from archscope_engine.models.gc_event import GcEvent


def parse_gc_log(path: Path) -> list[GcEvent]:
    raise NotImplementedError("GC log parsing is planned for a later phase.")
