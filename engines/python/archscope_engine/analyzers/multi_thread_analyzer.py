"""Multi-dump correlation analyzer (T-191).

Consumes a list of :class:`ThreadDumpBundle` objects and produces an
``AnalysisResult`` of type ``"thread_dump_multi"``. Two language-agnostic
findings are emitted today:

* ``LONG_RUNNING_THREAD`` — same thread name with the same stack
  signature in ``≥3`` consecutive dumps while in a runnable state.
* ``PERSISTENT_BLOCKED_THREAD`` — same thread blocked (BLOCKED /
  LOCK_WAIT) in ``≥3`` consecutive dumps regardless of whether the stack
  changed.

The analyzer never imports Java-specific helpers; runtime-specific
enrichment is the parser plugin's job (Phase 5 follow-ups T-194/T-195
for Java, etc.).
"""
from __future__ import annotations

from collections import Counter, defaultdict
from dataclasses import dataclass, field
from typing import Any, Sequence

from archscope_engine.models.analysis_result import AnalysisResult
from archscope_engine.models.thread_snapshot import (
    ThreadDumpBundle,
    ThreadSnapshot,
    ThreadState,
)

CONSECUTIVE_DUMPS_THRESHOLD = 3

_RUNNABLE_STATES: frozenset[ThreadState] = frozenset({ThreadState.RUNNABLE})
_BLOCKED_STATES: frozenset[ThreadState] = frozenset(
    {ThreadState.BLOCKED, ThreadState.LOCK_WAIT}
)

# Wait categories surfaced by the per-language enrichment plugins
# (T-195/T-197/T-199/T-201). A thread that stays in any of these for
# `threshold` consecutive dumps gets a LATENCY_SECTION_DETECTED finding.
# `LOCK_WAIT` is intentionally absent here — the dedicated
# `PERSISTENT_BLOCKED_THREAD` finding (T-191) already covers it, and
# folding both findings into one would dilute the lock-contention
# signal.
_LATENCY_WAIT_CATEGORIES: tuple[ThreadState, ...] = (
    ThreadState.NETWORK_WAIT,
    ThreadState.IO_WAIT,
    ThreadState.CHANNEL_WAIT,
)


@dataclass(frozen=True)
class ThreadObservation:
    dump_index: int
    state: ThreadState
    stack_signature: str


@dataclass
class ThreadTimeline:
    thread_name: str
    observations: list[ThreadObservation] = field(default_factory=list)

    def stack_run_lengths(self) -> dict[str, int]:
        """Maximum run-length per stack signature in this thread."""
        max_per_sig: dict[str, int] = {}
        prev: tuple[int, str] | None = None  # (dump_index, signature)
        run_length = 0
        current_sig: str | None = None
        for obs in sorted(self.observations, key=lambda o: o.dump_index):
            if (
                prev is not None
                and obs.dump_index == prev[0] + 1
                and obs.stack_signature == current_sig
            ):
                run_length += 1
            else:
                run_length = 1
                current_sig = obs.stack_signature
            max_per_sig[current_sig] = max(
                max_per_sig.get(current_sig, 0), run_length
            )
            prev = (obs.dump_index, current_sig)
        return max_per_sig

    def state_run_lengths(self, target_states: frozenset[ThreadState]) -> int:
        """Longest consecutive-dump run where state ∈ ``target_states``."""
        longest = 0
        current = 0
        prev_index: int | None = None
        for obs in sorted(self.observations, key=lambda o: o.dump_index):
            if obs.state not in target_states:
                current = 0
                prev_index = obs.dump_index
                continue
            if prev_index is None or obs.dump_index == prev_index + 1:
                current += 1
            else:
                current = 1
            longest = max(longest, current)
            prev_index = obs.dump_index
        return longest


