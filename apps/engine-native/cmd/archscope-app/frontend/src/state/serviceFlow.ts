import type { AnalysisWorkspaceEntry } from "@/state/analysisWorkspace";

export type ServiceFlowSourceType =
  | "trace_import_dependency"
  | "jennifer_msa_edge"
  | "jennifer_unprofiled_external_call_group";

export type ServiceFlowInputEdge = {
  id: string;
  source_type: ServiceFlowSourceType;
  source_analyzer: string;
  source_result_id: string;
  source_title: string;
  source_file?: string;
  caller: string;
  callee: string;
  call_count: number;
  total_latency_ms?: number;
  avg_latency_ms?: number;
  max_latency_ms?: number;
  error_count?: number;
  error_rate?: number;
  raw_network_gap_ms?: number;
  adjusted_network_gap_ms?: number;
  network_gap_ms?: number;
  match_status?: string;
  guid?: string;
  trace_id?: string;
  external_call_url?: string;
  evidence_ref: string;
  payload: Record<string, unknown>;
};

export type ServiceFlowInputModel = {
  generated_at: string;
  source_count: number;
  input_edge_count: number;
  by_source_type: Record<ServiceFlowSourceType, number>;
  edges: ServiceFlowInputEdge[];
};

const MAX_INPUT_EDGES = 1_500;

export function buildServiceFlowInputModel(entries: AnalysisWorkspaceEntry[]): ServiceFlowInputModel {
  const edges = entries.flatMap((entry) => inputEdgesFromEntry(entry)).slice(0, MAX_INPUT_EDGES);
  return {
    generated_at: new Date().toISOString(),
    source_count: entries.length,
    input_edge_count: edges.length,
    by_source_type: countBySourceType(edges),
    edges,
  };
}

export function buildServiceFlowInputEdges(entries: AnalysisWorkspaceEntry[]): ServiceFlowInputEdge[] {
  return buildServiceFlowInputModel(entries).edges;
}

function inputEdgesFromEntry(entry: AnalysisWorkspaceEntry): ServiceFlowInputEdge[] {
  const type = entry.result.type || entry.result_type;
  if (type === "trace_import") return traceImportEdges(entry);
  if (type === "jennifer_profile" || Array.isArray(entry.result.tables?.msa_edges)) {
    return jenniferEdges(entry);
  }
  return [];
}

function traceImportEdges(entry: AnalysisWorkspaceEntry): ServiceFlowInputEdge[] {
  return arrayOfObjects(entry.result.tables?.service_dependencies).map((row, index) => {
    const caller = normalizeServiceName(stringValue(row.caller) || "unknown-caller");
    const callee = normalizeServiceName(stringValue(row.callee) || "unknown-callee");
    return makeInputEdge(entry, {
      sourceType: "trace_import_dependency",
      caller,
      callee,
      callCount: numberValue(row.call_count, 1),
      totalLatencyMs: optionalNumber(row.total_duration_ms),
      avgLatencyMs: optionalNumber(row.avg_duration_ms),
      errorCount: optionalNumber(row.error_count),
      errorRate: optionalNumber(row.error_rate),
      evidenceRef: "tables.service_dependencies",
      payload: row,
      idSuffix: `trace-dependency-${caller}-${callee}-${index}`,
    });
  });
}

function jenniferEdges(entry: AnalysisWorkspaceEntry): ServiceFlowInputEdge[] {
  const edges = arrayOfObjects(entry.result.tables?.msa_edges).map((row, index) => {
    const caller = normalizeServiceName(
      stringValue(row.caller_application) || stringValue(row.caller_txid) || "unknown-caller",
    );
    const callee = normalizeServiceName(
      stringValue(row.callee_application) ||
        stringValue(row.external_call_target) ||
        normalizedExternalTarget(row.external_call_url) ||
        "external-service",
    );
    const elapsed = optionalNumber(row.external_call_elapsed_ms);
    const rawGap = optionalNumber(row.raw_network_gap_ms);
    const adjustedGap = optionalNumber(row.adjusted_network_gap_ms);
    return makeInputEdge(entry, {
      sourceType: "jennifer_msa_edge",
      caller,
      callee,
      callCount: 1,
      totalLatencyMs: elapsed,
      avgLatencyMs: elapsed,
      maxLatencyMs: elapsed,
      rawNetworkGapMs: rawGap,
      adjustedNetworkGapMs: adjustedGap,
      networkGapMs: adjustedGap ?? rawGap,
      matchStatus: stringValue(row.match_status) || "unknown",
      guid: stringValue(row.guid),
      externalCallUrl: stringValue(row.external_call_url),
      evidenceRef: "tables.msa_edges",
      payload: row,
      idSuffix: `jennifer-msa-edge-${caller}-${callee}-${stringValue(row.guid)}-${index}`,
    });
  });

  const unprofiled = arrayOfObjects(entry.result.tables?.unprofiled_external_call_groups).map((row, index) => {
    const caller = normalizeServiceName(stringValue(row.caller_application) || "unknown-caller");
    const callee = normalizeServiceName(stringValue(row.target) || "external-service");
    return makeInputEdge(entry, {
      sourceType: "jennifer_unprofiled_external_call_group",
      caller,
      callee,
      callCount: numberValue(row.count, 1),
      totalLatencyMs: optionalNumber(row.total_elapsed_ms),
      avgLatencyMs: optionalNumber(row.avg_elapsed_ms),
      maxLatencyMs: optionalNumber(row.max_elapsed_ms),
      matchStatus: stringValue(row.match_status) || "unprofiled",
      guid: stringValue(row.guid),
      externalCallUrl: firstString(row.external_call_urls),
      evidenceRef: "tables.unprofiled_external_call_groups",
      payload: row,
      idSuffix: `jennifer-unprofiled-${caller}-${callee}-${stringValue(row.guid)}-${index}`,
    });
  });

  return [...edges, ...unprofiled];
}

