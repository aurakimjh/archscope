"""Extract JVM / system metadata from the header of a GC log.

Real-world GC logs typically open with several lines describing the JVM
build, the host hardware, and the effective JVM flags. They are easy to
overlook and yet decisive for diagnosing performance issues — e.g. a
9-core box that ends up with ``Parallel Workers: 1`` because someone
pinned ``-XX:ParallelGCThreads=1``.

Two styles are recognised:

1. JDK 9+ unified format: each datum on a ``[gc,init]`` (or ``[gc,metaspace]``)
   tagged line, e.g. ``[0.002s][info][gc,init] CPUs: 8 total, 8 available``.
2. JDK 4-8 Serial/Parallel/CMS/G1 legacy: free-form lines at the top of
   the file (``Java HotSpot(TM) ...``, ``Memory: ...``, ``CommandLine flags:
   ...``).

This module is **purely** a scanner; it does not interpret or judge the
configuration. The frontend is responsible for highlighting suspicious
combinations.
"""
from __future__ import annotations

import re
from pathlib import Path
from typing import Any

from archscope_engine.common.file_utils import iter_text_lines


# ── Unified format ───────────────────────────────────────────────────────────

# [0.002s][info][gc,init] PAYLOAD   or   [0.002s][info][gc,metaspace] PAYLOAD
_UNIFIED_INIT_LINE = re.compile(
    r"\]\s*\[\s*info\s*\]\s*\[\s*gc,(?:init|metaspace)\s*\]\s*(?P<payload>.+?)\s*$"
)

# An event line (anything that is not [gc,init|metaspace]). Used to stop
# scanning once header context ends.
_UNIFIED_EVENT_LINE = re.compile(
    r"\]\s*\[\s*info\s*\]\s*\[\s*gc(?:[,\s][^\]]*)?\]\s+GC\(\d+\)"
)


# ── JDK 8 / legacy format ────────────────────────────────────────────────────

_JDK8_VM_LINE = re.compile(r"^Java\s+HotSpot|^OpenJDK")
_JDK8_MEMORY_LINE = re.compile(r"^Memory:\s+(?P<rest>.+)$")
_JDK8_FLAGS_LINE = re.compile(r"^CommandLine flags:\s*(?P<flags>.+)$")
# JDK 8 G1 logs include an early header like "garbage-first heap   total ..."
# that signals the start of the first event block — useful as stop marker.
_JDK8_GC_BLOCK_START = re.compile(r"^\s*\{?Heap before GC|^\s*\d{4}-\d{2}-\d{2}T")


# ── Scan limits ──────────────────────────────────────────────────────────────

# How many lines to scan before giving up. Real headers are <50 lines; we use
# a generous upper bound to tolerate verbose -XX:+PrintFlagsFinal style logs.
_MAX_HEADER_LINES = 400


def extract_jvm_header(path: Path) -> dict[str, Any]:
    """Return JVM/system metadata extracted from the start of *path*.

    Missing fields are simply omitted. Always includes a ``raw_lines`` list
    so the UI can render the verbatim header for capture/copy.
    """
    info: dict[str, Any] = {"raw_lines": []}

    try:
        for line_number, line in enumerate(iter_text_lines(path), start=1):
            if line_number > _MAX_HEADER_LINES:
                break
            stripped = line.strip()
            if not stripped:
                continue

            # ── Stop conditions ──────────────────────────────────────────────
            if _UNIFIED_EVENT_LINE.search(stripped):
                # Reached the first GC event in unified mode → header ends.
                break
            if _JDK8_GC_BLOCK_START.match(stripped):
                # Reached the first GC block / dated event → header ends.
                break

            # ── Unified [gc,init] / [gc,metaspace] payloads ──────────────────
            m = _UNIFIED_INIT_LINE.search(stripped)
            if m:
                _ingest_unified_payload(info, m.group("payload").strip())
                info["raw_lines"].append(stripped)
                continue

            # ── JDK 8 free-form header lines ─────────────────────────────────
            if _JDK8_VM_LINE.match(stripped):
                info.setdefault("vm_banner", stripped)
                _extract_jdk8_version(info, stripped)
                info["raw_lines"].append(stripped)
                continue

            mm = _JDK8_MEMORY_LINE.match(stripped)
            if mm:
                _ingest_jdk8_memory(info, mm.group("rest"))
                info["raw_lines"].append(stripped)
                continue

            mf = _JDK8_FLAGS_LINE.match(stripped)
            if mf:
                info["command_line"] = mf.group("flags").strip()
                _infer_collector_from_flags(info, info["command_line"])
                info["raw_lines"].append(stripped)
                continue
    except OSError:
        return info

    # Strip empty raw_lines list so callers can detect "no header found".
    if not info["raw_lines"]:
        info.pop("raw_lines", None)
    return info


# ── Unified helpers ──────────────────────────────────────────────────────────


