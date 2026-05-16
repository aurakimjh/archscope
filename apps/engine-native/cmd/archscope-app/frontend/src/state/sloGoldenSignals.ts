import type {
  AnalysisWorkspaceEntry,
  WorkspaceAnalysisResult,
} from "@/state/analysisWorkspace";
import { canonicalServiceIdentity } from "./serviceFlow.js";

export type GoldenSignalKind = "latency" | "traffic" | "errors" | "saturation";

export type GoldenSignalScopeType =
  | "global"
  | "service"
  | "endpoint"
  | "edge"
  | "runtime"
  | "thread"
  | "trace"
  | "jvm";

export type GoldenSignalAggregation =
  | "avg"
  | "p50"
  | "p90"
  | "p95"
  | "p99"
  | "max"
  | "min"
  | "sum"
  | "count"
  | "rate"
  | "latest"
  | "peak";

export type GoldenSignalUnit =
  | "ms"
  | "percent"
  | "count"
  | "requests"
  | "requests_per_sec"
  | "bytes"
  | "bytes_per_sec"
  | "spans"
  | "calls"
  | "events"
  | "mb"
  | "mb_per_sec"
  | "threads";

export type GoldenSignalTags = Record<string, string | number | boolean>;

export type GoldenSignal = {
  id: string;
  kind: GoldenSignalKind;
  name: string;
  value: number;
  unit: GoldenSignalUnit;
  aggregation: GoldenSignalAggregation;
  scope_type: GoldenSignalScopeType;
  scope: string;
  source_analyzer: string;
  source_result_id: string;
  source_title: string;
  source_file?: string;
  evidence_ref: string;
  payload: Record<string, unknown>;
  tags: GoldenSignalTags;
};

export type GoldenSignalInventory = {
  generated_at: string;
  source_count: number;
  signal_count: number;
  by_kind: Record<GoldenSignalKind, number>;
  by_source: Record<string, number>;
  signals: GoldenSignal[];
};

export type SliMetric = {
  id: string;
  metric_key: string;
  kind: GoldenSignalKind;
  name: string;
  value: number;
  unit: GoldenSignalUnit;
  aggregation: GoldenSignalAggregation;
  scope_type: GoldenSignalScopeType;
  scope: string;
  source_analyzers: string[];
  source_result_ids: string[];
  evidence_refs: string[];
  signal_ids: string[];
  contributor_count: number;
  tags: GoldenSignalTags;
};

export type SliMetricModel = {
  generated_at: string;
  signal_count: number;
  metric_count: number;
  by_kind: Record<GoldenSignalKind, number>;
  by_scope_type: Record<GoldenSignalScopeType, number>;
  metrics: SliMetric[];
};

export type SloComparator = "lte" | "gte";
export type SloSeverity = "critical" | "warning" | "info";

export type SloTargetMatch = {
  kind?: GoldenSignalKind;
  units?: GoldenSignalUnit[];
  aggregations?: GoldenSignalAggregation[];
  scopeTypes?: GoldenSignalScopeType[];
  sourceAnalyzers?: string[];
  nameIncludes?: string[];
  evidenceIncludes?: string[];
  tagEquals?: GoldenSignalTags;
};

export type SloTarget = {
  id: string;
  name: string;
  severity: SloSeverity;
  comparator: SloComparator;
  threshold: number;
  unit: GoldenSignalUnit;
  objective_percent?: number;
  window_label: string;
  match: SloTargetMatch;
};

export type SloTargetOverride = Partial<Omit<SloTarget, "id">> & {
  id: string;
};

export type SloViolation = {
  id: string;
  target_id: string;
  target_name: string;
  severity: SloSeverity;
  metric_id: string;
  metric_key: string;
  metric_name: string;
  kind: GoldenSignalKind;
  actual: number;
  threshold: number;
  comparator: SloComparator;
  unit: GoldenSignalUnit;
  window_label: string;
  delta: number;
  ratio_to_threshold: number;
  burn_rate: number;
  error_budget_consumed_percent: number;
  error_budget_remaining_percent: number;
  affected_scope_type: GoldenSignalScopeType;
  affected_scope: string;
  source_analyzers: string[];
  source_result_ids: string[];
  evidence_refs: string[];
  tags: GoldenSignalTags;
};

export type ErrorBudgetRow = {
  target_id: string;
  target_name: string;
  severity: SloSeverity;
  affected_scope_type: GoldenSignalScopeType;
  affected_scope: string;
  actual: number;
  threshold: number;
  comparator: SloComparator;
  unit: GoldenSignalUnit;
  consumed_percent: number;
  remaining_percent: number;
  burn_rate: number;
  evidence_refs: string[];
};

export type AffectedBreakdownRow = {
  affected_scope_type: GoldenSignalScopeType;
  affected_scope: string;
  violation_count: number;
  max_severity: SloSeverity;
  max_burn_rate: number;
  target_names: string[];
  source_analyzers: string[];
  evidence_refs: string[];
};

export type SloAnalysisResult = {
  generated_at: string;
  window_label: string;
  target_count: number;
  metric_count: number;
  violation_count: number;
  violations_by_severity: Record<SloSeverity, number>;
  targets: SloTarget[];
  metrics: SliMetric[];
  violations: SloViolation[];
  error_budget_rows: ErrorBudgetRow[];
  affected_breakdown: AffectedBreakdownRow[];
};

export type SloAnalysisExportResult = WorkspaceAnalysisResult &
  SloAnalysisResult & {
    type: "slo_golden_signals";
    source_files: string[];
    created_at: string;
    summary: {
      source_count: number;
      signal_count: number;
      metric_count: number;
      target_count: number;
      violation_count: number;
      critical_violation_count: number;
      warning_violation_count: number;
    };
    series: {
      signals_by_kind: Array<{ kind: GoldenSignalKind; count: number }>;
      metrics_by_kind: Array<{ kind: GoldenSignalKind; count: number }>;
      metrics_by_scope_type: Array<{ scope_type: GoldenSignalScopeType; count: number }>;
    };
    tables: {
      targets: SloTarget[];
      signals: GoldenSignal[];
      metrics: SliMetric[];
      violations: SloViolation[];
      error_budget_rows: ErrorBudgetRow[];
      affected_breakdown: AffectedBreakdownRow[];
    };
    charts: Record<string, never>;
    metadata: {
      schema_version: "0.1.0";
      parser: "wails_session_slo_golden_signals";
      projection: "wails_session";
      diagnostics: Record<string, unknown>;
      findings: Array<Record<string, unknown>>;
      extra: {
        source_results: Array<{
          id: string;
          result_type: string;
          source_files: string[];
        }>;
      };
    };
  };

type SignalInput = {
  kind: GoldenSignalKind;
  name: string;
  value: unknown;
  unit: GoldenSignalUnit;
  aggregation: GoldenSignalAggregation;
  scopeType?: GoldenSignalScopeType;
  scope?: string;
  evidenceRef: string;
  payload?: Record<string, unknown>;
  tags?: GoldenSignalTags;
};

const MAX_SIGNALS = 1_500;
const MAX_ROW_SIGNALS = 80;

