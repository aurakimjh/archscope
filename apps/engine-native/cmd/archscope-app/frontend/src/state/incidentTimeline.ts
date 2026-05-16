import type {
  AnalysisWorkspaceEntry,
  WorkspaceAnalysisResult,
} from "@/state/analysisWorkspace";

export type IncidentTimelineSeverity = "critical" | "warning" | "info";

export type IncidentTimelineEvent = {
  id: string;
  timestamp: string;
  start_time: string;
  end_time?: string;
  time_label: string;
  range_label: string;
  sort_time: number;
  duration_ms?: number;
  source_analyzer: string;
  source_result_id: string;
  source_title: string;
  source_file?: string;
  severity: IncidentTimelineSeverity;
  category: string;
  group_key: string;
  group_label: string;
  group_category: string;
  correlation_ids: Record<string, string>;
  label: string;
  description: string;
  evidence_ref: string;
  payload: Record<string, unknown>;
};

export type IncidentTimelineGroup = {
  group_key: string;
  group_label: string;
  group_category: string;
  start_time: string;
  end_time: string;
  start_label: string;
  end_label: string;
  duration_ms?: number;
  event_count: number;
  critical_event_count: number;
  warning_event_count: number;
  source_result_ids: string[];
  source_analyzers: string[];
  source_files: string[];
  correlation_ids: Record<string, string[]>;
  evidence_refs: string[];
};

export type IncidentTimelineNarrativeStep = {
  id: string;
  order: number;
  group_key: string;
  group_label: string;
  time_label: string;
  severity: IncidentTimelineSeverity;
  title: string;
  summary: string;
  evidence_refs: string[];
  event_ids: string[];
  source_result_ids: string[];
};

export type IncidentTimelineAnalysisResult = WorkspaceAnalysisResult & {
  type: "incident_timeline";
  summary: {
    event_count: number;
    group_count: number;
    narrative_step_count: number;
    ranged_event_count: number;
    correlated_event_count: number;
    critical_event_count: number;
    warning_event_count: number;
    info_event_count: number;
    source_result_count: number;
    earliest_timestamp: string | null;
    latest_timestamp: string | null;
  };
  series: {
    events_by_severity: Array<{ severity: IncidentTimelineSeverity; count: number }>;
    events_by_category: Array<{ category: string; count: number }>;
    events_by_source_analyzer: Array<{ source_analyzer: string; count: number }>;
    events_by_group: Array<{ group_key: string; group_label: string; count: number }>;
  };
  tables: {
    groups: IncidentTimelineGroup[];
    narrative: IncidentTimelineNarrativeStep[];
    events: IncidentTimelineEvent[];
  };
  metadata: {
    schema_version: "0.1.0";
    projection: "wails_session";
    source_results: Array<{
      id: string;
      title: string;
      result_type: string;
      source_files: string[];
      created_at: string;
      recorded_at: string;
    }>;
  };
};

type FindingLike = {
  severity?: unknown;
  code?: unknown;
  message?: unknown;
  evidence?: unknown;
};

const MAX_EVENTS_PER_RESULT = 120;
const ACCESS_ERROR_RATE_THRESHOLD = 5;
const ACCESS_P95_THRESHOLD_MS = 1_000;
const JFR_PAUSE_THRESHOLD_MS = 100;

export function buildIncidentTimelineEvents(
  entries: AnalysisWorkspaceEntry[],
): IncidentTimelineEvent[] {
  const events = entries.flatMap((entry) => eventsFromEntry(entry));
  return events
    .sort((left, right) => {
      if (left.sort_time !== right.sort_time) return left.sort_time - right.sort_time;
      const severityDelta = severityRank(right.severity) - severityRank(left.severity);
      if (severityDelta !== 0) return severityDelta;
      return left.label.localeCompare(right.label);
    })
    .slice(0, 1_000);
}

