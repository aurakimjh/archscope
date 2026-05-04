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
    long_running.sort(key=lambda finding: finding["dumps"], reverse=True)
    persistent_blocked.sort(key=lambda finding: finding["dumps"], reverse=True)
    latency_sections.sort(key=lambda finding: finding["dumps"], reverse=True)

    findings_payload = _findings_payload(
        long_running=long_running,
        persistent_blocked=persistent_blocked,
        latency_sections=latency_sections,
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
        "dumps": [
            {
                "dump_index": bundle.dump_index,
                "dump_label": bundle.dump_label,
                "source_file": bundle.source_file,
                "source_format": bundle.source_format,
                "language": bundle.language,
                "thread_count": len(bundle.snapshots),
            }
            for bundle in bundles
        ],
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


def _findings_payload(
    *,
    long_running: list[dict[str, Any]],
    persistent_blocked: list[dict[str, Any]],
    latency_sections: list[dict[str, Any]],
    threshold: int,
) -> list[dict[str, Any]]:
    findings: list[dict[str, Any]] = []
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