export const DEFAULT_SLO_TARGETS: SloTarget[] = [
  {
    id: "latency-p95-1s-warning",
    name: "p95 latency under 1s",
    severity: "warning",
    comparator: "lte",
    threshold: 1_000,
    unit: "ms",
    objective_percent: 95,
    window_label: "session",
    match: { kind: "latency", units: ["ms"], aggregations: ["p95"], nameIncludes: ["latency"] },
  },
  {
    id: "latency-p95-3s-critical",
    name: "p95 latency under 3s",
    severity: "critical",
    comparator: "lte",
    threshold: 3_000,
    unit: "ms",
    objective_percent: 99,
    window_label: "session",
    match: { kind: "latency", units: ["ms"], aggregations: ["p95"], nameIncludes: ["latency"] },
  },
  {
    id: "trace-critical-path-5s-critical",
    name: "Trace critical path under 5s",
    severity: "critical",
    comparator: "lte",
    threshold: 5_000,
    unit: "ms",
    objective_percent: 99,
    window_label: "session",
    match: { kind: "latency", units: ["ms"], nameIncludes: ["critical", "path"] },
  },
  {
    id: "msa-network-gap-avg-50ms-warning",
    name: "MSA average network gap under 50ms",
    severity: "warning",
    comparator: "lte",
    threshold: 50,
    unit: "ms",
    objective_percent: 95,
    window_label: "session",
    match: {
      kind: "latency",
      units: ["ms"],
      aggregations: ["avg"],
      scopeTypes: ["edge"],
      tagEquals: { dimension: "network_gap" },
    },
  },
  {
    id: "msa-network-gap-p95-100ms-warning",
    name: "MSA p95 network gap under 100ms",
    severity: "warning",
    comparator: "lte",
    threshold: 100,
    unit: "ms",
    objective_percent: 95,
    window_label: "session",
    match: {
      kind: "latency",
      units: ["ms"],
      aggregations: ["p95"],
      scopeTypes: ["edge"],
      tagEquals: { dimension: "network_gap" },
    },
  },
  {
    id: "msa-network-gap-p95-500ms-critical",
    name: "MSA p95 network gap under 500ms",
    severity: "critical",
    comparator: "lte",
    threshold: 500,
    unit: "ms",
    objective_percent: 99,
    window_label: "session",
    match: {
      kind: "latency",
      units: ["ms"],
      aggregations: ["p95"],
      scopeTypes: ["edge"],
      tagEquals: { dimension: "network_gap" },
    },
  },
  {
    id: "error-rate-5pct-warning",
    name: "Error rate under 5%",
    severity: "warning",
    comparator: "lte",
    threshold: 5,
    unit: "percent",
    objective_percent: 95,
    window_label: "session",
    match: { kind: "errors", units: ["percent"], nameIncludes: ["error", "rate"] },
  },
  {
    id: "error-rate-10pct-critical",
    name: "Error rate under 10%",
    severity: "critical",
    comparator: "lte",
    threshold: 10,
    unit: "percent",
    objective_percent: 90,
    window_label: "session",
    match: { kind: "errors", units: ["percent"], nameIncludes: ["error", "rate"] },
  },
  {
    id: "gc-p99-pause-200ms-warning",
    name: "GC p99 pause under 200ms",
    severity: "warning",
    comparator: "lte",
    threshold: 200,
    unit: "ms",
    objective_percent: 99,
    window_label: "session",
    match: { kind: "latency", units: ["ms"], aggregations: ["p99"], nameIncludes: ["gc", "pause"] },
  },
  {
    id: "gc-max-pause-1s-critical",
    name: "GC max pause under 1s",
    severity: "critical",
    comparator: "lte",
    threshold: 1_000,
    unit: "ms",
    objective_percent: 99,
    window_label: "session",
    match: { kind: "latency", units: ["ms"], aggregations: ["max"], nameIncludes: ["gc", "pause"] },
  },
  {
    id: "gc-throughput-95pct-warning",
    name: "GC throughput at least 95%",
    severity: "warning",
    comparator: "gte",
    threshold: 95,
    unit: "percent",
    objective_percent: 95,
    window_label: "session",
    match: { kind: "saturation", units: ["percent"], nameIncludes: ["gc", "throughput"] },
  },
  {
    id: "gc-throughput-90pct-critical",
    name: "GC throughput at least 90%",
    severity: "critical",
    comparator: "gte",
    threshold: 90,
    unit: "percent",
    objective_percent: 99,
    window_label: "session",
    match: { kind: "saturation", units: ["percent"], nameIncludes: ["gc", "throughput"] },
  },
  {
    id: "oom-zero-critical",
    name: "No OutOfMemory events",
    severity: "critical",
    comparator: "lte",
    threshold: 0,
    unit: "events",
    objective_percent: 100,
    window_label: "session",
    match: { kind: "errors", units: ["events"], nameIncludes: ["outofmemory"] },
  },
  {
    id: "deadlock-zero-critical",
    name: "No detected deadlocks",
    severity: "critical",
    comparator: "lte",
    threshold: 0,
    unit: "count",
    objective_percent: 100,
    window_label: "session",
    match: { kind: "errors", units: ["count"], nameIncludes: ["deadlock"] },
  },
  {
    id: "blocked-thread-zero-warning",
    name: "No blocked-thread signals",
    severity: "warning",
    comparator: "lte",
    threshold: 0,
    unit: "threads",
    objective_percent: 95,
    window_label: "session",
    match: { kind: "saturation", units: ["threads"], nameIncludes: ["blocked"] },
  },
  {
    id: "trace-integrity-zero-warning",
    name: "No trace integrity errors",
    severity: "warning",
    comparator: "lte",
    threshold: 0,
    unit: "spans",
    objective_percent: 95,
    window_label: "session",
    match: { kind: "errors", units: ["spans"], nameIncludes: ["trace"] },
  },
  {
    id: "jennifer-unmatched-zero-warning",
    name: "No unmatched Jennifer MSA calls",
    severity: "warning",
    comparator: "lte",
    threshold: 0,
    unit: "calls",
    objective_percent: 95,
    window_label: "session",
    match: { kind: "errors", units: ["calls"], nameIncludes: ["unmatched"] },
  },
];

export function buildGoldenSignalInventory(entries: AnalysisWorkspaceEntry[]): GoldenSignalInventory {
  const signals = dedupeSignals(entries.flatMap((entry) => signalsFromEntry(entry)))
    .sort(compareSignals)
    .slice(0, MAX_SIGNALS);
  return {
    generated_at: new Date().toISOString(),
    source_count: entries.length,
    signal_count: signals.length,
    by_kind: countByKind(signals),
    by_source: countBySource(signals),
    signals,
  };
}

export function buildGoldenSignals(entries: AnalysisWorkspaceEntry[]): GoldenSignal[] {
  return buildGoldenSignalInventory(entries).signals;
}

export function buildSliMetricModel(input: GoldenSignalInventory | GoldenSignal[]): SliMetricModel {
  const signals = Array.isArray(input) ? input : input.signals;
  const metrics = Array.from(groupSignalsBySli(signals).values()).sort(compareSliMetrics);
  return {
    generated_at: new Date().toISOString(),
    signal_count: signals.length,
    metric_count: metrics.length,
    by_kind: countMetricsByKind(metrics),
    by_scope_type: countMetricsByScope(metrics),
    metrics,
  };
}

export function buildSliMetricModelFromEntries(entries: AnalysisWorkspaceEntry[]): SliMetricModel {
  return buildSliMetricModel(buildGoldenSignalInventory(entries));
}

export function buildSliMetrics(input: GoldenSignalInventory | GoldenSignal[]): SliMetric[] {
  return buildSliMetricModel(input).metrics;
}

export function analyzeSloViolations(
  input: SliMetricModel | SliMetric[],
  targets: SloTarget[] = DEFAULT_SLO_TARGETS,
): SloAnalysisResult {
  const metrics = Array.isArray(input) ? input : input.metrics;
  const violations = detectSloViolations(metrics, targets);
  return {
    generated_at: new Date().toISOString(),
    window_label: "session",
    target_count: targets.length,
    metric_count: metrics.length,
    violation_count: violations.length,
    violations_by_severity: countViolationsBySeverity(violations),
    targets,
    metrics,
    violations,
    error_budget_rows: buildErrorBudgetRows(violations),
    affected_breakdown: buildAffectedBreakdown(violations),
  };
}

export function applySloTargetOverrides(
  targets: SloTarget[] = DEFAULT_SLO_TARGETS,
  overrides: SloTargetOverride[] = [],
): SloTarget[] {
  if (overrides.length === 0) return targets;
  const byID = new Map(targets.map((target) => [target.id, target]));
  for (const override of overrides) {
    const existing = byID.get(override.id);
    byID.set(override.id, existing ? { ...existing, ...override } : (override as SloTarget));
  }
  return Array.from(byID.values());
}

export function analyzeSloViolationsFromEntries(
  entries: AnalysisWorkspaceEntry[],
  targets: SloTarget[] = DEFAULT_SLO_TARGETS,
): SloAnalysisResult {
  return analyzeSloViolations(buildSliMetricModelFromEntries(entries), targets);
}