export function buildIncidentTimelineAnalysisResult(
  entries: AnalysisWorkspaceEntry[],
): IncidentTimelineAnalysisResult {
  const events = buildIncidentTimelineEvents(entries);
  const groups = buildIncidentTimelineGroups(events);
  const narrative = buildIncidentTimelineNarrative(events);
  const ordered = [...events].sort((left, right) => left.sort_time - right.sort_time);
  return {
    type: "incident_timeline",
    source_files: uniqueStrings(entries.flatMap((entry) => entry.source_files)),
    created_at: new Date().toISOString(),
    summary: {
      event_count: events.length,
      group_count: groups.length,
      narrative_step_count: narrative.length,
      ranged_event_count: events.filter((event) => event.duration_ms !== undefined || event.end_time !== undefined).length,
      correlated_event_count: events.filter((event) => Object.keys(event.correlation_ids).length > 0).length,
      critical_event_count: events.filter((event) => event.severity === "critical").length,
      warning_event_count: events.filter((event) => event.severity === "warning").length,
      info_event_count: events.filter((event) => event.severity === "info").length,
      source_result_count: entries.length,
      earliest_timestamp: ordered[0]?.timestamp ?? null,
      latest_timestamp: ordered[ordered.length - 1]?.timestamp ?? null,
    },
    series: {
      events_by_severity: countEvents(events, "severity").map(([severity, count]) => ({
        severity: severity as IncidentTimelineSeverity,
        count,
      })),
      events_by_category: countEvents(events, "category").map(([category, count]) => ({
        category,
        count,
      })),
      events_by_source_analyzer: countEvents(events, "source_analyzer").map(([source_analyzer, count]) => ({
        source_analyzer,
        count,
      })),
      events_by_group: groups.map((group) => ({
        group_key: group.group_key,
        group_label: group.group_label,
        count: group.event_count,
      })),
    },
    tables: {
      groups,
      narrative,
      events,
    },
    charts: {},
    metadata: {
      schema_version: "0.1.0",
      projection: "wails_session",
      source_results: entries.map((entry) => ({
        id: entry.id,
        title: entry.title,
        result_type: entry.result_type,
        source_files: entry.source_files,
        created_at: entry.created_at,
        recorded_at: entry.recorded_at,
      })),
    },
  };
}

function eventsFromEntry(entry: AnalysisWorkspaceEntry): IncidentTimelineEvent[] {
  const result = entry.result;
  const events: IncidentTimelineEvent[] = [];
  events.push(...eventsFromFindings(entry));
  switch (result.type) {
    case "access_log":
      events.push(...eventsFromAccessLog(entry));
      break;
    case "gc_log":
      events.push(...eventsFromGcLog(entry));
      break;
    case "jfr_recording":
    case "native_memory":
      events.push(...eventsFromJfr(entry));
      break;
    case "exception_stack":
      events.push(...eventsFromException(entry));
      break;
    case "thread_dump":
    case "thread_dump_multi":
    case "multi_thread_dump":
    case "thread_dump_locks":
    case "lock_contention":
      events.push(...eventsFromThreadDump(entry));
      break;
    case "trace_import":
      events.push(...eventsFromTraceImport(entry));
      break;
  }
  return uniqueEvents(events).slice(0, MAX_EVENTS_PER_RESULT);
}

function eventsFromFindings(entry: AnalysisWorkspaceEntry): IncidentTimelineEvent[] {
  const findings = arrayOfObjects(entry.result.metadata?.findings);
  return findings.map((finding, index) => {
    const typed = finding as FindingLike;
    const evidence = objectOrEmpty(typed.evidence);
    const code = stringValue(typed.code) || "FINDING";
    const message = stringValue(typed.message) || code;
    return makeEvent(entry, {
      idSuffix: `finding-${code}-${index}`,
      severity: normalizeSeverity(stringValue(typed.severity)),
      category: findingCategory(code, entry.result.type),
      label: code,
      description: message,
      evidenceRef: code,
      payload: finding,
      timeValue: firstTimeValue(evidence) ?? firstTimeValue(finding),
    });
  });
}