def analyze_multi_thread_dumps(
    bundles: Sequence[ThreadDumpBundle],
    *,
    threshold: int = CONSECUTIVE_DUMPS_THRESHOLD,
    top_n: int = 20,
) -> AnalysisResult:
    """Correlate threads across an ordered list of thread-dump bundles."""
    if not bundles:
        raise ValueError("analyze_multi_thread_dumps requires at least one bundle.")

    timelines = _build_timelines(bundles)
    languages = sorted({bundle.language for bundle in bundles})
    formats = sorted({bundle.source_format for bundle in bundles})

    long_running, persistent_blocked = _build_findings(timelines, threshold=threshold)
    latency_sections = _build_latency_sections(timelines, threshold=threshold)
    growing_locks = _build_growing_lock_findings(bundles, threshold=threshold)
    long_running.sort(key=lambda finding: finding["dumps"], reverse=True)
    persistent_blocked.sort(key=lambda finding: finding["dumps"], reverse=True)
    latency_sections.sort(key=lambda finding: finding["dumps"], reverse=True)
    growing_locks.sort(
        key=lambda finding: (
            -int(finding["max_waiters"]),
            -int(finding["consecutive_dumps"]),
        )
    )
    jvm_tables = _jvm_metadata_tables(bundles, top_n=top_n)
    jvm_findings = _jvm_metadata_findings(jvm_tables, top_n=top_n)
    heuristic_findings = _build_heuristic_findings(bundles)

    findings_payload = _findings_payload(
        long_running=long_running,
        persistent_blocked=persistent_blocked,
        latency_sections=latency_sections,
        growing_locks=growing_locks,
        jvm_metadata_findings=jvm_findings,
        heuristic_findings=heuristic_findings,
        threshold=threshold,
    )

    summary = {
        "total_dumps": len(bundles),
        "languages_detected": languages,
        "source_formats": formats,
        "unique_threads": len(timelines),
        "total_thread_observations": sum(
            len(timeline.observations) for timeline in timelines.values()
        ),
        "long_running_threads": len(long_running),
        "persistent_blocked_threads": len(persistent_blocked),
        "latency_sections": len(latency_sections),
        "growing_lock_contention": len(growing_locks),
        "virtual_thread_carrier_pinning": len(
            jvm_tables["virtual_thread_carrier_pinning"]
        ),
        "smr_unresolved_threads": len(jvm_tables["smr_unresolved_threads"]),
        "native_method_threads": len(jvm_tables["native_method_threads"]),
        "class_histogram_classes": len(jvm_tables["class_histogram_top_classes"]),
        "consecutive_dump_threshold": threshold,
    }
    series = {
        "thread_persistence": [
            {
                "thread_name": timeline.thread_name,
                "observed_in_dumps": len(timeline.observations),
            }
            for timeline in sorted(
                timelines.values(),
                key=lambda t: len(t.observations),
                reverse=True,
            )[:top_n]
        ],
        "state_distribution_per_dump": _state_distribution_per_dump(bundles),
        "state_transition_timeline": _state_transition_timeline(timelines, top_n=top_n),
    }
    tables = {
        "long_running_stacks": long_running[:top_n],
        "persistent_blocked_threads": persistent_blocked[:top_n],
        "latency_sections": latency_sections[:top_n],
        "growing_lock_contention": growing_locks[:top_n],
        "dumps": [
            {
                "dump_index": bundle.dump_index,
                "dump_label": bundle.dump_label,
                "source_file": bundle.source_file,
                "source_format": bundle.source_format,
                "language": bundle.language,
                "thread_count": len(bundle.snapshots),
                "start_line": bundle.metadata.get("start_line"),
                "end_line": bundle.metadata.get("end_line"),
                "raw_timestamp": bundle.metadata.get("raw_timestamp"),
            }
            for bundle in bundles
        ],
        **jvm_tables,
    }
    metadata = {
        "parser": "thread_dump_multi",
        "schema_version": "0.1.0",
        "diagnostics": {
            "total_lines": 0,
            "parsed_records": sum(len(b.snapshots) for b in bundles),
            "skipped_lines": 0,
            "skipped_by_reason": {},
            "samples": [],
        },
        "findings": findings_payload,
    }

    return AnalysisResult(
        type="thread_dump_multi",
        source_files=[bundle.source_file for bundle in bundles],
        summary=summary,
        series=series,
        tables=tables,
        metadata=metadata,
    )