export function buildSloAnalysisResultFromEntries(
  entries: AnalysisWorkspaceEntry[],
  targets: SloTarget[] = DEFAULT_SLO_TARGETS,
): SloAnalysisExportResult {
  const inventory = buildGoldenSignalInventory(entries);
  const metricModel = buildSliMetricModel(inventory);
  const analysis = analyzeSloViolations(metricModel, targets);
  return {
    ...analysis,
    type: "slo_golden_signals",
    source_files: uniqueSorted(entries.flatMap((entry) => entry.source_files)),
    created_at: analysis.generated_at,
    summary: {
      source_count: entries.length,
      signal_count: inventory.signal_count,
      metric_count: metricModel.metric_count,
      target_count: analysis.target_count,
      violation_count: analysis.violation_count,
      critical_violation_count: analysis.violations_by_severity.critical,
      warning_violation_count: analysis.violations_by_severity.warning,
    },
    series: {
      signals_by_kind: Object.entries(inventory.by_kind).map(([kind, count]) => ({
        kind: kind as GoldenSignalKind,
        count,
      })),
      metrics_by_kind: Object.entries(metricModel.by_kind).map(([kind, count]) => ({
        kind: kind as GoldenSignalKind,
        count,
      })),
      metrics_by_scope_type: Object.entries(metricModel.by_scope_type).map(([scope_type, count]) => ({
        scope_type: scope_type as GoldenSignalScopeType,
        count,
      })),
    },
    tables: {
      targets: analysis.targets,
      signals: inventory.signals,
      metrics: metricModel.metrics,
      violations: analysis.violations,
      error_budget_rows: analysis.error_budget_rows,
      affected_breakdown: analysis.affected_breakdown,
    },
    charts: {},
    metadata: {
      schema_version: "0.1.0",
      parser: "wails_session_slo_golden_signals",
      projection: "wails_session",
      diagnostics: {
        source_count: entries.length,
        signal_count: inventory.signal_count,
        metric_count: metricModel.metric_count,
        target_count: analysis.target_count,
        violation_count: analysis.violation_count,
      },
      findings: analysis.violations.map((violation) => ({
        severity: violation.severity,
        code: violation.target_id,
        message: violation.target_name,
        evidence: {
          metric: violation.metric_name,
          actual: violation.actual,
          threshold: violation.threshold,
          affected_scope: violation.affected_scope,
        },
        evidence_refs: violation.evidence_refs,
      })),
      extra: {
        source_results: entries.map((entry) => ({
          id: entry.id,
          result_type: entry.result_type,
          source_files: entry.source_files,
        })),
      },
    },
  };
}

export function detectSloViolations(
  metrics: SliMetric[],
  targets: SloTarget[] = DEFAULT_SLO_TARGETS,
): SloViolation[] {
  const violations: SloViolation[] = [];
  for (const target of targets) {
    for (const metric of metrics) {
      if (!matchesTarget(metric, target)) continue;
      if (!comparisonViolates(metric.value, target.comparator, target.threshold)) continue;
      const budget = computeErrorBudget(metric.value, target);
      violations.push({
        id: `slo-${hashString([target.id, metric.id, String(metric.value)].join("\x00"))}`,
        target_id: target.id,
        target_name: target.name,
        severity: target.severity,
        metric_id: metric.id,
        metric_key: metric.metric_key,
        metric_name: metric.name,
        kind: metric.kind,
        actual: round(metric.value, 3),
        threshold: target.threshold,
        comparator: target.comparator,
        unit: target.unit,
        window_label: target.window_label,
        delta: round(violationDelta(metric.value, target.comparator, target.threshold), 3),
        ratio_to_threshold: round(ratioToThreshold(metric.value, target.comparator, target.threshold), 3),
        burn_rate: round(budget.burnRate, 3),
        error_budget_consumed_percent: round(budget.consumedPercent, 2),
        error_budget_remaining_percent: round(budget.remainingPercent, 2),
        affected_scope_type: metric.scope_type,
        affected_scope: metric.scope,
        source_analyzers: metric.source_analyzers,
        source_result_ids: metric.source_result_ids,
        evidence_refs: metric.evidence_refs,
        tags: metric.tags,
      });
    }
  }
  return violations.sort(compareViolations);
}

export function buildErrorBudgetRows(violations: SloViolation[]): ErrorBudgetRow[] {
  return violations
    .map((violation) => ({
      target_id: violation.target_id,
      target_name: violation.target_name,
      severity: violation.severity,
      affected_scope_type: violation.affected_scope_type,
      affected_scope: violation.affected_scope,
      actual: violation.actual,
      threshold: violation.threshold,
      comparator: violation.comparator,
      unit: violation.unit,
      consumed_percent: violation.error_budget_consumed_percent,
      remaining_percent: violation.error_budget_remaining_percent,
      burn_rate: violation.burn_rate,
      evidence_refs: violation.evidence_refs,
    }))
    .sort((left, right) => severityRankSlo(right.severity) - severityRankSlo(left.severity) || right.burn_rate - left.burn_rate);
}

export function buildAffectedBreakdown(violations: SloViolation[]): AffectedBreakdownRow[] {
  const grouped = new Map<string, SloViolation[]>();
  for (const violation of violations) {
    const key = `${violation.affected_scope_type}\x00${violation.affected_scope}`;
    const bucket = grouped.get(key);
    if (bucket) {
      bucket.push(violation);
    } else {
      grouped.set(key, [violation]);
    }
  }

  return Array.from(grouped.values())
    .map((bucket) => {
      const representative = bucket[0];
      return {
        affected_scope_type: representative.affected_scope_type,
        affected_scope: representative.affected_scope,
        violation_count: bucket.length,
        max_severity: bucket.reduce(
          (severity, violation) =>
            severityRankSlo(violation.severity) > severityRankSlo(severity) ? violation.severity : severity,
          "info" as SloSeverity,
        ),
        max_burn_rate: maxNumber(bucket.map((violation) => violation.burn_rate)),
        target_names: uniqueSorted(bucket.map((violation) => violation.target_name)),
        source_analyzers: uniqueSorted(bucket.flatMap((violation) => violation.source_analyzers)),
        evidence_refs: uniqueSorted(bucket.flatMap((violation) => violation.evidence_refs)),
      };
    })
    .sort(
      (left, right) =>
        severityRankSlo(right.max_severity) - severityRankSlo(left.max_severity) ||
        right.max_burn_rate - left.max_burn_rate ||
        right.violation_count - left.violation_count,
    );
}

function signalsFromEntry(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const type = entry.result.type || entry.result_type;
  let signals: GoldenSignal[] = [];
  switch (type) {
    case "access_log":
      signals = signalsFromAccessLog(entry);
      break;
    case "trace_import":
      signals = signalsFromTraceImport(entry);
      break;
    case "jennifer_profile":
      signals = signalsFromJenniferMsa(entry);
      break;
    case "exception_stack":
      signals = signalsFromException(entry);
      break;
    case "gc_log":
      signals = signalsFromGcLog(entry);
      break;
    case "server_log":
      signals = signalsFromServerLog(entry);
      break;
    case "metrics_snapshot":
      signals = signalsFromMetricsSnapshot(entry);
      break;
    case "observability_evidence":
      signals = signalsFromObservabilityEvidence(entry);
      break;
    case "database_slow_query":
      signals = signalsFromDatabaseLog(entry);
      break;
    case "jfr_recording":
      signals = signalsFromJfr(entry);
      break;
    case "nodejs_stack":
    case "python_traceback":
    case "go_panic":
    case "dotnet_exception_iis":
      signals = signalsFromRuntimeStack(entry);
      break;
    case "thread_dump":
    case "thread_dump_multi":
    case "multi_thread_dump":
    case "lock_contention":
    case "thread_dump_locks":
      signals = signalsFromThreadDump(entry);
      break;
    default:
      if (Array.isArray(entry.result.tables?.msa_edges)) {
        signals = signalsFromJenniferMsa(entry);
      } else {
        signals = signalsFromGenericJvm(entry);
      }
  }
  return [...signals, ...signalsFromJvmInfo(entry)];
}