function eventsFromAccessLog(entry: AnalysisWorkspaceEntry): IncidentTimelineEvent[] {
  const result = entry.result;
  const events: IncidentTimelineEvent[] = [];
  const errorRows = arrayOfObjects(result.series?.error_rate_per_minute);
  for (const [index, row] of errorRows.entries()) {
    const total = numberValue(row.total);
    const errors = numberValue(row.errors);
    const rate = numberValue(row.value);
    if (total < 5 || errors <= 0 || rate < ACCESS_ERROR_RATE_THRESHOLD) continue;
    events.push(
      makeEvent(entry, {
        idSuffix: `access-error-rate-${index}`,
        severity: rate >= 10 ? "critical" : "warning",
        category: "error_burst",
        label: "ACCESS_ERROR_BURST",
        description: `${errors}/${total} requests failed (${formatNumber(rate)}%).`,
        evidenceRef: "series.error_rate_per_minute",
        payload: row,
        timeValue: row.time,
      }),
    );
  }

  const p95Rows = arrayOfObjects(result.series?.p95_response_time_per_minute);
  for (const [index, row] of p95Rows.entries()) {
    const p95 = numberValue(row.value);
    if (p95 < ACCESS_P95_THRESHOLD_MS) continue;
    events.push(
      makeEvent(entry, {
        idSuffix: `access-p95-${index}`,
        severity: p95 >= ACCESS_P95_THRESHOLD_MS * 2 ? "critical" : "warning",
        category: "latency",
        label: "ACCESS_P95_LATENCY",
        description: `Access-log p95 latency reached ${formatNumber(p95)} ms.`,
        evidenceRef: "series.p95_response_time_per_minute",
        payload: row,
        timeValue: row.time,
      }),
    );
  }
  return events;
}

function eventsFromGcLog(entry: AnalysisWorkspaceEntry): IncidentTimelineEvent[] {
  const alerts = arrayOfObjects(entry.result.tables?.alerts);
  return alerts.map((alert, index) =>
    makeEvent(entry, {
      idSuffix: `gc-alert-${stringValue(alert.code) || index}`,
      severity: normalizeSeverity(stringValue(alert.severity)),
      category: stringValue(alert.event_category) || "gc",
      label: stringValue(alert.code) || "GC_ALERT",
      description: stringValue(alert.message) || "GC alert",
      evidenceRef: "tables.alerts",
      payload: alert,
      timeValue: alert.time,
    }),
  );
}

function eventsFromJfr(entry: AnalysisWorkspaceEntry): IncidentTimelineEvent[] {
  const events: IncidentTimelineEvent[] = [];
  const pauses = arrayOfObjects(entry.result.series?.pause_events);
  for (const [index, row] of pauses.entries()) {
    const duration = numberValue(row.duration_ms);
    if (duration < JFR_PAUSE_THRESHOLD_MS) continue;
    events.push(
      makeEvent(entry, {
        idSuffix: `jfr-pause-${index}`,
        severity: duration >= 1_000 ? "critical" : "warning",
        category: "runtime_pause",
        label: stringValue(row.event_type) || "JFR_PAUSE",
        description: `JFR pause event lasted ${formatNumber(duration)} ms.`,
        evidenceRef: "series.pause_events",
        payload: row,
        timeValue: row.time,
      }),
    );
  }

  const notable = arrayOfObjects(entry.result.tables?.notable_events);
  for (const [index, row] of notable.entries()) {
    events.push(
      makeEvent(entry, {
        idSuffix: `jfr-notable-${index}`,
        severity: "info",
        category: "runtime_event",
        label: stringValue(row.event_type) || "JFR_NOTABLE_EVENT",
        description: stringValue(row.message) || stringValue(row.thread) || "Notable JFR event",
        evidenceRef: stringValue(row.evidence_ref) || "tables.notable_events",
        payload: row,
        timeValue: row.time,
      }),
    );
  }
  return events;
}

function eventsFromException(entry: AnalysisWorkspaceEntry): IncidentTimelineEvent[] {
  const rows = [
    ...arrayOfObjects(entry.result.tables?.exceptions),
    ...arrayOfObjects(entry.result.tables?.exception_groups),
  ];
  return rows.slice(0, 25).map((row, index) =>
    makeEvent(entry, {
      idSuffix: `exception-${index}`,
      severity: "warning",
      category: "exception",
      label: stringValue(row.exception_type) || stringValue(row.signature) || "EXCEPTION",
      description: stringValue(row.message) || "Exception event",
      evidenceRef: "tables.exceptions",
      payload: row,
      timeValue: firstTimeValue(row),
    }),
  );
}

