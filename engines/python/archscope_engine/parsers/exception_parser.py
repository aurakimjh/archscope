from __future__ import annotations

from pathlib import Path

from archscope_engine.models.thread_dump import ExceptionRecord


def parse_exception_stack(path: Path) -> list[ExceptionRecord]:
    raise NotImplementedError("Exception parsing is planned for a later phase.")