function signalsFromAccessLog(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "avg_response_ms", "Access average latency", "latency", "ms", "avg");
  addSummary(out, entry, "p50_response_ms", "Access p50 latency", "latency", "ms", "p50");
  addSummary(out, entry, "p90_response_ms", "Access p90 latency", "latency", "ms", "p90");
  addSummary(out, entry, "p95_response_ms", "Access p95 latency", "latency", "ms", "p95");
  addSummary(out, entry, "p99_response_ms", "Access p99 latency", "latency", "ms", "p99");
  addSummary(out, entry, "total_requests", "Access request volume", "traffic", "requests", "count");
  addSummary(out, entry, "avg_requests_per_sec", "Access request rate", "traffic", "requests_per_sec", "rate");
  addSummary(out, entry, "total_bytes", "Access response bytes", "traffic", "bytes", "sum");
  addSummary(out, entry, "avg_bytes_per_sec", "Access byte rate", "traffic", "bytes_per_sec", "rate");
  addSummary(out, entry, "error_rate", "Access error rate", "errors", "percent", "rate", {
    tags: { metric_family: "access_error_rate", rate_unit: "percent" },
  });
  addSummary(out, entry, "error_count", "Access error count", "errors", "count", "count");

  for (const row of arrayOfObjects(entry.result.tables?.url_stats).slice(0, MAX_ROW_SIGNALS)) {
    const endpoint = endpointScope(row);
    addRowSignal(out, entry, row, {
      key: "avg_response_ms",
      name: "Endpoint average latency",
      kind: "latency",
      unit: "ms",
      aggregation: "avg",
      scopeType: "endpoint",
      scope: endpoint,
      evidenceRef: "tables.url_stats",
    });
    addRowSignal(out, entry, row, {
      key: "p95_response_ms",
      name: "Endpoint p95 latency",
      kind: "latency",
      unit: "ms",
      aggregation: "p95",
      scopeType: "endpoint",
      scope: endpoint,
      evidenceRef: "tables.url_stats",
    });
    addRowSignal(out, entry, row, {
      key: "p99_response_ms",
      name: "Endpoint p99 latency",
      kind: "latency",
      unit: "ms",
      aggregation: "p99",
      scopeType: "endpoint",
      scope: endpoint,
      evidenceRef: "tables.url_stats",
    });
    addRowSignal(out, entry, row, {
      key: "count",
      name: "Endpoint request volume",
      kind: "traffic",
      unit: "requests",
      aggregation: "count",
      scopeType: "endpoint",
      scope: endpoint,
      evidenceRef: "tables.url_stats",
    });
    addRowSignal(out, entry, row, {
      key: "error_count",
      name: "Endpoint error count",
      kind: "errors",
      unit: "count",
      aggregation: "count",
      scopeType: "endpoint",
      scope: endpoint,
      evidenceRef: "tables.url_stats",
    });
  }

  for (const row of arrayOfObjects(entry.result.tables?.service_dependencies).slice(0, MAX_ROW_SIGNALS)) {
    const scope = edgeScope(row, "caller", "callee");
    addRowSignal(out, entry, row, {
      key: "avg_duration_ms",
      name: "Access edge average latency",
      kind: "latency",
      unit: "ms",
      aggregation: "avg",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.service_dependencies",
    });
    addRowSignal(out, entry, row, {
      key: "call_count",
      name: "Access edge request volume",
      kind: "traffic",
      unit: "requests",
      aggregation: "count",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.service_dependencies",
      tags: { metric_family: "service_edge_traffic", dedupe_policy: "equivalent_edge_max" },
    });
    addRowSignal(out, entry, row, {
      key: "error_rate",
      name: "Access edge error rate",
      kind: "errors",
      unit: "percent",
      aggregation: "rate",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.service_dependencies",
      tags: {
        metric_family: "service_edge_error_rate",
        rate_unit: "fraction",
      },
    });
    addRowSignal(out, entry, row, {
      key: "error_count",
      name: "Access edge error count",
      kind: "errors",
      unit: "count",
      aggregation: "count",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.service_dependencies",
      tags: { metric_family: "service_edge_errors" },
    });
  }
  return out;
}

function signalsFromTraceImport(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "p95_trace_duration_ms", "Trace p95 duration", "latency", "ms", "p95", {
    scopeType: "trace",
  });
  addSummary(out, entry, "max_trace_duration_ms", "Trace max duration", "latency", "ms", "max", {
    scopeType: "trace",
  });
  addSummary(out, entry, "total_spans", "Trace span volume", "traffic", "spans", "count");
  addSummary(out, entry, "unique_traces", "Trace volume", "traffic", "count", "count", {
    tags: { dimension: "trace_count" },
  });
  addSummary(out, entry, "error_spans", "Trace error spans", "errors", "spans", "count");
  addSummary(out, entry, "missing_parent_spans", "Trace missing-parent spans", "errors", "spans", "count");
  addSummary(out, entry, "clock_skew_suspected_spans", "Trace clock-skew spans", "errors", "spans", "count");

  for (const row of arrayOfObjects(entry.result.tables?.service_summary).slice(0, MAX_ROW_SIGNALS)) {
    const service = stringValue(row.service) || "unknown-service";
    addRowSignal(out, entry, row, {
      key: "avg_duration_ms",
      name: "Service average span latency",
      kind: "latency",
      unit: "ms",
      aggregation: "avg",
      scopeType: "service",
      scope: service,
      evidenceRef: "tables.service_summary",
    });
    addRowSignal(out, entry, row, {
      key: "span_count",
      name: "Service span volume",
      kind: "traffic",
      unit: "spans",
      aggregation: "count",
      scopeType: "service",
      scope: service,
      evidenceRef: "tables.service_summary",
    });
    addRowSignal(out, entry, row, {
      key: "error_count",
      name: "Service span errors",
      kind: "errors",
      unit: "spans",
      aggregation: "count",
      scopeType: "service",
      scope: service,
      evidenceRef: "tables.service_summary",
    });
  }

  for (const row of arrayOfObjects(entry.result.tables?.service_dependencies).slice(0, MAX_ROW_SIGNALS)) {
    const scope = edgeScope(row, "caller", "callee");
    addRowSignal(out, entry, row, {
      key: "avg_duration_ms",
      name: "Dependency average latency",
      kind: "latency",
      unit: "ms",
      aggregation: "avg",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.service_dependencies",
    });
    addRowSignal(out, entry, row, {
      key: "call_count",
      name: "Dependency call volume",
      kind: "traffic",
      unit: "calls",
      aggregation: "count",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.service_dependencies",
      tags: { metric_family: "service_edge_traffic", dedupe_policy: "equivalent_edge_max" },
    });
    addRowSignal(out, entry, row, {
      key: "error_rate",
      name: "Dependency error rate",
      kind: "errors",
      unit: "percent",
      aggregation: "rate",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.service_dependencies",
      tags: {
        metric_family: "service_edge_error_rate",
        rate_unit: "fraction",
      },
    });
    addRowSignal(out, entry, row, {
      key: "error_count",
      name: "Dependency error count",
      kind: "errors",
      unit: "count",
      aggregation: "count",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.service_dependencies",
      tags: { metric_family: "service_edge_errors" },
    });
  }

  for (const row of arrayOfObjects(entry.result.tables?.critical_paths).slice(0, MAX_ROW_SIGNALS)) {
    addRowSignal(out, entry, row, {
      key: "critical_path_ms",
      name: "Trace critical-path latency",
      kind: "latency",
      unit: "ms",
      aggregation: "max",
      scopeType: "trace",
      scope: stringValue(row.trace_id) || "unknown-trace",
      evidenceRef: "tables.critical_paths",
    });
  }
  return out;
}