function eventsFromThreadDump(entry: AnalysisWorkspaceEntry): IncidentTimelineEvent[] {
  const events: IncidentTimelineEvent[] = [];
  const tables = entry.result.tables ?? {};

  if (entry.result.type === "thread_dump" || entry.result.type === "thread_dump_single") {
    for (const [index, row] of arrayOfObjects(tables.threads).entries()) {
      if (!isThreadAttentionRow(row)) continue;
      events.push(
        makeThreadEvent(entry, "threads", row, index, {
          category: "thread_dump",
          label: "THREAD_ATTENTION",
          severity: normalizeThreadSeverity(row),
        }),
      );
    }
    return events;
  }

  if (entry.result.type === "thread_dump_locks" || entry.result.type === "lock_contention") {
    for (const [index, row] of arrayOfObjects(tables.deadlock_chains).entries()) {
      events.push(
        makeThreadEvent(entry, "deadlock_chains", row, index, {
          category: "deadlock",
          label: "DEADLOCK_CHAIN",
          severity: "critical",
        }),
      );
    }
    for (const [index, row] of arrayOfObjects(tables.locks).entries()) {
      if (numberValue(row.waiter_count) <= 0 && row.contention_candidate !== true) continue;
      events.push(
        makeThreadEvent(entry, "locks", row, index, {
          category: "lock_contention",
          label: "LOCK_CONTENTION",
          severity: numberValue(row.waiter_count) >= 5 ? "critical" : "warning",
        }),
      );
    }
    return events;
  }

  const multiTables: Array<{
    name: string;
    category: string;
    label: string;
    severity: IncidentTimelineSeverity;
  }> = [
    { name: "persistent_blocked_threads", category: "persistent_blocked_thread", label: "PERSISTENT_BLOCKED_THREAD", severity: "warning" },
    { name: "long_running_stacks", category: "long_running_thread", label: "LONG_RUNNING_THREAD", severity: "warning" },
    { name: "growing_lock_contention", category: "growing_lock_contention", label: "GROWING_LOCK_CONTENTION", severity: "warning" },
    { name: "latency_sections", category: "thread_latency", label: "THREAD_LATENCY_SECTION", severity: "warning" },
    { name: "virtual_thread_carrier_pinning", category: "carrier_pinning", label: "VIRTUAL_THREAD_CARRIER_PINNING", severity: "warning" },
    { name: "smr_unresolved_threads", category: "smr_unresolved_thread", label: "SMR_UNRESOLVED_THREAD", severity: "warning" },
    { name: "native_method_threads", category: "native_method_thread", label: "NATIVE_METHOD_THREAD", severity: "info" },
  ];
  for (const table of multiTables) {
    for (const [index, row] of arrayOfObjects(tables[table.name]).entries()) {
      events.push(
        makeThreadEvent(entry, table.name, row, index, {
          category: table.category,
          label: table.label,
          severity: table.severity,
        }),
      );
    }
  }
  return events;
}

function makeThreadEvent(
  entry: AnalysisWorkspaceEntry,
  tableName: string,
  row: Record<string, unknown>,
  index: number,
  input: {
    category: string;
    label: string;
    severity: IncidentTimelineSeverity;
  },
): IncidentTimelineEvent {
  return makeEvent(entry, {
    idSuffix: `${tableName}-${index}`,
    severity: input.severity,
    category: input.category,
    label: input.label,
    description:
      stringValue(row.thread_name) ||
      stringValue(row.thread) ||
      stringValue(row.name) ||
      stringValue(row.lock_id) ||
      stringValue(row.signature) ||
      stringValue(row.stack_signature) ||
      stringValue(row.candidate_method) ||
      input.label,
    evidenceRef: `tables.${tableName}`,
    payload: row,
    timeValue: firstTimeValue(row),
  });
}

function isThreadAttentionRow(row: Record<string, unknown>): boolean {
  const state = stringValue(row.state).toUpperCase();
  const category = stringValue(row.category).toUpperCase();
  return (
    state === "BLOCKED" ||
    state === "WAITING" ||
    category === "BLOCKED" ||
    category === "WAITING" ||
    stringValue(row.lock_info) !== ""
  );
}

