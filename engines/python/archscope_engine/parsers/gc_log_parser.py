from __future__ import annotations

import re
from datetime import datetime
from pathlib import Path
from typing import Generator, Iterable

from archscope_engine.common.debug_log import DebugLogCollector, infer_field_shapes
from archscope_engine.common.diagnostics import ParserDiagnostics
from archscope_engine.common.file_utils import (
    TextLineContext,
    iter_text_lines,
    iter_text_lines_with_context,
)
from archscope_engine.models.gc_event import GcEvent


# ─── JDK 9+ Unified GC Log ────────────────────────────────────────────────────

UNIFIED_GC_RE = re.compile(
    r"^\[(?P<timestamp>[^\]]+)\].*?GC\(\d+\)\s+"
    r"(?P<label>.*?)\s+"
    r"(?P<before>\d+(?:\.\d+)?)(?P<before_unit>[KMG])->"
    r"(?P<after>\d+(?:\.\d+)?)(?P<after_unit>[KMG])"
    r"(?:\((?P<committed>\d+(?:\.\d+)?)(?P<committed_unit>[KMG])\))?\s+"
    r"(?P<pause>\d+(?:\.\d+)?)ms"
)

# ─── JDK 8 G1GC Legacy ────────────────────────────────────────────────────────

# Matches: [optional datestamp: ] uptime: [GC pause/remark/cleanup ..., secs]
# Uses greedy .+ so nested brackets in remark are consumed before matching final ", secs]"
_G1_PAUSE_RE = re.compile(
    r"^(?:(?P<datestamp>\d{4}-\d{2}-\d{2}T[\d:.]+[+-]\d{4}):\s+)?"
    r"(?P<uptime>[\d.,]+):\s+"
    r"\[(?P<label>GC\s+(?:pause|remark|cleanup).+),\s+"
    r"(?P<pause>[\d.]+)\s+secs\]\s*$"
)

# Matches: [Eden: before(cap)->after(cap) Survivors: before->after Heap: before(cap)->after(cap)]
_G1_MEMORY_RE = re.compile(
    r"\[Eden:\s*([\d.]+)([BKMG])\([^)]+\)->([\d.]+)([BKMG])\([^)]+\)\s+"
    r"Survivors:\s*([\d.]+)([BKMG])->([\d.]+)([BKMG])\s+"
    r"Heap:\s*([\d.]+)([BKMG])\(([\d.]+)([BKMG])\)->([\d.]+)([BKMG])\(([\d.]+)([BKMG])\)"
)

# Matches heap sizes embedded in GC cleanup label: "GC cleanup 75M->25M(103M)"
_G1_CLEANUP_MEM_RE = re.compile(
    r"GC\s+cleanup\s+([\d.]+)([BKMG])->([\d.]+)([BKMG])\(([\d.]+)([BKMG])\)"
)

_G1_PHASES = frozenset({"young", "mixed", "partial"})

# ─── JDK 4–8 Serial/Parallel/CMS ─────────────────────────────────────────────

_LEGACY_PAUSE_RE = re.compile(
    r"^(?:(?P<datestamp>\d{4}-\d{2}-\d{2}T[\d:.]+[+-]\d{4}):\s+)?"
    r"(?P<uptime>[\d.,]+):\s+"
    r"\[(?P<label>(?:Full\s+)?GC(?:\s+\([^)]*\))?)"
)

# Matches the outermost heap transition at end of a legacy GC line
_LEGACY_HEAP_RE = re.compile(
    r"(?P<before>[\d.]+)(?P<bu>[KMG])->(?P<after>[\d.]+)(?P<au>[KMG])"
    r"\((?P<committed>[\d.]+)(?P<cu>[KMG])\),\s+(?P<pause>[\d.]+)\s+secs"
)

# ─── Timezone fix (Python 3.10 cannot parse +0900, needs +09:00) ──────────────

_TZ_FIX_RE = re.compile(r"([+-]\d{2})(\d{2})$")


# ═════════════════════════════════════════════════════════════════════════════
# Public API
# ═════════════════════════════════════════════════════════════════════════════