def _build_timelines(bundles: Sequence[ThreadDumpBundle]) -> dict[str, ThreadTimeline]:
    timelines: dict[str, ThreadTimeline] = defaultdict(lambda: ThreadTimeline(""))
    for bundle in bundles:
        for snapshot in bundle.snapshots:
            timeline = timelines[snapshot.thread_name]
            timeline.thread_name = snapshot.thread_name
            timeline.observations.append(
                ThreadObservation(
                    dump_index=bundle.dump_index,
                    state=snapshot.state,
                    stack_signature=_signature_from_snapshot(snapshot),
                )
            )
    return dict(timelines)


def _signature_from_snapshot(snapshot: ThreadSnapshot) -> str:
    return snapshot.stack_signature()


def _build_findings(
    timelines: dict[str, ThreadTimeline],
    *,
    threshold: int,
) -> tuple[list[dict[str, Any]], list[dict[str, Any]]]:
    long_running: list[dict[str, Any]] = []
    persistent_blocked: list[dict[str, Any]] = []

    for timeline in timelines.values():
        # LONG_RUNNING: ≥threshold consecutive dumps with the same RUNNABLE stack.
        runnable_obs = [
            obs for obs in timeline.observations if obs.state in _RUNNABLE_STATES
        ]
        if runnable_obs:
            sig_run_lengths: dict[str, int] = {}
            prev_index: int | None = None
            current_sig: str | None = None
            current_run = 0
            for obs in sorted(runnable_obs, key=lambda o: o.dump_index):
                if (
                    prev_index is not None
                    and obs.dump_index == prev_index + 1
                    and obs.stack_signature == current_sig
                ):
                    current_run += 1
                else:
                    current_sig = obs.stack_signature
                    current_run = 1
                sig_run_lengths[current_sig] = max(
                    sig_run_lengths.get(current_sig, 0), current_run
                )
                prev_index = obs.dump_index

            for signature, run_length in sig_run_lengths.items():
                if run_length >= threshold:
                    long_running.append(
                        {
                            "thread_name": timeline.thread_name,
                            "stack_signature": signature,
                            "dumps": run_length,
                            "first_dump_index": min(
                                obs.dump_index
                                for obs in runnable_obs
                                if obs.stack_signature == signature
                            ),
                        }
                    )
                    break  # one finding per thread is enough

        # PERSISTENT_BLOCKED: ≥threshold consecutive dumps in BLOCKED/LOCK_WAIT.
        blocked_run = timeline.state_run_lengths(_BLOCKED_STATES)
        if blocked_run >= threshold:
            blocked_obs = [
                obs for obs in timeline.observations if obs.state in _BLOCKED_STATES
            ]
            persistent_blocked.append(
                {
                    "thread_name": timeline.thread_name,
                    "dumps": blocked_run,
                    "first_dump_index": min(obs.dump_index for obs in blocked_obs),
                    "stack_signatures": sorted(
                        {obs.stack_signature for obs in blocked_obs}
                    ),
                }
            )
    return long_running, persistent_blocked


# ---------------------------------------------------------------------------
# Heuristic findings (TDA-inspired ratio-based hot-spot detection)
# ---------------------------------------------------------------------------

# Tunables. Inspired by ``tda/parser/Analyzer.java`` thresholds, but
# expressed as ratios so they apply across runtimes.
_CONGESTION_WAITING_RATIO = 0.10
_EXTERNAL_RESOURCE_SLEEPING_RATIO = 0.25
_GC_PAUSE_UNOWNED_BLOCK_RATIO = 0.50