function signalsFromJenniferMsa(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "external_call_cum_ms", "Jennifer external-call elapsed time", "latency", "ms", "sum");
  addSummary(out, entry, "external_call_count", "Jennifer external-call volume", "traffic", "calls", "count");
  addSummary(out, entry, "matched_external_call_count", "Jennifer matched external calls", "traffic", "calls", "count");
  addSummary(out, entry, "unmatched_external_call_count", "Jennifer unmatched external calls", "errors", "calls", "count");
  addSummary(out, entry, "total_unprofiled_external_call_ms", "Jennifer unprofiled external-call time", "latency", "ms", "sum");
  addSummary(out, entry, "total_network_gap_cum_ms", "Jennifer network gap time", "latency", "ms", "sum", {
    tags: { dimension: "network_gap" },
  });

  for (const row of arrayOfObjects(entry.result.tables?.msa_edges).slice(0, MAX_ROW_SIGNALS)) {
    const scope = jenniferEdgeScope(row);
    addRowSignal(out, entry, row, {
      key: "external_call_elapsed_ms",
      name: "MSA edge external-call latency",
      kind: "latency",
      unit: "ms",
      aggregation: "sum",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.msa_edges",
      tags: { dimension: "external_call_elapsed" },
    });
    addRowSignal(out, entry, row, {
      key: "adjusted_network_gap_ms",
      name: "MSA edge network gap",
      kind: "latency",
      unit: "ms",
      aggregation: "sum",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.msa_edges",
      tags: { dimension: "network_gap" },
    });
    addRowSignal(out, entry, row, {
      key: "raw_network_gap_ms",
      name: "MSA edge raw network gap",
      kind: "latency",
      unit: "ms",
      aggregation: "sum",
      scopeType: "edge",
      scope,
      evidenceRef: "tables.msa_edges",
      tags: { dimension: "raw_network_gap" },
    });
    if (stringValue(row.match_status).toLowerCase() !== "matched") {
      addSignal(out, entry, {
        kind: "errors",
        name: "MSA unmatched external call",
        value: 1,
        unit: "count",
        aggregation: "count",
        scopeType: "edge",
        scope,
        evidenceRef: "tables.msa_edges",
        payload: row,
        tags: { dimension: "msa_match", match_status: stringValue(row.match_status) || "unknown" },
      });
    }
  }

  for (const row of arrayOfObjects(entry.result.series?.service_call_network_summary).slice(0, MAX_ROW_SIGNALS)) {
    const scope = serviceCallScope(row);
    addRowSignal(out, entry, row, {
      key: "avg_network_gap_ms",
      name: "Service-call average network gap",
      kind: "latency",
      unit: "ms",
      aggregation: "avg",
      scopeType: "edge",
      scope,
      evidenceRef: "series.service_call_network_summary",
      tags: { dimension: "network_gap" },
    });
    addRowSignal(out, entry, row, {
      key: "p95_network_gap_ms",
      name: "Service-call p95 network gap",
      kind: "latency",
      unit: "ms",
      aggregation: "p95",
      scopeType: "edge",
      scope,
      evidenceRef: "series.service_call_network_summary",
      tags: { dimension: "network_gap" },
    });
    addRowSignal(out, entry, row, {
      key: "call_count",
      name: "Service-call volume",
      kind: "traffic",
      unit: "calls",
      aggregation: "count",
      scopeType: "edge",
      scope,
      evidenceRef: "series.service_call_network_summary",
      tags: { metric_family: "service_edge_traffic", dedupe_policy: "equivalent_edge_max" },
    });
  }
  return out;
}

function signalsFromException(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "total_exceptions", "Exception volume", "errors", "count", "count");
  addSummary(out, entry, "unique_exception_types", "Unique exception types", "errors", "count", "count");
  addSummary(out, entry, "unique_signatures", "Unique exception signatures", "errors", "count", "count");

  for (const row of arrayOfObjects(entry.result.series?.exception_type_distribution).slice(0, MAX_ROW_SIGNALS)) {
    addRowSignal(out, entry, row, {
      key: "count",
      name: "Exception type volume",
      kind: "errors",
      unit: "count",
      aggregation: "count",
      scopeType: "runtime",
      scope: stringValue(row.exception_type) || "unknown-exception",
      evidenceRef: "series.exception_type_distribution",
    });
  }
  return out;
}

function signalsFromGcLog(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "avg_pause_ms", "GC average pause", "latency", "ms", "avg", { scopeType: "runtime" });
  addSummary(out, entry, "p50_pause_ms", "GC p50 pause", "latency", "ms", "p50", { scopeType: "runtime" });
  addSummary(out, entry, "p95_pause_ms", "GC p95 pause", "latency", "ms", "p95", { scopeType: "runtime" });
  addSummary(out, entry, "p99_pause_ms", "GC p99 pause", "latency", "ms", "p99", { scopeType: "runtime" });
  addSummary(out, entry, "max_pause_ms", "GC max pause", "latency", "ms", "max", { scopeType: "runtime" });
  addSummary(out, entry, "total_pause_ms", "GC total pause", "latency", "ms", "sum", { scopeType: "runtime" });
  addSummary(out, entry, "total_events", "GC event volume", "traffic", "events", "count", { scopeType: "runtime" });
  addSummary(out, entry, "young_gc_count", "Young GC count", "traffic", "events", "count", { scopeType: "runtime" });
  addSummary(out, entry, "full_gc_count", "Full GC count", "traffic", "events", "count", { scopeType: "runtime" });
  addSummary(out, entry, "throughput_percent", "GC throughput", "saturation", "percent", "avg", { scopeType: "runtime" });
  addSummary(out, entry, "avg_allocation_rate_mb_per_sec", "Allocation rate", "saturation", "mb_per_sec", "rate", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "avg_promotion_rate_mb_per_sec", "Promotion rate", "saturation", "mb_per_sec", "rate", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "humongous_allocation_count", "Humongous allocation count", "saturation", "count", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "concurrent_mode_failure_count", "Concurrent mode failure count", "errors", "count", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "promotion_failure_count", "Promotion failure count", "errors", "count", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "long_pause_event_count", "Long GC pause count", "saturation", "events", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "critical_pause_event_count", "Critical GC pause count", "errors", "events", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "oom_event_count", "OutOfMemory event count", "errors", "events", "count", {
    scopeType: "runtime",
  });

  addPeakSeries(out, entry, "heap_after_mb", "Peak heap after GC", "heap");
  addPeakSeries(out, entry, "heap_committed_mb", "Peak committed heap", "heap_committed");
  addPeakSeries(out, entry, "young_after_mb", "Peak young generation after GC", "young_gen");
  addPeakSeries(out, entry, "old_after_mb", "Peak old generation after GC", "old_gen");
  addPeakSeries(out, entry, "metaspace_after_mb", "Peak metaspace after GC", "metaspace");

  for (const row of arrayOfObjects(entry.result.tables?.alerts).slice(0, MAX_ROW_SIGNALS)) {
    addSignal(out, entry, {
      kind: stringValue(row.severity).toLowerCase() === "critical" ? "errors" : "saturation",
      name: stringValue(row.code) || "GC alert",
      value: 1,
      unit: "events",
      aggregation: "count",
      scopeType: "runtime",
      scope: stringValue(row.event_category) || "gc",
      evidenceRef: "tables.alerts",
      payload: row,
      tags: { severity: stringValue(row.severity) || "warning" },
    });
  }
  return out;
}

function signalsFromServerLog(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "total_events", "Server log event volume", "traffic", "events", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "error_count", "Server log error count", "errors", "count", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "worker_error_count", "Web server worker errors", "errors", "count", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "stuck_thread_count", "Stuck thread count", "saturation", "count", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "hung_thread_count", "Hung thread count", "saturation", "count", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "datasource_event_count", "Datasource warning count", "saturation", "count", "count", {
    scopeType: "runtime",
  });
  return out;
}

function signalsFromMetricsSnapshot(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  for (const row of arrayOfObjects(entry.result.tables?.golden_signal_candidates).slice(0, MAX_ROW_SIGNALS)) {
    addSignal(out, entry, {
      kind: (stringValue(row.kind) as GoldenSignalKind) || "traffic",
      name: stringValue(row.name) || "Metric sample",
      value: row.value,
      unit: "count",
      aggregation: "latest",
      scopeType: "runtime",
      scope: stringValue(objectOrEmpty(row.labels).service) || stringValue(row.name) || "metrics",
      evidenceRef: "tables.golden_signal_candidates",
      payload: row,
    });
  }
  return out;
}

function signalsFromObservabilityEvidence(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "error_count", "Observability error count", "errors", "count", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "dashboard_panel_count", "Grafana panel references", "traffic", "count", "count", {
    scopeType: "runtime",
  });
  return out;
}

function signalsFromDatabaseLog(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "p95_query_ms", "Database query p95 latency", "latency", "ms", "p95", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "slow_query_count", "Database slow query count", "saturation", "count", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "error_count", "Database error count", "errors", "count", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "lock_wait_count", "Database lock wait count", "saturation", "count", "count", {
    scopeType: "runtime",
  });
  return out;
}