def _ingest_unified_payload(info: dict[str, Any], payload: str) -> None:
    # Match a few well-known forms in priority order.
    # "Using G1" → collector
    if payload.startswith("Using "):
        info.setdefault("collector", payload[len("Using ") :].strip())
        return

    # "Version: 17.0.7+8 (release)"
    m = re.match(r"Version:\s+(?P<version>.+)$", payload)
    if m:
        info["vm_version"] = m.group("version").strip()
        return

    # "CPUs: 8 total, 8 available"
    m = re.match(r"CPUs:\s+(?P<total>\d+)\s+total(?:,\s+(?P<avail>\d+)\s+available)?", payload)
    if m:
        info["cpus_total"] = int(m.group("total"))
        if m.group("avail"):
            info["cpus_available"] = int(m.group("avail"))
        return

    # "Memory: 16384M"
    m = re.match(r"Memory:\s+(?P<size>[\d.]+)(?P<unit>[KMG])\b", payload)
    if m:
        info["memory_mb"] = _to_mb(m.group("size"), m.group("unit"))
        return

    # "Heap Region Size: 4M" / "Heap Min Capacity: 8M" / "Heap Initial Capacity: 256M" / "Heap Max Capacity: 4G"
    for key, label in (
        ("heap_region_size_mb", "Heap Region Size"),
        ("heap_min_mb", "Heap Min Capacity"),
        ("heap_initial_mb", "Heap Initial Capacity"),
        ("heap_max_mb", "Heap Max Capacity"),
    ):
        m = re.match(rf"{re.escape(label)}:\s+(?P<size>[\d.]+)(?P<unit>[KMG])\b", payload)
        if m:
            info[key] = _to_mb(m.group("size"), m.group("unit"))
            return

    # Worker counts
    for key, label in (
        ("parallel_workers", "Parallel Workers"),
        ("concurrent_workers", "Concurrent Workers"),
        ("concurrent_refinement_workers", "Concurrent Refinement Workers"),
    ):
        m = re.match(rf"{re.escape(label)}:\s+(?P<n>\d+)", payload)
        if m:
            info[key] = int(m.group("n"))
            return

    # Boolean-ish flags surfaced as strings so we don't lose the
    # "Enabled (32-bit)" detail on Compressed Oops.
    for key, label in (
        ("large_pages", "Large Page Support"),
        ("numa", "NUMA Support"),
        ("compressed_oops", "Compressed Oops"),
        ("pre_touch", "Pre-touch"),
        ("periodic_gc", "Periodic GC"),
    ):
        m = re.match(rf"{re.escape(label)}:\s+(?P<value>.+)$", payload)
        if m:
            info[key] = m.group("value").strip()
            return

    # CommandLine flags (unified format also emits these on a [gc,init] line)
    m = re.match(r"CommandLine flags:\s+(?P<flags>.+)$", payload)
    if m:
        info["command_line"] = m.group("flags").strip()
        _infer_collector_from_flags(info, info["command_line"])
        return


# ── JDK 8 helpers ────────────────────────────────────────────────────────────


def _extract_jdk8_version(info: dict[str, Any], banner: str) -> None:
    # "Java HotSpot(TM) 64-Bit Server VM (25.181-b13) for solaris-sparc JRE (1.8.0_181-b13)"
    # We capture both the build (25.181-b13) and JRE label (1.8.0_181-b13).
    m = re.search(r"\(([\d.]+(?:[-_]b?\d+)?)\)\s+for\s+(?P<platform>[\w\-]+)", banner)
    if m:
        info["vm_build"] = m.group(1)
        info["platform"] = m.group("platform")
    m = re.search(r"JRE\s+\(([^)]+)\)", banner)
    if m:
        info["vm_version"] = m.group(1)


def _ingest_jdk8_memory(info: dict[str, Any], rest: str) -> None:
    # "8k page, physical 132644864k(99543560k free)"
    m = re.search(r"physical\s+(\d+)k", rest)
    if m:
        # physical is in KB; convert to MB.
        info["memory_mb"] = int(m.group(1)) // 1024
    m = re.search(r"(\d+)k\s*page", rest)
    if m:
        info["page_size_kb"] = int(m.group(1))


def _infer_collector_from_flags(info: dict[str, Any], flags: str) -> None:
    if "collector" in info:
        return
    if "+UseG1GC" in flags:
        info["collector"] = "G1"
    elif "+UseShenandoahGC" in flags:
        info["collector"] = "Shenandoah"
    elif "+UseZGC" in flags:
        info["collector"] = "ZGC"
    elif "+UseConcMarkSweepGC" in flags:
        info["collector"] = "CMS"
    elif "+UseParallelGC" in flags or "+UseParallelOldGC" in flags:
        info["collector"] = "Parallel"
    elif "+UseSerialGC" in flags:
        info["collector"] = "Serial"


# ── Misc ─────────────────────────────────────────────────────────────────────


def _to_mb(size: str, unit: str) -> float:
    value = float(size)
    if unit == "K":
        return round(value / 1024, 2)
    if unit == "M":
        return round(value, 2)
    if unit == "G":
        return round(value * 1024, 2)
    return round(value, 2)