function normalizeThreadSeverity(row: Record<string, unknown>): IncidentTimelineSeverity {
  const state = stringValue(row.state).toUpperCase();
  const category = stringValue(row.category).toUpperCase();
  return state === "BLOCKED" || category === "BLOCKED" ? "warning" : "info";
}

function eventsFromTraceImport(entry: AnalysisWorkspaceEntry): IncidentTimelineEvent[] {
  const events: IncidentTimelineEvent[] = [];
  const traces = arrayOfObjects(entry.result.tables?.traces);
  for (const [index, row] of traces.entries()) {
    const errors = numberValue(row.error_count);
    if (errors <= 0) continue;
    events.push(
      makeEvent(entry, {
        idSuffix: `trace-error-${stringValue(row.trace_id) || index}`,
        severity: "warning",
        category: "trace_error",
        label: "TRACE_ERRORS",
        description: `${errors} error spans in trace ${stringValue(row.trace_id) || "(unknown)"}.`,
        evidenceRef: "tables.traces",
        payload: row,
        timeValue: firstTraceStart(entry.result, stringValue(row.trace_id)),
      }),
    );
  }

  const criticalPaths = arrayOfObjects(entry.result.tables?.critical_paths);
  for (const [index, row] of criticalPaths.entries()) {
    const duration = numberValue(row.critical_path_ms);
    if (duration < ACCESS_P95_THRESHOLD_MS) continue;
    events.push(
      makeEvent(entry, {
        idSuffix: `trace-critical-path-${stringValue(row.trace_id) || index}`,
        severity: duration >= 3_000 ? "critical" : "warning",
        category: "trace_latency",
        label: "TRACE_CRITICAL_PATH",
        description: `Critical path reached ${formatNumber(duration)} ms.`,
        evidenceRef: "tables.critical_paths",
        payload: row,
        timeValue: firstTraceStart(entry.result, stringValue(row.trace_id)),
      }),
    );
  }
  return events;
}

function makeEvent(
  entry: AnalysisWorkspaceEntry,
  input: {
    idSuffix: string;
    severity: IncidentTimelineSeverity;
    category: string;
    label: string;
    description: string;
    evidenceRef: string;
    payload: Record<string, unknown>;
    timeValue?: unknown;
    endTimeValue?: unknown;
    durationMs?: number;
  },
): IncidentTimelineEvent {
  const fallback = parseTime(entry.result.created_at || entry.recorded_at);
  const parsed = parseTime(input.timeValue) ?? fallback ?? {
    iso: new Date(0).toISOString(),
    label: "unknown",
    sort: 0,
  };
  const duration = input.durationMs ?? durationFromPayload(input.payload);
  const parsedEnd = parseTime(input.endTimeValue ?? firstEndTimeValue(input.payload));
  const end =
    parsedEnd ??
    (duration !== undefined
      ? {
          iso: new Date(parsed.sort + duration).toISOString(),
          label: new Date(parsed.sort + duration).toLocaleString(),
          sort: parsed.sort + duration,
        }
      : undefined);
  const correlation = extractCorrelationIds(input.payload, entry.result);
  const group = inferEventGroup(entry, input.category, input.label, correlation);
  return {
    id: `${entry.id}-${input.idSuffix}`,
    timestamp: parsed.iso,
    start_time: parsed.iso,
    end_time: end?.iso,
    time_label: parsed.label,
    range_label: formatRangeLabel(parsed, end, duration),
    sort_time: parsed.sort,
    duration_ms: duration,
    source_analyzer: entry.result_type,
    source_result_id: entry.id,
    source_title: entry.title,
    source_file: entry.source_files[0],
    severity: input.severity,
    category: input.category,
    group_key: group.key,
    group_label: group.label,
    group_category: group.category,
    correlation_ids: correlation,
    label: input.label,
    description: input.description,
    evidence_ref: input.evidenceRef,
    payload: input.payload,
  };
}

function firstTraceStart(result: WorkspaceAnalysisResult, traceID: string): unknown {
  if (!traceID) return undefined;
  const spans = arrayOfObjects(result.tables?.spans);
  const matches = spans
    .filter((span) => stringValue(span.trace_id) === traceID)
    .map((span) => numberValue(span.start_unix_nanos))
    .filter((value) => value > 0)
    .sort((a, b) => a - b);
  return matches[0];
}