def _build_heuristic_findings(
    bundles: Sequence[ThreadDumpBundle],
) -> list[dict[str, Any]]:
    """Per-bundle ratio findings — congestion, external-resource wait,
    and likely-GC-pause. These are coarse signals to give a non-expert
    user an immediate "is this dump healthy?" indication.
    """
    findings: list[dict[str, Any]] = []
    for index, bundle in enumerate(bundles):
        snapshots = list(bundle.snapshots)
        total = len(snapshots)
        if total == 0:
            continue

        waiting = sum(
            1
            for s in snapshots
            if s.state in {ThreadState.WAITING, ThreadState.LOCK_WAIT, ThreadState.BLOCKED}
        )
        sleeping = sum(
            1 for s in snapshots if s.state in {ThreadState.TIMED_WAITING}
        )
        # "Threads blocked on a lock that no application thread is holding"
        # — the JVM/OS owns those monitors, classically a GC pause sign.
        held_locks: set[str] = set()
        for s in snapshots:
            for handle in s.lock_holds:
                if getattr(handle, "lock_id", None):
                    held_locks.add(handle.lock_id)
        unowned_blocked = 0
        for s in snapshots:
            if s.state not in {ThreadState.BLOCKED, ThreadState.LOCK_WAIT}:
                continue
            target = s.lock_waiting
            if target is None:
                continue
            target_id = getattr(target, "lock_id", None)
            if target_id and target_id not in held_locks:
                unowned_blocked += 1

        waiting_ratio = waiting / total
        sleeping_ratio = sleeping / total
        unowned_ratio = unowned_blocked / total

        if waiting_ratio > _CONGESTION_WAITING_RATIO:
            findings.append(
                {
                    "severity": "warning",
                    "code": "THREAD_CONGESTION_DETECTED",
                    "message": (
                        f"Dump #{index}: {waiting_ratio * 100:.1f}% of threads are waiting "
                        f"for a monitor — possible congestion or upstream deadlock."
                    ),
                    "evidence": {
                        "dump_index": index,
                        "source_file": bundle.source_file,
                        "waiting_threads": waiting,
                        "total_threads": total,
                        "waiting_ratio": round(waiting_ratio, 4),
                    },
                    "thresholds": {"waiting_ratio": _CONGESTION_WAITING_RATIO},
                }
            )
        if sleeping_ratio > _EXTERNAL_RESOURCE_SLEEPING_RATIO:
            findings.append(
                {
                    "severity": "info",
                    "code": "EXTERNAL_RESOURCE_WAIT_HIGH",
                    "message": (
                        f"Dump #{index}: {sleeping_ratio * 100:.1f}% of threads are sleeping "
                        f"on a monitor — likely waiting on an external resource (DB, network, "
                        f"or idle worker pool)."
                    ),
                    "evidence": {
                        "dump_index": index,
                        "source_file": bundle.source_file,
                        "sleeping_threads": sleeping,
                        "total_threads": total,
                        "sleeping_ratio": round(sleeping_ratio, 4),
                    },
                    "thresholds": {"sleeping_ratio": _EXTERNAL_RESOURCE_SLEEPING_RATIO},
                }
            )
        if unowned_ratio > _GC_PAUSE_UNOWNED_BLOCK_RATIO:
            findings.append(
                {
                    "severity": "warning",
                    "code": "LIKELY_GC_PAUSE_DETECTED",
                    "message": (
                        f"Dump #{index}: {unowned_ratio * 100:.1f}% of threads are blocked "
                        f"on monitors with no application owner — strongly suggests a GC pause."
                    ),
                    "evidence": {
                        "dump_index": index,
                        "source_file": bundle.source_file,
                        "unowned_blocked_threads": unowned_blocked,
                        "total_threads": total,
                        "unowned_block_ratio": round(unowned_ratio, 4),
                    },
                    "thresholds": {"unowned_block_ratio": _GC_PAUSE_UNOWNED_BLOCK_RATIO},
                }
            )
    return findings