def detect_gc_log_format(path: Path) -> str:
    """Detect the GC log format by sampling the first 8 KB.

    Returns one of: 'unified', 'g1_legacy', 'legacy', 'unknown'.
    """
    try:
        with open(path, encoding="utf-8", errors="replace") as f:
            sample = f.read(8192)
    except OSError:
        return "unknown"

    if "][gc" in sample or ("] GC(" in sample and "[info]" in sample):
        return "unified"
    if (
        "garbage-first heap" in sample
        or "G1 Evacuation Pause" in sample
        or "GC pause (young)" in sample
        or "GC pause (mixed)" in sample
    ):
        return "g1_legacy"
    if (
        "PSYoungGen" in sample
        or "ParNew" in sample
        or "DefNew" in sample
        or "CMS-" in sample
        or "PSPermGen" in sample
    ):
        return "legacy"
    return "unknown"


def parse_gc_log(
    path: Path,
    *,
    diagnostics: ParserDiagnostics | None = None,
    debug_log: DebugLogCollector | None = None,
) -> list[GcEvent]:
    own_diagnostics = diagnostics or ParserDiagnostics()
    return list(
        iter_gc_log_events_with_diagnostics(
            path,
            diagnostics=own_diagnostics,
            debug_log=debug_log,
        )
    )


def iter_gc_log_events_with_diagnostics(
    path: Path,
    *,
    diagnostics: ParserDiagnostics,
    debug_log: DebugLogCollector | None = None,
) -> Iterable[GcEvent]:
    fmt = detect_gc_log_format(path)
    if fmt == "g1_legacy":
        yield from _iter_g1_legacy(path, diagnostics=diagnostics, debug_log=debug_log)
    elif fmt == "legacy":
        yield from _iter_legacy(path, diagnostics=diagnostics, debug_log=debug_log)
    else:
        yield from _iter_unified(path, diagnostics=diagnostics, debug_log=debug_log)


# ═════════════════════════════════════════════════════════════════════════════
# Unified (JDK 9+)
# ═════════════════════════════════════════════════════════════════════════════

def _iter_unified(
    path: Path,
    *,
    diagnostics: ParserDiagnostics,
    debug_log: DebugLogCollector | None,
) -> Iterable[GcEvent]:
    line_iterable = (
        iter_text_lines_with_context(path)
        if debug_log is not None
        else _line_contexts_without_neighbors(path)
    )
    for context in line_iterable:
        line_number = context.line_number
        line = context.target
        diagnostics.total_lines += 1
        if not line.strip():
            continue

        event = _parse_unified_gc_line(line)
        if event is None:
            diagnostics.add_skipped(
                line_number=line_number,
                reason="NO_GC_FORMAT_MATCH",
                message="Line did not match the supported HotSpot unified GC format.",
                raw_line=line,
            )
            if debug_log is not None:
                debug_log.add_parse_error(
                    line_number=line_number,
                    reason="NO_GC_FORMAT_MATCH",
                    message="Line did not match the supported HotSpot unified GC format.",
                    raw_context={
                        "before": context.before,
                        "target": line,
                        "after": context.after,
                    },
                    failed_pattern="HOTSPOT_UNIFIED_GC_PAUSE",
                    field_shapes=infer_field_shapes(line),
                )
            continue

        yield event
        diagnostics.parsed_records += 1


def _parse_unified_gc_line(line: str) -> GcEvent | None:
    match = UNIFIED_GC_RE.search(line)
    if match is None:
        return None

    label = match.group("label").strip()
    gc_type, cause = _split_unified_label(label)

    return GcEvent(
        timestamp=_parse_timestamp(match.group("timestamp")),
        uptime_sec=None,
        gc_type=gc_type,
        cause=cause,
        pause_ms=float(match.group("pause")),
        heap_before_mb=_to_mb(match.group("before"), match.group("before_unit")),
        heap_after_mb=_to_mb(match.group("after"), match.group("after_unit")),
        heap_committed_mb=(
            _to_mb(match.group("committed"), match.group("committed_unit"))
            if match.group("committed")
            else None
        ),
        young_before_mb=None,
        young_after_mb=None,
        old_before_mb=None,
        old_after_mb=None,
        metaspace_before_mb=None,
        metaspace_after_mb=None,
        raw_line=line,
    )