function signalsFromJfr(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "duration_ms", "JFR selected duration", "latency", "ms", "sum", { scopeType: "runtime" });
  addSummary(out, entry, "gc_pause_total_ms", "JFR GC pause total", "latency", "ms", "sum", { scopeType: "runtime" });
  addSummary(out, entry, "event_count", "JFR event volume", "traffic", "events", "count", { scopeType: "runtime" });
  addSummary(out, entry, "event_count_total", "JFR total event volume", "traffic", "events", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "blocked_thread_events", "JFR blocked-thread events", "saturation", "events", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "sample_event_count", "JFR sample event volume", "traffic", "events", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "stack_sample_count", "JFR stack sample volume", "traffic", "events", "count", {
    scopeType: "runtime",
  });

  for (const row of arrayOfObjects(entry.result.series?.pause_events).slice(0, MAX_ROW_SIGNALS)) {
    addRowSignal(out, entry, row, {
      key: "duration_ms",
      name: "JFR pause latency",
      kind: "latency",
      unit: "ms",
      aggregation: "max",
      scopeType: "runtime",
      scope: stringValue(row.event_type) || "pause",
      evidenceRef: "series.pause_events",
    });
  }
  for (const row of arrayOfObjects(entry.result.tables?.top_threads).slice(0, MAX_ROW_SIGNALS)) {
    addRowSignal(out, entry, row, {
      key: "samples",
      name: "JFR sampled thread load",
      kind: "saturation",
      unit: "events",
      aggregation: "count",
      scopeType: "thread",
      scope: stringValue(row.thread) || "unknown-thread",
      evidenceRef: "tables.top_threads",
    });
  }
  return out;
}

function signalsFromRuntimeStack(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "total_records", "Runtime stack record volume", "errors", "events", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "unique_record_types", "Runtime stack record types", "errors", "count", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "unique_signatures", "Runtime stack signatures", "errors", "count", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "total_exceptions", "Runtime exception volume", "errors", "events", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "iis_requests", ".NET IIS request volume", "traffic", "requests", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "iis_error_requests", ".NET IIS error requests", "errors", "requests", "count", {
    scopeType: "runtime",
  });
  addSummary(out, entry, "max_iis_time_taken_ms", ".NET IIS max time taken", "latency", "ms", "max", {
    scopeType: "runtime",
  });
  for (const row of arrayOfObjects(entry.result.series?.top_stack_signatures).slice(0, MAX_ROW_SIGNALS)) {
    addRowSignal(out, entry, row, {
      key: "count",
      name: "Runtime stack signature frequency",
      kind: "errors",
      unit: "events",
      aggregation: "count",
      scopeType: "runtime",
      scope: stringValue(row.signature) || "runtime-stack",
      evidenceRef: "series.top_stack_signatures",
    });
  }
  return out;
}

function signalsFromThreadDump(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addSummary(out, entry, "total_threads", "Thread count", "saturation", "threads", "count", { scopeType: "thread" });
  addSummary(out, entry, "runnable_threads", "Runnable thread count", "traffic", "threads", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "blocked_threads", "Blocked thread count", "saturation", "threads", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "waiting_threads", "Waiting thread count", "saturation", "threads", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "threads_with_locks", "Threads with locks", "saturation", "threads", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "long_running_threads", "Long-running threads", "saturation", "threads", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "persistent_blocked_threads", "Persistent blocked threads", "saturation", "threads", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "virtual_thread_carrier_pinning", "Virtual-thread carrier pinning", "saturation", "threads", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "native_method_threads", "Native-method threads", "saturation", "threads", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "threads_waiting_on_lock", "Threads waiting on locks", "saturation", "threads", "count", {
    scopeType: "thread",
  });
  addSummary(out, entry, "contended_locks", "Contended locks", "saturation", "count", "count", { scopeType: "thread" });
  addSummary(out, entry, "deadlocks_detected", "Detected deadlocks", "errors", "count", "count", { scopeType: "thread" });

  for (const tableName of [
    "long_running_stacks",
    "persistent_blocked_threads",
    "growing_lock_contention",
    "virtual_thread_carrier_pinning",
    "native_method_threads",
    "locks",
    "deadlock_chains",
  ]) {
    const rows = arrayOfObjects(entry.result.tables?.[tableName]);
    if (rows.length === 0) continue;
    addSignal(out, entry, {
      kind: tableName === "deadlock_chains" ? "errors" : "saturation",
      name: humanizeKey(tableName),
      value: rows.length,
      unit: "count",
      aggregation: "count",
      scopeType: "thread",
      scope: tableName,
      evidenceRef: `tables.${tableName}`,
      payload: { count: rows.length, table: tableName },
    });
  }
  return out;
}

function signalsFromJvmInfo(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  const jvmInfo = objectOrEmpty(entry.result.metadata?.jvm_info);
  addJvmInfoSignal(out, entry, jvmInfo, "cpus_available", "JVM available CPUs", "count", "runtime");
  addJvmInfoSignal(out, entry, jvmInfo, "cpus_total", "JVM total CPUs", "count", "runtime");
  addJvmInfoSignal(out, entry, jvmInfo, "memory_mb", "Host memory", "mb", "memory");
  addJvmInfoSignal(out, entry, jvmInfo, "heap_min_mb", "JVM minimum heap", "mb", "heap");
  addJvmInfoSignal(out, entry, jvmInfo, "heap_initial_mb", "JVM initial heap", "mb", "heap");
  addJvmInfoSignal(out, entry, jvmInfo, "heap_max_mb", "JVM maximum heap", "mb", "heap");
  addJvmInfoSignal(out, entry, jvmInfo, "heap_region_size_mb", "JVM heap region size", "mb", "heap");
  addJvmInfoSignal(out, entry, jvmInfo, "parallel_workers", "JVM parallel GC workers", "threads", "gc_workers");
  addJvmInfoSignal(out, entry, jvmInfo, "concurrent_workers", "JVM concurrent GC workers", "threads", "gc_workers");
  return out;
}

function signalsFromGenericJvm(entry: AnalysisWorkspaceEntry): GoldenSignal[] {
  const out: GoldenSignal[] = [];
  addGenericSummary(out, entry, "cpu_usage_percent", "CPU usage", "saturation", "percent");
  addGenericSummary(out, entry, "process_cpu_percent", "Process CPU usage", "saturation", "percent");
  addGenericSummary(out, entry, "heap_used_mb", "Heap used", "saturation", "mb");
  addGenericSummary(out, entry, "heap_committed_mb", "Heap committed", "saturation", "mb");
  addGenericSummary(out, entry, "heap_max_mb", "Heap max", "saturation", "mb");
  addGenericSummary(out, entry, "non_heap_used_mb", "Non-heap used", "saturation", "mb");
  addGenericSummary(out, entry, "metaspace_used_mb", "Metaspace used", "saturation", "mb");
  addGenericSummary(out, entry, "thread_count", "Thread count", "saturation", "threads");
  addGenericSummary(out, entry, "daemon_thread_count", "Daemon thread count", "saturation", "threads");
  addGenericSummary(out, entry, "loaded_class_count", "Loaded class count", "saturation", "count");
  return out;
}

function addSummary(
  out: GoldenSignal[],
  entry: AnalysisWorkspaceEntry,
  key: string,
  name: string,
  kind: GoldenSignalKind,
  unit: GoldenSignalUnit,
  aggregation: GoldenSignalAggregation,
  options: {
    scopeType?: GoldenSignalScopeType;
    scope?: string;
    tags?: GoldenSignalTags;
  } = {},
): void {
  const summary = entry.result.summary ?? {};
  addSignal(out, entry, {
    kind,
    name,
    value: summary[key],
    unit,
    aggregation,
    scopeType: options.scopeType,
    scope: options.scope,
    evidenceRef: `summary.${key}`,
    payload: { [key]: summary[key] },
    tags: options.tags,
  });
}

function addGenericSummary(
  out: GoldenSignal[],
  entry: AnalysisWorkspaceEntry,
  key: string,
  name: string,
  kind: GoldenSignalKind,
  unit: GoldenSignalUnit,
): void {
  addSummary(out, entry, key, name, kind, unit, "latest", { scopeType: "jvm" });
}

function addRowSignal(
  out: GoldenSignal[],
  entry: AnalysisWorkspaceEntry,
  row: Record<string, unknown>,
  input: Omit<SignalInput, "value" | "payload"> & { key: string },
): void {
  addSignal(out, entry, {
    ...input,
    value: row[input.key],
    payload: row,
  });
}

