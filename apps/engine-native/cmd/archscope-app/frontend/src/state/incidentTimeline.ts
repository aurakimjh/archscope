import type {
  AnalysisWorkspaceEntry,
  WorkspaceAnalysisResult,
} from "@/state/analysisWorkspace";

export type IncidentTimelineSeverity = "critical" | "warning" | "info";

export type IncidentTimelineEvent = {
  id: string;
  timestamp: string;
  time_label: string;
  sort_time: number;
  source_analyzer: string;
  source_result_id: string;
  source_title: string;
  source_file?: string;
  severity: IncidentTimelineSeverity;
  category: string;
  label: string;
  description: string;
  evidence_ref: string;
  payload: Record<string, unknown>;
};

export type IncidentTimelineAnalysisResult = WorkspaceAnalysisResult & {
  type: "incident_timeline";
  summary: {
    event_count: number;
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
  };
  tables: {
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
  const ordered = [...events].sort((left, right) => left.sort_time - right.sort_time);
  return {
    type: "incident_timeline",
    source_files: uniqueStrings(entries.flatMap((entry) => entry.source_files)),
    created_at: new Date().toISOString(),
    summary: {
      event_count: events.length,
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
    },
    tables: {
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
    case "multi_thread_dump":
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
  for (const tableName of ["deadlocks", "contended_locks", "blocked_threads", "long_running_threads"]) {
    for (const [index, row] of arrayOfObjects(tables[tableName]).entries()) {
      events.push(
        makeEvent(entry, {
          idSuffix: `${tableName}-${index}`,
          severity: tableName === "deadlocks" ? "critical" : "warning",
          category: tableName,
          label: tableName.toUpperCase(),
          description:
            stringValue(row.thread_name) ||
            stringValue(row.thread) ||
            stringValue(row.lock_id) ||
            stringValue(row.signature) ||
            tableName,
          evidenceRef: `tables.${tableName}`,
          payload: row,
          timeValue: firstTimeValue(row),
        }),
      );
    }
  }
  return events;
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
  },
): IncidentTimelineEvent {
  const fallback = parseTime(entry.result.created_at || entry.recorded_at);
  const parsed = parseTime(input.timeValue) ?? fallback ?? {
    iso: new Date(0).toISOString(),
    label: "unknown",
    sort: 0,
  };
  return {
    id: `${entry.id}-${input.idSuffix}`,
    timestamp: parsed.iso,
    time_label: parsed.label,
    sort_time: parsed.sort,
    source_analyzer: entry.result_type,
    source_result_id: entry.id,
    source_title: entry.title,
    source_file: entry.source_files[0],
    severity: input.severity,
    category: input.category,
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