export function buildIncidentTimelineGroups(events: IncidentTimelineEvent[]): IncidentTimelineGroup[] {
  const grouped = new Map<string, IncidentTimelineEvent[]>();
  for (const event of events) {
    const items = grouped.get(event.group_key) ?? [];
    items.push(event);
    grouped.set(event.group_key, items);
  }
  return Array.from(grouped.entries())
    .map(([groupKey, groupEvents]) => buildIncidentTimelineGroup(groupKey, groupEvents))
    .sort((left, right) => Date.parse(left.start_time) - Date.parse(right.start_time) || left.group_label.localeCompare(right.group_label));
}

export function buildIncidentTimelineNarrative(events: IncidentTimelineEvent[]): IncidentTimelineNarrativeStep[] {
  const grouped = new Map<string, IncidentTimelineEvent[]>();
  for (const event of events) {
    const items = grouped.get(event.group_key) ?? [];
    items.push(event);
    grouped.set(event.group_key, items);
  }
  return Array.from(grouped.entries())
    .map(([, groupEvents], index) => buildNarrativeStep(groupEvents, index + 1))
    .sort((left, right) => {
      const leftEvent = events.find((event) => event.id === left.event_ids[0]);
      const rightEvent = events.find((event) => event.id === right.event_ids[0]);
      return (leftEvent?.sort_time ?? 0) - (rightEvent?.sort_time ?? 0) || left.title.localeCompare(right.title);
    })
    .map((step, index) => ({ ...step, order: index + 1 }));
}

function buildNarrativeStep(events: IncidentTimelineEvent[], order: number): IncidentTimelineNarrativeStep {
  const ordered = [...events].sort((left, right) => left.sort_time - right.sort_time);
  const first = ordered[0];
  const last = ordered[ordered.length - 1];
  const highest = [...ordered].sort((left, right) => severityRank(right.severity) - severityRank(left.severity))[0];
  const duration = groupDurationLabel(ordered);
  const sourceLabel = uniqueStrings(ordered.map((event) => event.source_analyzer)).join(", ");
  const eventLabel = events.length === 1 ? "event" : "events";
  const summary =
    events.length === 1
      ? `${first.label} occurred in ${sourceLabel}: ${first.description}`
      : `${events.length} ${eventLabel} were grouped under ${first.group_label}${duration ? ` over ${duration}` : ""}. The first event was ${first.label}; the highest severity event was ${highest.label}; the latest event was ${last.label}.`;
  return {
    id: `narrative-${first.group_key.replace(/[^A-Za-z0-9_.-]/g, "-")}`,
    order,
    group_key: first.group_key,
    group_label: first.group_label,
    time_label: first.range_label,
    severity: highest.severity,
    title: first.group_label,
    summary,
    evidence_refs: uniqueStrings(ordered.flatMap((event) => [event.evidence_ref]).slice(0, 8)),
    event_ids: ordered.map((event) => event.id),
    source_result_ids: uniqueStrings(ordered.map((event) => event.source_result_id)),
  };
}

function groupDurationLabel(events: IncidentTimelineEvent[]): string {
  const first = events[0];
  const last = events.reduce((candidate, event) => eventEndSort(event) > eventEndSort(candidate) ? event : candidate, first);
  const duration = eventEndSort(last) - first.sort_time;
  return duration > 0 ? `${formatNumber(duration)} ms` : "";
}

function buildIncidentTimelineGroup(groupKey: string, events: IncidentTimelineEvent[]): IncidentTimelineGroup {
  const ordered = [...events].sort((left, right) => left.sort_time - right.sort_time);
  const first = ordered[0];
  const last = ordered.reduce((candidate, event) => eventEndSort(event) > eventEndSort(candidate) ? event : candidate, first);
  const start = first.start_time;
  const end = last.end_time ?? last.start_time;
  const startMs = Date.parse(start);
  const endMs = Date.parse(end);
  const duration = Number.isFinite(startMs) && Number.isFinite(endMs) && endMs > startMs ? endMs - startMs : undefined;
  return {
    group_key: groupKey,
    group_label: first.group_label,
    group_category: first.group_category,
    start_time: start,
    end_time: end,
    start_label: first.time_label,
    end_label: last.end_time ? new Date(end).toLocaleString() : last.time_label,
    duration_ms: duration,
    event_count: events.length,
    critical_event_count: events.filter((event) => event.severity === "critical").length,
    warning_event_count: events.filter((event) => event.severity === "warning").length,
    source_result_ids: uniqueStrings(events.map((event) => event.source_result_id)),
    source_analyzers: uniqueStrings(events.map((event) => event.source_analyzer)),
    source_files: uniqueStrings(events.map((event) => event.source_file ?? "")),
    correlation_ids: mergeCorrelationIds(events),
    evidence_refs: uniqueStrings(events.map((event) => event.evidence_ref)),
  };
}