# ═════════════════════════════════════════════════════════════════════════════
# G1 Legacy (JDK 8 -XX:+UseG1GC)
# ═════════════════════════════════════════════════════════════════════════════

def _iter_g1_legacy(
    path: Path,
    *,
    diagnostics: ParserDiagnostics,
    debug_log: DebugLogCollector | None,
) -> Generator[GcEvent, None, None]:
    """State-machine parser for JDK 8 G1GC legacy multi-line log format.

    Strategy:
    - Buffer the current GC event when its pause line is seen.
    - Update the buffer with heap metrics when the Eden/Heap detail line is seen.
    - Flush the buffer (yield) when the *next* pause line is encountered or at EOF.
    - All other lines (heap blocks, worker details, headers) are silently skipped.
    """
    pending: dict | None = None

    for line_number, line in enumerate(iter_text_lines(path), start=1):
        diagnostics.total_lines += 1
        if not line.strip():
            continue

        # Eden/Survivors/Heap memory detail — check BEFORE skip-prefix guard
        # because this line starts with "   [Eden:..." (indented bracket).
        m = _G1_MEMORY_RE.search(line)
        if m and pending is not None:
            g = m.groups()
            # g indices: 0-1=Eden before, 2-3=Eden after, 4-5=Survivors before,
            #            6-7=Survivors after, 8-9=Heap before, 10-11=Heap cap before,
            #            12-13=Heap after, 14-15=Heap cap after
            pending["heap_before_mb"] = _to_mb(g[8], g[9])
            pending["heap_after_mb"] = _to_mb(g[12], g[13])
            pending["heap_committed_mb"] = _to_mb(g[14], g[15])
            pending["young_before_mb"] = _safe_add(_to_mb(g[0], g[1]), _to_mb(g[4], g[5]))
            pending["young_after_mb"] = _safe_add(_to_mb(g[2], g[3]), _to_mb(g[6], g[7]))
            continue

        # GC pause / remark / cleanup event line
        m = _G1_PAUSE_RE.match(line)
        if m:
            if pending is not None:
                yield _build_g1_event(pending)
                diagnostics.parsed_records += 1
            pending = {
                "datestamp": m.group("datestamp"),
                "uptime": m.group("uptime"),
                "label": m.group("label"),
                "pause_ms": float(m.group("pause")) * 1000.0,
                "raw_line": line,
            }
            # GC cleanup embeds heap sizes in its label: "GC cleanup 75M->25M(103M)"
            cm = _G1_CLEANUP_MEM_RE.search(line)
            if cm:
                pending["heap_before_mb"] = _to_mb(cm.group(1), cm.group(2))
                pending["heap_after_mb"] = _to_mb(cm.group(3), cm.group(4))
                pending["heap_committed_mb"] = _to_mb(cm.group(5), cm.group(6))
            continue

        # All other lines (heap block headers, worker detail lines, JVM headers): skip.

    if pending is not None:
        yield _build_g1_event(pending)
        diagnostics.parsed_records += 1


def _build_g1_event(data: dict) -> GcEvent:
    gc_type, cause = _parse_g1_label(data["label"])
    uptime_raw = data.get("uptime")
    uptime = float(uptime_raw.replace(",", ".")) if uptime_raw else None
    return GcEvent(
        timestamp=_parse_legacy_timestamp(data.get("datestamp")),
        uptime_sec=uptime,
        gc_type=gc_type,
        cause=cause,
        pause_ms=data.get("pause_ms"),
        heap_before_mb=data.get("heap_before_mb"),
        heap_after_mb=data.get("heap_after_mb"),
        heap_committed_mb=data.get("heap_committed_mb"),
        young_before_mb=data.get("young_before_mb"),
        young_after_mb=data.get("young_after_mb"),
        old_before_mb=None,
        old_after_mb=None,
        metaspace_before_mb=None,
        metaspace_after_mb=None,
        raw_line=data["raw_line"],
    )