function addPeakSeries(
  out: GoldenSignal[],
  entry: AnalysisWorkspaceEntry,
  seriesKey: string,
  name: string,
  scope: string,
): void {
  const rows = arrayOfObjects(entry.result.series?.[seriesKey]);
  const values = rows.map((row) => finiteNumber(row.value)).filter((value): value is number => value !== null);
  if (values.length === 0) return;
  const peak = maxNumber(values);
  addSignal(out, entry, {
    kind: "saturation",
    name,
    value: peak,
    unit: "mb",
    aggregation: "peak",
    scopeType: "runtime",
    scope,
    evidenceRef: `series.${seriesKey}`,
    payload: { points: values.length, peak_mb: peak },
  });
}

function addJvmInfoSignal(
  out: GoldenSignal[],
  entry: AnalysisWorkspaceEntry,
  jvmInfo: Record<string, unknown>,
  key: string,
  name: string,
  unit: GoldenSignalUnit,
  scope: string,
): void {
  addSignal(out, entry, {
    kind: "saturation",
    name,
    value: jvmInfo[key],
    unit,
    aggregation: "latest",
    scopeType: "jvm",
    scope,
    evidenceRef: `metadata.jvm_info.${key}`,
    payload: { [key]: jvmInfo[key] },
  });
}

function addSignal(out: GoldenSignal[], entry: AnalysisWorkspaceEntry, input: SignalInput): void {
  const tags = input.tags ?? {};
  const value = normalizedSignalValue(input.value, input.unit, tags);
  if (value === null) return;
  const scope = input.scope || defaultScope(input.scopeType);
  const signal: GoldenSignal = {
    id: `gs-${hashString(
      [entry.id, input.evidenceRef, input.name, input.kind, scope, String(value)].join("\x00"),
    )}`,
    kind: input.kind,
    name: input.name,
    value,
    unit: input.unit,
    aggregation: input.aggregation,
    scope_type: input.scopeType ?? "global",
    scope,
    source_analyzer: entry.result.type || entry.result_type,
    source_result_id: entry.id,
    source_title: entry.title,
    source_file: entry.source_files[0],
    evidence_ref: input.evidenceRef,
    payload: input.payload ?? {},
    tags,
  };
  out.push(signal);
}

function normalizedSignalValue(
  rawValue: unknown,
  unit: GoldenSignalUnit,
  tags: GoldenSignalTags,
): number | null {
  const value = finiteNumber(rawValue);
  if (value === null) return null;
  if (unit !== "percent") return value;
  const rateUnit = stringValue(tags.rate_unit).toLowerCase();
  if (rateUnit === "fraction") return round(value * 100, 3);
  return value;
}

function dedupeSignals(signals: GoldenSignal[]): GoldenSignal[] {
  const seen = new Set<string>();
  const out: GoldenSignal[] = [];
  for (const signal of signals) {
    const key = [
      signal.source_result_id,
      signal.kind,
      signal.name,
      signal.scope_type,
      signal.scope,
      signal.evidence_ref,
      signal.value,
    ].join("\x00");
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(signal);
  }
  return out;
}

function compareSignals(left: GoldenSignal, right: GoldenSignal): number {
  const kindDelta = kindRank(left.kind) - kindRank(right.kind);
  if (kindDelta !== 0) return kindDelta;
  if (left.source_analyzer !== right.source_analyzer) {
    return left.source_analyzer.localeCompare(right.source_analyzer);
  }
  if (left.scope_type !== right.scope_type) return left.scope_type.localeCompare(right.scope_type);
  if (left.scope !== right.scope) return left.scope.localeCompare(right.scope);
  return right.value - left.value;
}

function kindRank(kind: GoldenSignalKind): number {
  switch (kind) {
    case "latency":
      return 1;
    case "traffic":
      return 2;
    case "errors":
      return 3;
    case "saturation":
      return 4;
  }
}

function countByKind(signals: GoldenSignal[]): Record<GoldenSignalKind, number> {
  const counts: Record<GoldenSignalKind, number> = {
    latency: 0,
    traffic: 0,
    errors: 0,
    saturation: 0,
  };
  for (const signal of signals) counts[signal.kind] += 1;
  return counts;
}

function countBySource(signals: GoldenSignal[]): Record<string, number> {
  const counts: Record<string, number> = {};
  for (const signal of signals) {
    counts[signal.source_analyzer] = (counts[signal.source_analyzer] ?? 0) + 1;
  }
  return counts;
}

function groupSignalsBySli(signals: GoldenSignal[]): Map<string, SliMetric> {
  const grouped = new Map<string, GoldenSignal[]>();
  for (const signal of signals) {
    const key = sliMetricKey(signal);
    const bucket = grouped.get(key);
    if (bucket) {
      bucket.push(signal);
    } else {
      grouped.set(key, [signal]);
    }
  }

  const metrics = new Map<string, SliMetric>();
  for (const [key, bucket] of grouped.entries()) {
    const representative = bucket[0];
    const value = aggregateSignalValues(representative.kind, representative.aggregation, bucket);
    metrics.set(key, {
      id: `sli-${hashString(key)}`,
      metric_key: key,
      kind: representative.kind,
      name: representative.name,
      value,
      unit: representative.unit,
      aggregation: representative.aggregation,
      scope_type: representative.scope_type,
      scope: representative.scope,
      source_analyzers: uniqueSorted(bucket.map((signal) => signal.source_analyzer)),
      source_result_ids: uniqueSorted(bucket.map((signal) => signal.source_result_id)),
      evidence_refs: uniqueSorted(bucket.map((signal) => signal.evidence_ref)),
      signal_ids: uniqueSorted(bucket.map((signal) => signal.id)),
      contributor_count: bucket.length,
      tags: mergeTags(bucket.map((signal) => signal.tags)),
    });
  }
  return metrics;
}

function sliMetricKey(signal: GoldenSignal): string {
  const dimension = stringValue(signal.tags.dimension);
  const metricFamily = stringValue(signal.tags.metric_family);
  return [
    signal.kind,
    metricFamily || canonicalMetricName(signal.name),
    dimension,
    signal.unit,
    signal.aggregation,
    signal.scope_type,
    canonicalSignalScope(signal),
  ].join(":");
}

function aggregateSignalValues(
  kind: GoldenSignalKind,
  aggregation: GoldenSignalAggregation,
  signals: GoldenSignal[],
): number {
  const sourceAwareValue = aggregateSourceAwareEquivalentSignals(signals);
  if (sourceAwareValue !== null) return sourceAwareValue;
  const values = signals.map((signal) => signal.value);
  const finite = values.filter((value) => Number.isFinite(value));
  if (finite.length === 0) return 0;
  if (aggregation === "min") return minNumber(finite);
  if (aggregation === "count" || aggregation === "sum") {
    return kind === "latency" || kind === "saturation" ? maxNumber(finite) : sum(finite);
  }
  if (aggregation === "avg" && kind === "traffic") {
    return sum(finite) / finite.length;
  }
  return maxNumber(finite);
}

function aggregateSourceAwareEquivalentSignals(signals: GoldenSignal[]): number | null {
  if (signals.length === 0) return null;
  const representative = signals[0];
  if (
    representative.scope_type !== "edge" ||
    stringValue(representative.tags.dedupe_policy) !== "equivalent_edge_max"
  ) {
    return null;
  }
  const totalsByAnalyzer = new Map<string, number>();
  for (const signal of signals) {
    totalsByAnalyzer.set(signal.source_analyzer, (totalsByAnalyzer.get(signal.source_analyzer) ?? 0) + signal.value);
  }
  return maxNumber(Array.from(totalsByAnalyzer.values()));
}

function compareSliMetrics(left: SliMetric, right: SliMetric): number {
  const kindDelta = kindRank(left.kind) - kindRank(right.kind);
  if (kindDelta !== 0) return kindDelta;
  if (left.scope_type !== right.scope_type) return left.scope_type.localeCompare(right.scope_type);
  if (left.scope !== right.scope) return left.scope.localeCompare(right.scope);
  if (left.name !== right.name) return left.name.localeCompare(right.name);
  return right.value - left.value;
}

function countMetricsByKind(metrics: SliMetric[]): Record<GoldenSignalKind, number> {
  const counts: Record<GoldenSignalKind, number> = {
    latency: 0,
    traffic: 0,
    errors: 0,
    saturation: 0,
  };
  for (const metric of metrics) counts[metric.kind] += 1;
  return counts;
}