function eventEndSort(event: IncidentTimelineEvent): number {
  if (event.end_time) {
    const parsed = Date.parse(event.end_time);
    if (Number.isFinite(parsed)) return parsed;
  }
  return event.sort_time;
}

function mergeCorrelationIds(events: IncidentTimelineEvent[]): Record<string, string[]> {
  const out: Record<string, string[]> = {};
  for (const event of events) {
    for (const [key, value] of Object.entries(event.correlation_ids)) {
      out[key] = uniqueStrings([...(out[key] ?? []), value]);
    }
  }
  return out;
}

function uniqueEvents(events: IncidentTimelineEvent[]): IncidentTimelineEvent[] {
  const seen = new Set<string>();
  const out: IncidentTimelineEvent[] = [];
  for (const event of events) {
    const key = [
      event.source_result_id,
      event.timestamp,
      event.severity,
      event.label,
      event.evidence_ref,
      JSON.stringify(event.payload).slice(0, 120),
    ].join("\x00");
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(event);
  }
  return out;
}

function findingCategory(code: string, resultType: string): string {
  const normalized = code.toLowerCase();
  if (normalized.includes("error") || normalized.includes("exception")) return "error";
  if (normalized.includes("pause") || normalized.includes("gc")) return "runtime_pause";
  if (normalized.includes("slow") || normalized.includes("latency")) return "latency";
  if (normalized.includes("deadlock") || normalized.includes("blocked")) return "contention";
  if (normalized.includes("trace")) return "trace";
  return resultType || "finding";
}

function firstTimeValue(source: Record<string, unknown>): unknown {
  for (const key of [
    "timestamp",
    "time",
    "minute",
    "start_time",
    "start",
    "first_seen",
    "earliest_timestamp",
    "latest_timestamp",
    "start_unix_nanos",
  ]) {
    if (source[key] !== undefined && source[key] !== null && source[key] !== "") {
      return source[key];
    }
  }
  return undefined;
}

function firstEndTimeValue(source: Record<string, unknown>): unknown {
  for (const key of [
    "end_time",
    "end",
    "finish_time",
    "finished_at",
    "last_seen",
    "latest_timestamp",
    "end_unix_nanos",
  ]) {
    if (source[key] !== undefined && source[key] !== null && source[key] !== "") {
      return source[key];
    }
  }
  return undefined;
}

function durationFromPayload(source: Record<string, unknown>): number | undefined {
  for (const key of [
    "duration_ms",
    "elapsed_ms",
    "latency_ms",
    "response_time_ms",
    "critical_path_ms",
    "total_duration_ms",
    "pause_ms",
  ]) {
    const value = numberValue(source[key]);
    if (value > 0) return value;
  }
  const seconds = numberValue(source.duration_sec) || numberValue(source.elapsed_sec);
  if (seconds > 0) return seconds * 1_000;
  const nanos = numberValue(source.duration_nanos) || numberValue(source.duration_ns);
  if (nanos > 0) return nanos / 1_000_000;
  return undefined;
}

function extractCorrelationIds(
  payload: Record<string, unknown>,
  result: WorkspaceAnalysisResult,
): Record<string, string> {
  const out: Record<string, string> = {};
  const metadata = objectOrEmpty(result.metadata);
  for (const key of [
    "trace_id",
    "span_id",
    "parent_span_id",
    "request_id",
    "correlation_id",
    "transaction_id",
    "txn_id",
    "thread_id",
    "thread_name",
    "service",
    "service_name",
    "endpoint",
    "uri",
    "method",
  ]) {
    const value = stringValue(payload[key]) || stringValue(metadata[key]);
    if (value) out[normalizeCorrelationKey(key)] = value;
  }
  return out;
}

