from __future__ import annotations

from typing import TypedDict


class DiagnosticSample(TypedDict):
    line_number: int
    reason: str
    message: str
    raw_preview: str


class ParserDiagnostics(TypedDict):
    total_lines: int
    parsed_records: int
    skipped_lines: int
    skipped_by_reason: dict[str, int]
    samples: list[DiagnosticSample]


class AccessLogSummary(TypedDict):
    total_requests: int
    avg_response_ms: float
    p95_response_ms: float
    p99_response_ms: float
    error_rate: float


class TimeValuePoint(TypedDict):
    time: str
    value: float


class StatusCodeDistributionRow(TypedDict):
    status: str
    count: int


class TopUrlCountRow(TypedDict):
    uri: str
    count: int


class TopUrlAvgResponseRow(TypedDict):
    uri: str
    avg_response_ms: float
    count: int


class AccessLogFinding(TypedDict):
    severity: str
    code: str
    message: str
    evidence: dict[str, str | int | float]


class AccessLogAnalysisOptions(TypedDict):
    max_lines: int | None
    start_time: str | None
    end_time: str | None


class AccessLogSeries(TypedDict):
    requests_per_minute: list[TimeValuePoint]
    avg_response_time_per_minute: list[TimeValuePoint]
    p95_response_time_per_minute: list[TimeValuePoint]
    status_code_distribution: list[StatusCodeDistributionRow]
    top_urls_by_count: list[TopUrlCountRow]
    top_urls_by_avg_response_time: list[TopUrlAvgResponseRow]


class AccessLogSampleRecordRow(TypedDict):
    timestamp: str
    method: str
    uri: str
    status: int
    response_time_ms: float


class AccessLogTables(TypedDict):
    sample_records: list[AccessLogSampleRecordRow]


class AccessLogMetadata(TypedDict):
    format: str
    parser: str
    schema_version: str
    diagnostics: ParserDiagnostics
    analysis_options: AccessLogAnalysisOptions
    findings: list[AccessLogFinding]


class ProfilerCollapsedSummary(TypedDict):
    profile_kind: str
    total_samples: int
    interval_ms: float
    estimated_seconds: float
    elapsed_seconds: float | None


class ProfilerTopStackSeriesRow(TypedDict):
    stack: str
    samples: int
    estimated_seconds: float
    sample_ratio: float
    elapsed_ratio: float | None


class ComponentBreakdownRow(TypedDict):
    component: str
    samples: int


class ProfilerCollapsedSeries(TypedDict):
    top_stacks: list[ProfilerTopStackSeriesRow]
    component_breakdown: list[ComponentBreakdownRow]


class ProfilerTopStackTableRow(ProfilerTopStackSeriesRow):
    frames: list[str]


class ProfilerCollapsedTables(TypedDict):
    top_stacks: list[ProfilerTopStackTableRow]


class ProfilerCollapsedMetadata(TypedDict):
    parser: str
    schema_version: str
    diagnostics: ParserDiagnostics


class JfrRecordingSummary(TypedDict):
    event_count: int
    duration_ms: float
    gc_pause_total_ms: float
    blocked_thread_events: int


class JfrEventOverTimeRow(TypedDict):
    time: str
    event_type: str
    count: int


class JfrPauseEventRow(TypedDict):
    time: str
    duration_ms: float | None
    event_type: str
    thread: str | None
    sampling_type: str


class JfrNotableEventRow(JfrPauseEventRow):
    message: str
    frames: list[str]
    evidence_ref: str
    raw_preview: str


class JfrRecordingSeries(TypedDict):
    events_over_time: list[JfrEventOverTimeRow]
    pause_events: list[JfrPauseEventRow]


class JfrRecordingTables(TypedDict):
    notable_events: list[JfrNotableEventRow]


class JfrRecordingMetadata(TypedDict):
    parser: str
    schema_version: str
    jfr_command_version: str
    event_filters: list[str]
    poc: bool


class EvidenceRefParts(TypedDict):
    source_type: str
    entity_type: str
    identifier: str


class AiFinding(TypedDict, total=False):
    id: str
    label: str
    severity: str
    generated_by: str
    model: str
    summary: str
    reasoning: str
    evidence_refs: list[str]
    evidence_quotes: dict[str, str]
    confidence: float
    limitations: list[str]
    primary_category: str


class InterpretationResult(TypedDict):
    schema_version: str
    provider: str
    model: str
    prompt_version: str
    source_result_type: str
    source_schema_version: str
    generated_at: str
    findings: list[AiFinding]
    disabled: bool