def _findings_payload(
    *,
    long_running: list[dict[str, Any]],
    persistent_blocked: list[dict[str, Any]],
    latency_sections: list[dict[str, Any]],
    growing_locks: list[dict[str, Any]],
    jvm_metadata_findings: list[dict[str, Any]],
    heuristic_findings: list[dict[str, Any]] | None = None,
    threshold: int,
) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    if heuristic_findings:
        findings.extend(heuristic_findings)
    for entry in long_running:
        findings.append(
            {
                "severity": "warning",
                "code": "LONG_RUNNING_THREAD",
                "message": (
                    f"Thread {entry['thread_name']!r} stayed RUNNABLE on the same stack for "
                    f"{entry['dumps']} consecutive dumps."
                ),
                "evidence": entry,
                "thresholds": {"consecutive_dumps": threshold},
            }
        )
    for entry in persistent_blocked:
        findings.append(
            {
                "severity": "critical",
                "code": "PERSISTENT_BLOCKED_THREAD",
                "message": (
                    f"Thread {entry['thread_name']!r} stayed BLOCKED for "
                    f"{entry['dumps']} consecutive dumps."
                ),
                "evidence": entry,
                "thresholds": {"consecutive_dumps": threshold},
            }
        )
    for entry in latency_sections:
        findings.append(
            {
                "severity": "warning",
                "code": "LATENCY_SECTION_DETECTED",
                "message": (
                    f"Thread {entry['thread_name']!r} stayed in {entry['wait_category']} "
                    f"for {entry['dumps']} consecutive dumps."
                ),
                "evidence": entry,
                "thresholds": {"consecutive_dumps": threshold},
            }
        )
    for entry in growing_locks:
        findings.append(
            {
                "severity": "warning",
                "code": "GROWING_LOCK_CONTENTION",
                "message": (
                    f"Lock {entry['lock_id']} waiter count grew strictly across "
                    f"{entry['consecutive_dumps']} consecutive dumps "
                    f"(max waiters: {entry['max_waiters']})."
                ),
                "evidence": entry,
                "thresholds": {"consecutive_dumps": threshold},
            }
        )
    findings.extend(jvm_metadata_findings)
    return findings


def _build_growing_lock_findings(
    bundles: Sequence[ThreadDumpBundle],
    *,
    threshold: int,
) -> list[dict[str, Any]]:
    """Detect locks whose waiter count strictly increases across consecutive dumps.

    Builds a per-lock waiter-count vector keyed by ``dump_index`` and
    looks for runs of length ``≥ threshold`` where every step increases
    the waiter count. The Java jstack parser is the only one that
    populates ``lock_waiting`` today, so non-Java bundles produce no
    findings — exactly as intended.
    """
    waiters_by_lock: dict[str, dict[int, int]] = {}
    classes_by_lock: dict[str, str | None] = {}
    for bundle in bundles:
        per_dump_counts: dict[str, set[str]] = {}
        per_dump_classes: dict[str, str | None] = {}
        for snapshot in bundle.snapshots:
            waiting = snapshot.lock_waiting
            if waiting is None:
                continue
            if waiting.wait_mode in {"object_wait", "parking_condition_wait"}:
                continue
            per_dump_counts.setdefault(waiting.lock_id, set()).add(snapshot.thread_name)
            per_dump_classes.setdefault(waiting.lock_id, waiting.lock_class)
        for lock_id, names in per_dump_counts.items():
            waiters_by_lock.setdefault(lock_id, {})[bundle.dump_index] = len(names)
            classes_by_lock.setdefault(lock_id, per_dump_classes.get(lock_id))

    findings: list[dict[str, Any]] = []
    for lock_id, per_dump in waiters_by_lock.items():
        ordered_indices = sorted(per_dump.keys())
        if len(ordered_indices) < threshold:
            continue
        # Walk the dump-index timeline; track the longest run with
        # strictly increasing waiter count and consecutive dump indexes.
        longest_run = 0
        longest_max = 0
        current_run = 0
        current_max = 0
        prev_index: int | None = None
        prev_count: int | None = None
        for index in ordered_indices:
            count = per_dump[index]
            if (
                prev_index is not None
                and index == prev_index + 1
                and prev_count is not None
                and count > prev_count
            ):
                current_run += 1
            else:
                current_run = 1
            current_max = max(current_max, count)
            if current_run > longest_run:
                longest_run = current_run
                longest_max = current_max
            prev_index = index
            prev_count = count
        if longest_run >= threshold:
            findings.append(
                {
                    "lock_id": lock_id,
                    "lock_class": classes_by_lock.get(lock_id),
                    "consecutive_dumps": longest_run,
                    "max_waiters": longest_max,
                }
            )
    return findings