function makeInputEdge(
  entry: AnalysisWorkspaceEntry,
  input: {
    sourceType: ServiceFlowSourceType;
    caller: string;
    callee: string;
    callCount: number;
    totalLatencyMs?: number;
    avgLatencyMs?: number;
    maxLatencyMs?: number;
    errorCount?: number;
    errorRate?: number;
    rawNetworkGapMs?: number;
    adjustedNetworkGapMs?: number;
    networkGapMs?: number;
    matchStatus?: string;
    guid?: string;
    traceID?: string;
    externalCallUrl?: string;
    evidenceRef: string;
    payload: Record<string, unknown>;
    idSuffix: string;
  },
): ServiceFlowInputEdge {
  return withoutUndefined({
    id: `sfi-${hashString([entry.id, input.sourceType, input.idSuffix].join("\x00"))}`,
    source_type: input.sourceType,
    source_analyzer: entry.result.type || entry.result_type,
    source_result_id: entry.id,
    source_title: entry.title,
    source_file: entry.source_files[0],
    caller: input.caller,
    callee: input.callee,
    call_count: Math.max(1, Math.round(input.callCount)),
    total_latency_ms: input.totalLatencyMs,
    avg_latency_ms: input.avgLatencyMs,
    max_latency_ms: input.maxLatencyMs,
    error_count: input.errorCount,
    error_rate: input.errorRate,
    raw_network_gap_ms: input.rawNetworkGapMs,
    adjusted_network_gap_ms: input.adjustedNetworkGapMs,
    network_gap_ms: input.networkGapMs,
    match_status: input.matchStatus,
    guid: input.guid,
    trace_id: input.traceID,
    external_call_url: input.externalCallUrl,
    evidence_ref: input.evidenceRef,
    payload: input.payload,
  });
}

function countBySourceType(edges: ServiceFlowInputEdge[]): Record<ServiceFlowSourceType, number> {
  const counts: Record<ServiceFlowSourceType, number> = {
    trace_import_dependency: 0,
    jennifer_msa_edge: 0,
    jennifer_unprofiled_external_call_group: 0,
  };
  for (const edge of edges) counts[edge.source_type] += 1;
  return counts;
}

function normalizedExternalTarget(value: unknown): string {
  const text = stringValue(value).trim();
  if (!text) return "";
  return text.split(/[?#]/)[0] || text;
}

function normalizeServiceName(value: string): string {
  const trimmed = value.trim();
  return trimmed || "unknown-service";
}

function firstString(value: unknown): string {
  if (Array.isArray(value)) {
    for (const item of value) {
      const text = stringValue(item);
      if (text) return text;
    }
  }
  return stringValue(value);
}

function arrayOfObjects(value: unknown): Record<string, unknown>[] {
  return Array.isArray(value)
    ? value.filter((item): item is Record<string, unknown> => isPlainObject(item))
    : [];
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function stringValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return "";
}

function numberValue(value: unknown, fallback: number): number {
  const parsed = optionalNumber(value);
  return parsed ?? fallback;
}

function optionalNumber(value: unknown): number | undefined {
  if (typeof value === "number" && Number.isFinite(value)) return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : undefined;
  }
  return undefined;
}

function withoutUndefined<T extends Record<string, unknown>>(value: T): T {
  return Object.fromEntries(
    Object.entries(value).filter(([, entryValue]) => entryValue !== undefined && entryValue !== ""),
  ) as T;
}

function hashString(value: string): string {
  let hash = 2166136261;
  for (let index = 0; index < value.length; index += 1) {
    hash ^= value.charCodeAt(index);
    hash = Math.imul(hash, 16777619);
  }
  return (hash >>> 0).toString(16);
}