function normalizeCorrelationKey(key: string): string {
  if (key === "txn_id") return "transaction_id";
  if (key === "thread_name") return "thread";
  if (key === "service_name") return "service";
  return key;
}

function inferEventGroup(
  entry: AnalysisWorkspaceEntry,
  category: string,
  label: string,
  correlation: Record<string, string>,
): { key: string; label: string; category: string } {
  for (const key of ["trace_id", "request_id", "correlation_id", "transaction_id", "thread_id", "thread"]) {
    const value = correlation[key];
    if (value) {
      return {
        key: `${key}:${value}`,
        label: `${key}: ${value}`,
        category: key,
      };
    }
  }
  if (correlation.service) {
    const endpoint = correlation.endpoint || correlation.uri || label;
    return {
      key: `service:${correlation.service}:${endpoint}`,
      label: `${correlation.service} / ${endpoint}`,
      category: "service",
    };
  }
  const source = entry.source_files[0] || entry.result_type;
  return {
    key: `category:${category}:${label}:${source}`,
    label: `${category} / ${label}`,
    category,
  };
}

function formatRangeLabel(
  start: { iso: string; label: string; sort: number },
  end: { iso: string; label: string; sort: number } | undefined,
  durationMs: number | undefined,
): string {
  if (end && end.iso !== start.iso) return `${start.label} - ${end.label}`;
  if (durationMs !== undefined) return `${start.label} (+${formatNumber(durationMs)} ms)`;
  return start.label;
}

function parseTime(value: unknown): { iso: string; label: string; sort: number } | null {
  if (typeof value === "number" && Number.isFinite(value)) {
    const millis =
      value > 10_000_000_000_000_000
        ? Math.floor(value / 1_000_000)
        : value > 10_000_000_000_000
        ? Math.floor(value / 1_000)
        : value > 10_000_000_000
        ? value
        : null;
    if (millis !== null) {
      const date = new Date(millis);
      if (!Number.isNaN(date.getTime())) {
        return { iso: date.toISOString(), label: date.toLocaleString(), sort: date.getTime() };
      }
    }
    return null;
  }
  if (typeof value !== "string") return null;
  const trimmed = value.trim();
  if (!trimmed) return null;
  const parsed = Date.parse(trimmed);
  if (!Number.isNaN(parsed)) {
    const date = new Date(parsed);
    return { iso: date.toISOString(), label: date.toLocaleString(), sort: parsed };
  }
  return null;
}

function normalizeSeverity(value: string): IncidentTimelineSeverity {
  const normalized = value.toLowerCase();
  if (normalized === "critical" || normalized === "error" || normalized === "fatal") {
    return "critical";
  }
  if (normalized === "warning" || normalized === "warn") return "warning";
  return "info";
}

function severityRank(severity: IncidentTimelineSeverity): number {
  switch (severity) {
    case "critical":
      return 3;
    case "warning":
      return 2;
    default:
      return 1;
  }
}

function arrayOfObjects(value: unknown): Record<string, unknown>[] {
  return Array.isArray(value)
    ? value.filter((item): item is Record<string, unknown> => isPlainObject(item))
    : [];
}

function objectOrEmpty(value: unknown): Record<string, unknown> {
  return isPlainObject(value) ? value : {};
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function numberValue(value: unknown): number {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

function formatNumber(value: number): string {
  return Number.isFinite(value)
    ? value.toLocaleString(undefined, { maximumFractionDigits: 2 })
    : String(value);
}

function countEvents(
  events: IncidentTimelineEvent[],
  key: "severity" | "category" | "source_analyzer",
): Array<[string, number]> {
  const counts = new Map<string, number>();
  for (const event of events) {
    const value = event[key];
    counts.set(value, (counts.get(value) ?? 0) + 1);
  }
  return Array.from(counts.entries()).sort((left, right) => right[1] - left[1] || left[0].localeCompare(right[0]));
}

function uniqueStrings(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((left, right) => left.localeCompare(right));
}