function countMetricsByScope(metrics: SliMetric[]): Record<GoldenSignalScopeType, number> {
  const counts: Record<GoldenSignalScopeType, number> = {
    global: 0,
    service: 0,
    endpoint: 0,
    edge: 0,
    runtime: 0,
    thread: 0,
    trace: 0,
    jvm: 0,
  };
  for (const metric of metrics) counts[metric.scope_type] += 1;
  return counts;
}

function mergeTags(tags: GoldenSignalTags[]): GoldenSignalTags {
  const merged: GoldenSignalTags = {};
  for (const tagSet of tags) {
    for (const [key, value] of Object.entries(tagSet)) {
      if (merged[key] === undefined || merged[key] === value) {
        merged[key] = value;
      }
    }
  }
  return merged;
}

function canonicalMetricName(value: string): string {
  return value.trim().toLowerCase().replace(/[^a-z0-9]+/g, "_").replace(/^_+|_+$/g, "");
}

function canonicalScope(value: string): string {
  return value.trim().toLowerCase().replace(/\s+/g, " ");
}

function canonicalSignalScope(signal: GoldenSignal): string {
  if (signal.scope_type === "edge") {
    const [caller, callee] = signal.scope.split(/\s*->\s*/, 2);
    if (caller && callee) {
      return `${canonicalServiceIdentity(caller)} -> ${canonicalServiceIdentity(callee)}`;
    }
  }
  if (signal.scope_type === "service") return canonicalServiceIdentity(signal.scope);
  return canonicalScope(signal.scope);
}

function uniqueSorted(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((left, right) => left.localeCompare(right));
}

function sum(values: number[]): number {
  return values.reduce((total, value) => total + value, 0);
}

function maxNumber(values: Iterable<number>): number {
  let max = Number.NEGATIVE_INFINITY;
  for (const value of values) {
    if (value > max) max = value;
  }
  return max;
}

function minNumber(values: Iterable<number>): number {
  let min = Number.POSITIVE_INFINITY;
  for (const value of values) {
    if (value < min) min = value;
  }
  return min;
}

function matchesTarget(metric: SliMetric, target: SloTarget): boolean {
  const match = target.match;
  if (match.kind && metric.kind !== match.kind) return false;
  if (match.units && !match.units.includes(metric.unit)) return false;
  if (match.aggregations && !match.aggregations.includes(metric.aggregation)) return false;
  if (match.scopeTypes && !match.scopeTypes.includes(metric.scope_type)) return false;
  if (match.sourceAnalyzers && !metric.source_analyzers.some((source) => match.sourceAnalyzers?.includes(source))) {
    return false;
  }
  const searchableName = `${metric.name} ${metric.metric_key}`.toLowerCase();
  if (match.nameIncludes && !match.nameIncludes.every((term) => searchableName.includes(term.toLowerCase()))) {
    return false;
  }
  const searchableEvidence = metric.evidence_refs.join(" ").toLowerCase();
  if (
    match.evidenceIncludes &&
    !match.evidenceIncludes.every((term) => searchableEvidence.includes(term.toLowerCase()))
  ) {
    return false;
  }
  if (match.tagEquals) {
    for (const [key, value] of Object.entries(match.tagEquals)) {
      if (String(metric.tags[key]) !== String(value)) return false;
    }
  }
  return metric.unit === target.unit;
}

function comparisonViolates(actual: number, comparator: SloComparator, threshold: number): boolean {
  return comparator === "lte" ? actual > threshold : actual < threshold;
}

function violationDelta(actual: number, comparator: SloComparator, threshold: number): number {
  return comparator === "lte" ? Math.max(0, actual - threshold) : Math.max(0, threshold - actual);
}

function ratioToThreshold(actual: number, comparator: SloComparator, threshold: number): number {
  if (threshold === 0) return actual === 0 ? 1 : Number.POSITIVE_INFINITY;
  return comparator === "lte" ? actual / threshold : threshold / Math.max(actual, 0.000001);
}

function computeErrorBudget(actual: number, target: SloTarget): {
  consumedPercent: number;
  remainingPercent: number;
  burnRate: number;
} {
  const consumedPercent =
    target.comparator === "lte"
      ? consumedForUpperBound(actual, target)
      : consumedForLowerBound(actual, target);
  return {
    consumedPercent,
    remainingPercent: Math.max(0, 100 - consumedPercent),
    burnRate: consumedPercent / 100,
  };
}

function consumedForUpperBound(actual: number, target: SloTarget): number {
  const allowedBudget = upperBoundAllowedBudget(target);
  if (allowedBudget === 0) return actual > 0 ? 100 : 0;
  return Math.max(0, (actual / allowedBudget) * 100);
}

function consumedForLowerBound(actual: number, target: SloTarget): number {
  const budget = lowerBoundAllowedBudget(target);
  const shortfall = Math.max(0, target.threshold - actual);
  return Math.max(0, (shortfall / budget) * 100);
}

function upperBoundAllowedBudget(target: SloTarget): number {
  if (target.unit === "percent" && target.match.kind === "errors") {
    return Math.max(0, target.threshold);
  }
  return target.threshold;
}

function lowerBoundAllowedBudget(target: SloTarget): number {
  if (target.unit === "percent") {
    const objective = target.objective_percent ?? target.threshold;
    return Math.max(0.000001, 100 - objective);
  }
  return Math.max(0.000001, target.threshold);
}

function compareViolations(left: SloViolation, right: SloViolation): number {
  return (
    severityRankSlo(right.severity) - severityRankSlo(left.severity) ||
    right.burn_rate - left.burn_rate ||
    right.delta - left.delta ||
    left.affected_scope.localeCompare(right.affected_scope)
  );
}

function countViolationsBySeverity(violations: SloViolation[]): Record<SloSeverity, number> {
  const counts: Record<SloSeverity, number> = {
    critical: 0,
    warning: 0,
    info: 0,
  };
  for (const violation of violations) counts[violation.severity] += 1;
  return counts;
}

function severityRankSlo(severity: SloSeverity): number {
  switch (severity) {
    case "critical":
      return 3;
    case "warning":
      return 2;
    default:
      return 1;
  }
}

function round(value: number, digits: number): number {
  if (!Number.isFinite(value)) return value;
  const factor = 10 ** digits;
  return Math.round(value * factor) / factor;
}

function endpointScope(row: Record<string, unknown>): string {
  const method = stringValue(row.method);
  const uri = stringValue(row.uri) || stringValue(row.path) || "unknown-endpoint";
  return method ? `${method} ${uri}` : uri;
}

function edgeScope(row: Record<string, unknown>, callerKey: string, calleeKey: string): string {
  const caller = stringValue(row[callerKey]) || "unknown-caller";
  const callee = stringValue(row[calleeKey]) || "unknown-callee";
  return `${caller} -> ${callee}`;
}

function jenniferEdgeScope(row: Record<string, unknown>): string {
  const caller = stringValue(row.caller_application) || stringValue(row.caller_txid) || "unknown-caller";
  const callee =
    stringValue(row.callee_application) ||
    stringValue(row.external_call_target) ||
    stringValue(row.external_call_url) ||
    "external-service";
  return `${caller} -> ${callee}`;
}

function serviceCallScope(row: Record<string, unknown>): string {
  const caller = stringValue(row.caller_application) || stringValue(row.caller) || "unknown-caller";
  const callee = stringValue(row.callee_application) || stringValue(row.callee) || "unknown-callee";
  return `${caller} -> ${callee}`;
}

function defaultScope(scopeType: GoldenSignalScopeType | undefined): string {
  switch (scopeType) {
    case "service":
      return "all-services";
    case "endpoint":
      return "all-endpoints";
    case "edge":
      return "all-edges";
    case "runtime":
      return "runtime";
    case "thread":
      return "threads";
    case "trace":
      return "traces";
    case "jvm":
      return "jvm";
    default:
      return "global";
  }
}

function humanizeKey(value: string): string {
  return value
    .split("_")
    .filter(Boolean)
    .map((part) => part.slice(0, 1).toUpperCase() + part.slice(1))
    .join(" ");
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

function finiteNumber(value: unknown): number | null {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : null;
  }
  return null;
}

function hashString(value: string): string {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0).toString(16);
}