# ---------------------------------------------------------------------------
# Optional JVM metadata tables
# ---------------------------------------------------------------------------


def _jvm_metadata_tables(
    bundles: Sequence[ThreadDumpBundle],
    *,
    top_n: int,
) -> dict[str, list[dict[str, Any]]]:
    carrier_pinning: list[dict[str, Any]] = []
    smr_unresolved: list[dict[str, Any]] = []
    native_methods: list[dict[str, Any]] = []
    histogram_rows: list[dict[str, Any]] = []

    for bundle in bundles:
        for snapshot in bundle.snapshots:
            pinning = snapshot.metadata.get("carrier_pinning")
            if isinstance(pinning, dict):
                carrier_pinning.append(
                    {
                        "dump_index": bundle.dump_index,
                        "dump_label": bundle.dump_label,
                        "thread_name": snapshot.thread_name,
                        "thread_id": snapshot.thread_id,
                        "state": snapshot.state.value,
                        "candidate_method": pinning.get("candidate_method"),
                        "top_frame": pinning.get("top_frame"),
                        "reason": pinning.get("reason"),
                    }
                )
            native_method = snapshot.metadata.get("native_method")
            if isinstance(native_method, str):
                native_methods.append(
                    {
                        "dump_index": bundle.dump_index,
                        "dump_label": bundle.dump_label,
                        "thread_name": snapshot.thread_name,
                        "thread_id": snapshot.thread_id,
                        "state": snapshot.state.value,
                        "native_method": native_method,
                        "stack_signature": snapshot.stack_signature(),
                    }
                )

        smr = bundle.metadata.get("smr")
        if isinstance(smr, dict):
            for entry in smr.get("unresolved", []):
                if not isinstance(entry, dict):
                    continue
                smr_unresolved.append(
                    {
                        "dump_index": bundle.dump_index,
                        "dump_label": bundle.dump_label,
                        "section_line": entry.get("section_line"),
                        "line": entry.get("line"),
                    }
                )

        histogram = bundle.metadata.get("class_histogram")
        if isinstance(histogram, dict):
            for entry in histogram.get("classes", []):
                if not isinstance(entry, dict):
                    continue
                histogram_rows.append(
                    {
                        "dump_index": bundle.dump_index,
                        "dump_label": bundle.dump_label,
                        "rank": entry.get("rank"),
                        "class_name": entry.get("class_name"),
                        "instances": entry.get("instances"),
                        "bytes": entry.get("bytes"),
                    }
                )

    histogram_rows.sort(key=lambda row: -int(row.get("bytes") or 0))
    native_methods.sort(key=lambda row: (row["dump_index"], str(row["thread_name"])))
    carrier_pinning.sort(key=lambda row: (row["dump_index"], str(row["thread_name"])))
    return {
        "virtual_thread_carrier_pinning": carrier_pinning[:top_n],
        "smr_unresolved_threads": smr_unresolved[:top_n],
        "native_method_threads": native_methods[:top_n],
        "class_histogram_top_classes": histogram_rows[:top_n],
    }


def _jvm_metadata_findings(
    jvm_tables: dict[str, list[dict[str, Any]]],
    *,
    top_n: int,
) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
    for entry in jvm_tables.get("virtual_thread_carrier_pinning", [])[:top_n]:
        findings.append(
            {
                "severity": "warning",
                "code": "VIRTUAL_THREAD_CARRIER_PINNING",
                "message": (
                    f"Thread {entry['thread_name']!r} contains a virtual-thread "
                    "carrier/pinning marker."
                ),
                "evidence": entry,
            }
        )
    for entry in jvm_tables.get("smr_unresolved_threads", [])[:top_n]:
        findings.append(
            {
                "severity": "warning",
                "code": "SMR_UNRESOLVED_THREAD",
                "message": "JVM SMR diagnostics include an unresolved/zombie thread marker.",
                "evidence": entry,
            }
        )
    return findings