def _parse_g1_label(label: str) -> tuple[str, str | None]:
    """Parse G1 legacy GC label into (gc_type, cause)."""
    label = label.strip()
    if label.startswith("GC remark"):
        return "G1 Remark", None
    if label.startswith("GC cleanup"):
        return "G1 Cleanup", None
    if not label.startswith("GC pause"):
        return label, None

    groups = re.findall(r"\(([^)]+)\)", label)
    if not groups:
        return "G1 Young", None

    cause: str | None = None
    phase = "young"
    modifiers: list[str] = []

    for g in groups:
        g_lower = g.lower()
        if g_lower in _G1_PHASES:
            phase = g_lower
        elif g_lower in ("initial-mark", "to-space exhausted", "to-space overflow"):
            modifiers.append(g)
        else:
            cause = g  # e.g. "G1 Evacuation Pause", "GC Locker"

    gc_type = f"G1 {phase.title()}"
    if modifiers:
        gc_type += f" ({', '.join(modifiers)})"
    return gc_type, cause


# ═════════════════════════════════════════════════════════════════════════════
# Legacy Non-G1 (JDK 4–8 Serial/Parallel/CMS)
# ═════════════════════════════════════════════════════════════════════════════

def _iter_legacy(
    path: Path,
    *,
    diagnostics: ParserDiagnostics,
    debug_log: DebugLogCollector | None,
) -> Generator[GcEvent, None, None]:
    """Single-line parser for JDK 4–8 Serial/Parallel/CMS GC log format."""
    for line_number, line in enumerate(iter_text_lines(path), start=1):
        diagnostics.total_lines += 1
        if not line.strip():
            continue

        pm = _LEGACY_PAUSE_RE.match(line)
        if pm is None:
            continue

        hm = _LEGACY_HEAP_RE.search(line)
        if hm is None:
            continue

        label = pm.group("label").strip()
        uptime_raw = pm.group("uptime")
        uptime = float(uptime_raw.replace(",", ".")) if uptime_raw else None

        is_full = "Full" in label
        cause_match = re.search(r"\(([^)]+)\)", label)
        cause = cause_match.group(1) if cause_match else None
        gc_type = "Full GC" if is_full else "Young GC"

        yield GcEvent(
            timestamp=_parse_legacy_timestamp(pm.group("datestamp")),
            uptime_sec=uptime,
            gc_type=gc_type,
            cause=cause,
            pause_ms=float(hm.group("pause")) * 1000.0,
            heap_before_mb=_to_mb(hm.group("before"), hm.group("bu")),
            heap_after_mb=_to_mb(hm.group("after"), hm.group("au")),
            heap_committed_mb=_to_mb(hm.group("committed"), hm.group("cu")),
            young_before_mb=None,
            young_after_mb=None,
            old_before_mb=None,
            old_after_mb=None,
            metaspace_before_mb=None,
            metaspace_after_mb=None,
            raw_line=line,
        )
        diagnostics.parsed_records += 1


# ═════════════════════════════════════════════════════════════════════════════
# Shared utilities
# ═════════════════════════════════════════════════════════════════════════════

def _split_unified_label(label: str) -> tuple[str, str | None]:
    gc_type = label.split(" (", 1)[0].strip()
    start = label.rfind(" (")
    if start == -1 or not label.endswith(")"):
        return gc_type or label, None
    return gc_type or label, label[start + 2 : -1]


def _parse_timestamp(value: str) -> datetime | None:
    try:
        return datetime.fromisoformat(value)
    except ValueError:
        return None


def _parse_legacy_timestamp(value: str | None) -> datetime | None:
    if not value:
        return None
    fixed = _TZ_FIX_RE.sub(r"\1:\2", value)
    try:
        return datetime.fromisoformat(fixed)
    except ValueError:
        return None


def _to_mb(value: str | None, unit: str | None) -> float | None:
    if value is None or unit is None:
        return None
    numeric = float(str(value).replace(",", "."))
    u = unit.upper()
    if u == "B":
        return round(numeric / (1024.0 * 1024.0), 3)
    if u == "K":
        return round(numeric / 1024.0, 3)
    if u == "G":
        return round(numeric * 1024.0, 3)
    return round(numeric, 3)  # M


def _safe_add(a: float | None, b: float | None) -> float | None:
    if a is None and b is None:
        return None
    return round((a or 0.0) + (b or 0.0), 3)


def _line_contexts_without_neighbors(path: Path):
    for line_number, line in enumerate(iter_text_lines(path), start=1):
        yield TextLineContext(
            line_number=line_number,
            before=None,
            target=line,
            after=None,
        )