# ---------------------------------------------------------------------------
# T-203 — Cross-language LATENCY_SECTION_DETECTED finding
# ---------------------------------------------------------------------------


def _build_latency_sections(
    timelines: dict[str, ThreadTimeline],
    *,
    threshold: int,
) -> list[dict[str, Any]]:
    """Detect threads that linger in a single wait category across dumps.

    Walks each thread's observation timeline once per category in
    :data:`_LATENCY_WAIT_CATEGORIES`, records the longest consecutive run,
    and emits one entry per (thread, category) pair that hits the
    threshold. Stack signatures are collected so the UI can reconstruct
    the latency hot spot.

    Language-agnostic: the per-language enrichment plugins are responsible
    for promoting RUNNABLE/UNKNOWN states into ``NETWORK_WAIT``,
    ``IO_WAIT``, or ``CHANNEL_WAIT``; this analyzer only consumes the
    normalized :class:`ThreadState` enum.
    """
    out: list[dict[str, Any]] = []
    for timeline in timelines.values():
        sorted_obs = sorted(timeline.observations, key=lambda o: o.dump_index)
        for category in _LATENCY_WAIT_CATEGORIES:
            longest_run = 0
            current_run = 0
            current_signatures: list[str] = []
            best_signatures: list[str] = []
            best_first_index: int | None = None
            current_first_index: int | None = None
            prev_index: int | None = None
            for obs in sorted_obs:
                in_category = obs.state is category
                consecutive = (
                    in_category
                    and prev_index is not None
                    and obs.dump_index == prev_index + 1
                )
                if in_category and not consecutive:
                    current_run = 1
                    current_signatures = [obs.stack_signature]
                    current_first_index = obs.dump_index
                elif in_category and consecutive:
                    current_run += 1
                    current_signatures.append(obs.stack_signature)
                else:
                    current_run = 0
                    current_signatures = []
                    current_first_index = None
                if current_run > longest_run:
                    longest_run = current_run
                    best_signatures = list(current_signatures)
                    best_first_index = current_first_index
                prev_index = obs.dump_index
            if longest_run >= threshold and best_first_index is not None:
                out.append(
                    {
                        "thread_name": timeline.thread_name,
                        "wait_category": category.value,
                        "dumps": longest_run,
                        "first_dump_index": best_first_index,
                        "stack_signatures": sorted(set(best_signatures)),
                    }
                )
    return out


def _state_distribution_per_dump(
    bundles: Sequence[ThreadDumpBundle],
) -> list[dict[str, Any]]:
    distribution: list[dict[str, Any]] = []
    for bundle in bundles:
        counts: Counter[str] = Counter(
            snapshot.state.value for snapshot in bundle.snapshots
        )
        distribution.append(
            {
                "dump_index": bundle.dump_index,
                "dump_label": bundle.dump_label,
                "counts": dict(counts),
            }
        )
    return distribution


def _state_transition_timeline(
    timelines: dict[str, ThreadTimeline],
    *,
    top_n: int,
) -> list[dict[str, Any]]:
    sorted_timelines = sorted(
        timelines.values(),
        key=lambda t: len(t.observations),
        reverse=True,
    )[:top_n]
    rows: list[dict[str, Any]] = []
    for timeline in sorted_timelines:
        rows.append(
            {
                "thread_name": timeline.thread_name,
                "transitions": [
                    {
                        "dump_index": obs.dump_index,
                        "state": obs.state.value,
                        "stack_signature": obs.stack_signature,
                    }
                    for obs in sorted(timeline.observations, key=lambda o: o.dump_index)
                ],
            }
        )
    return rows
