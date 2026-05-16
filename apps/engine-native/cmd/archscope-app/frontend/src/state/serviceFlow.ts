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

export type ServiceEdge = {
  id: string;
  caller: string;
  callee: string;
  call_count: number;
  total_latency_ms?: number;
  avg_latency_ms?: number;
  max_latency_ms?: number;
  error_count: number;
  error_rate: number;
  network_gap_sample_count: number;
  total_network_gap_ms?: number;
  avg_network_gap_ms?: number;
  max_network_gap_ms?: number;
  matched_call_count: number;
  unmatched_call_count: number;
  source_types: ServiceFlowSourceType[];
  source_analyzers: string[];
  source_result_ids: string[];
  source_files: string[];
  evidence_refs: string[];
  input_edge_ids: string[];
};

export type ServiceEdgeModel = {
  generated_at: string;
  source_count: number;
  input_edge_count: number;
  edge_count: number;
  total_call_count: number;
  total_error_count: number;
  total_unmatched_call_count: number;
  edges: ServiceEdge[];
};

export type ServiceFlowFindingSeverity = "critical" | "warning" | "info";

export type ServiceFlowFinding = {
  id: string;
  severity: ServiceFlowFindingSeverity;
  code: string;
  message: string;
  caller?: string;
  callee?: string;
  edge_id?: string;
  source_analyzers: string[];
  source_result_ids: string[];
  evidence_refs: string[];
  payload: Record<string, unknown>;
};

export type ServiceFlowAnalysis = {
  generated_at: string;
  input_model: ServiceFlowInputModel;
  edge_model: ServiceEdgeModel;
  findings: ServiceFlowFinding[];
  findings_by_severity: Record<ServiceFlowFindingSeverity, number>;
};

export type ServiceFlowExportPayload = {
  type: "archscope_service_flow";
  schema_version: "0.1.0";
  generated_at: string;
  summary: {
    source_count: number;
    input_edge_count: number;
    edge_count: number;
    total_call_count: number;
    total_error_count: number;
    total_unmatched_call_count: number;
    finding_count: number;
  };
  edges: ServiceEdge[];
  findings: ServiceFlowFinding[];
};

const MAX_INPUT_EDGES = 1_500;
const MAX_SEQUENCE_EDGES = 80;
const NETWORK_GAP_WARNING_MS = 20;
const NETWORK_GAP_CRITICAL_MS = 50;

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

export function buildServiceEdgeModel(input: ServiceFlowInputModel | ServiceFlowInputEdge[]): ServiceEdgeModel {
  const edges = Array.isArray(input) ? input : input.edges;
  const serviceEdges = Array.from(groupServiceEdges(edges).values()).sort(compareServiceEdges);
  return {
    generated_at: new Date().toISOString(),
    source_count: Array.isArray(input) ? uniqueSorted(edges.map((edge) => edge.source_result_id)).length : input.source_count,
    input_edge_count: edges.length,
    edge_count: serviceEdges.length,
    total_call_count: sum(serviceEdges.map((edge) => edge.call_count)),
    total_error_count: sum(serviceEdges.map((edge) => edge.error_count)),
    total_unmatched_call_count: sum(serviceEdges.map((edge) => edge.unmatched_call_count)),
    edges: serviceEdges,
  };
}

export function buildServiceEdgeModelFromEntries(entries: AnalysisWorkspaceEntry[]): ServiceEdgeModel {
  return buildServiceEdgeModel(buildServiceFlowInputModel(entries));
}

export function buildServiceFlowAnalysis(entries: AnalysisWorkspaceEntry[]): ServiceFlowAnalysis {
  const inputModel = buildServiceFlowInputModel(entries);
  const edgeModel = buildServiceEdgeModel(inputModel);
  const findings = buildServiceFlowFindings(edgeModel, entries);
  return {
    generated_at: new Date().toISOString(),
    input_model: inputModel,
    edge_model: edgeModel,
    findings,
    findings_by_severity: countFindingsBySeverity(findings),
  };
}

export function buildServiceFlowFindings(
  edgeModel: ServiceEdgeModel,
  entries: AnalysisWorkspaceEntry[] = [],
): ServiceFlowFinding[] {
  return [
    ...edgeModel.edges.flatMap((edge) => edgeFindings(edge)),
    ...missingParentFindings(entries),
  ].sort(compareFindings);
}

export function buildServiceFlowExportPayload(analysis: ServiceFlowAnalysis): ServiceFlowExportPayload {
  return {
    type: "archscope_service_flow",
    schema_version: "0.1.0",
    generated_at: new Date().toISOString(),
    summary: {
      source_count: analysis.edge_model.source_count,
      input_edge_count: analysis.edge_model.input_edge_count,
      edge_count: analysis.edge_model.edge_count,
      total_call_count: analysis.edge_model.total_call_count,
      total_error_count: analysis.edge_model.total_error_count,
      total_unmatched_call_count: analysis.edge_model.total_unmatched_call_count,
      finding_count: analysis.findings.length,
    },
    edges: analysis.edge_model.edges,
    findings: analysis.findings,
  };
}

export function buildServiceFlowMermaidSequence(analysis: ServiceFlowAnalysis): string {
  const edges = analysis.edge_model.edges.slice(0, MAX_SEQUENCE_EDGES);
  const services = uniqueSorted(edges.flatMap((edge) => [edge.caller, edge.callee]));
  const aliases = new Map(services.map((service, index) => [service, `S${index + 1}`]));
  const lines = [
    "sequenceDiagram",
    "    autonumber",
    ...services.map((service) => `    participant ${aliases.get(service)} as ${mermaidText(service)}`),
  ];
  const findingsByEdge = groupFindingsByEdge(analysis.findings);
  for (const edge of edges) {
    const caller = aliases.get(edge.caller) ?? "UnknownCaller";
    const callee = aliases.get(edge.callee) ?? "UnknownCallee";
    lines.push(`    ${caller}->>${callee}: ${mermaidText(edgeMessage(edge))}`);
    for (const finding of findingsByEdge.get(edge.id) ?? []) {
      lines.push(`    Note over ${caller},${callee}: ${mermaidText(`${finding.code}: ${finding.message}`)}`);
    }
  }
  const sourceOnlyFindings = analysis.findings.filter((finding) => !finding.edge_id).slice(0, 20);
  for (const finding of sourceOnlyFindings) {
    lines.push(`    Note over ${services.length > 0 ? aliases.get(services[0]) : "S1"}: ${mermaidText(`${finding.code}: ${finding.message}`)}`);
  }
  return `${lines.join("\n")}\n`;
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

function groupServiceEdges(edges: ServiceFlowInputEdge[]): Map<string, ServiceEdge> {
  const buckets = new Map<string, ServiceFlowInputEdge[]>();
  for (const edge of edges) {
    const key = serviceEdgeKey(edge.caller, edge.callee);
    const bucket = buckets.get(key);
    if (bucket) {
      bucket.push(edge);
    } else {
      buckets.set(key, [edge]);
    }
  }

  const serviceEdges = new Map<string, ServiceEdge>();
  for (const [key, bucket] of buckets.entries()) {
    const representative = bucket[0];
    const callCount = sum(bucket.map((edge) => edge.call_count));
    const totalLatency = sumOptional(bucket.map((edge) => edge.total_latency_ms ?? derivedTotalLatency(edge)));
    const maxLatency = maxOptional(bucket.flatMap((edge) => [edge.max_latency_ms, edge.avg_latency_ms]));
    const errorCount = Math.round(sum(bucket.map((edge) => edge.error_count ?? 0)));
    const networkGapSamples = sum(
      bucket.map((edge) => (edge.network_gap_ms === undefined ? 0 : Math.max(1, edge.call_count))),
    );
    const totalNetworkGap = sumOptional(
      bucket.map((edge) =>
        edge.network_gap_ms === undefined ? undefined : edge.network_gap_ms * Math.max(1, edge.call_count),
      ),
    );
    serviceEdges.set(key, {
      id: `svc-edge-${hashString(key)}`,
      caller: representative.caller,
      callee: representative.callee,
      call_count: callCount,
      total_latency_ms: roundOptional(totalLatency, 3),
      avg_latency_ms: roundOptional(
        totalLatency === undefined ? avgOptional(bucket.map((edge) => edge.avg_latency_ms)) : totalLatency / callCount,
        3,
      ),
      max_latency_ms: roundOptional(maxLatency, 3),
      error_count: errorCount,
      error_rate: callCount > 0 ? round((errorCount / callCount) * 100, 3) : 0,
      network_gap_sample_count: networkGapSamples,
      total_network_gap_ms: roundOptional(totalNetworkGap, 3),
      avg_network_gap_ms: roundOptional(
        totalNetworkGap === undefined || networkGapSamples <= 0 ? undefined : totalNetworkGap / networkGapSamples,
        3,
      ),
      max_network_gap_ms: roundOptional(maxOptional(bucket.map((edge) => edge.network_gap_ms)), 3),
      matched_call_count: sum(bucket.map((edge) => matchedCallCount(edge))),
      unmatched_call_count: sum(bucket.map((edge) => unmatchedCallCount(edge))),
      source_types: uniqueSorted(bucket.map((edge) => edge.source_type)) as ServiceFlowSourceType[],
      source_analyzers: uniqueSorted(bucket.map((edge) => edge.source_analyzer)),
      source_result_ids: uniqueSorted(bucket.map((edge) => edge.source_result_id)),
      source_files: uniqueSorted(bucket.map((edge) => edge.source_file ?? "")),
      evidence_refs: uniqueSorted(bucket.map((edge) => edge.evidence_ref)),
      input_edge_ids: uniqueSorted(bucket.map((edge) => edge.id)),
    });
  }
  return serviceEdges;
}

function serviceEdgeKey(caller: string, callee: string): string {
  return `${canonicalServiceName(caller)}\x00${canonicalServiceName(callee)}`;
}

function canonicalServiceName(value: string): string {
  return value.trim().toLowerCase().replace(/\s+/g, " ");
}

function derivedTotalLatency(edge: ServiceFlowInputEdge): number | undefined {
  return edge.avg_latency_ms === undefined ? undefined : edge.avg_latency_ms * Math.max(1, edge.call_count);
}

function matchedCallCount(edge: ServiceFlowInputEdge): number {
  if (edge.source_type !== "jennifer_msa_edge") return 0;
  return edge.match_status?.toLowerCase() === "matched" ? edge.call_count : 0;
}

function unmatchedCallCount(edge: ServiceFlowInputEdge): number {
  if (edge.source_type === "jennifer_unprofiled_external_call_group") return edge.call_count;
  if (edge.source_type !== "jennifer_msa_edge") return 0;
  return edge.match_status?.toLowerCase() === "matched" ? 0 : edge.call_count;
}

function compareServiceEdges(left: ServiceEdge, right: ServiceEdge): number {
  return (
    right.unmatched_call_count - left.unmatched_call_count ||
    right.error_count - left.error_count ||
    (right.max_network_gap_ms ?? 0) - (left.max_network_gap_ms ?? 0) ||
    right.call_count - left.call_count ||
    left.caller.localeCompare(right.caller) ||
    left.callee.localeCompare(right.callee)
  );
}

function edgeFindings(edge: ServiceEdge): ServiceFlowFinding[] {
  const findings: ServiceFlowFinding[] = [];
  if (edge.unmatched_call_count > 0) {
    findings.push({
      id: `svc-finding-${hashString(["unmatched", edge.id, String(edge.unmatched_call_count)].join("\x00"))}`,
      severity: "warning",
      code: "SERVICE_FLOW_UNMATCHED_CALLS",
      message: `${edge.unmatched_call_count.toLocaleString()} calls from ${edge.caller} to ${edge.callee} were unmatched or unprofiled.`,
      caller: edge.caller,
      callee: edge.callee,
      edge_id: edge.id,
      source_analyzers: edge.source_analyzers,
      source_result_ids: edge.source_result_ids,
      evidence_refs: edge.evidence_refs,
      payload: {
        caller: edge.caller,
        callee: edge.callee,
        unmatched_call_count: edge.unmatched_call_count,
        call_count: edge.call_count,
      },
    });
  }

  const networkGap = edge.avg_network_gap_ms ?? edge.max_network_gap_ms;
  if (networkGap !== undefined && networkGap >= NETWORK_GAP_WARNING_MS) {
    const severity: ServiceFlowFindingSeverity =
      networkGap >= NETWORK_GAP_CRITICAL_MS ? "critical" : "warning";
    findings.push({
      id: `svc-finding-${hashString(["network-gap", edge.id, String(networkGap)].join("\x00"))}`,
      severity,
      code: "SERVICE_FLOW_HIGH_NETWORK_GAP",
      message: `${edge.caller} -> ${edge.callee} network gap is ${formatMs(networkGap)}.`,
      caller: edge.caller,
      callee: edge.callee,
      edge_id: edge.id,
      source_analyzers: edge.source_analyzers,
      source_result_ids: edge.source_result_ids,
      evidence_refs: edge.evidence_refs,
      payload: {
        caller: edge.caller,
        callee: edge.callee,
        avg_network_gap_ms: edge.avg_network_gap_ms,
        max_network_gap_ms: edge.max_network_gap_ms,
        warning_threshold_ms: NETWORK_GAP_WARNING_MS,
        critical_threshold_ms: NETWORK_GAP_CRITICAL_MS,
      },
    });
  }
  return findings;
}

function missingParentFindings(entries: AnalysisWorkspaceEntry[]): ServiceFlowFinding[] {
  const findings: ServiceFlowFinding[] = [];
  for (const entry of entries) {
    if ((entry.result.type || entry.result_type) !== "trace_import") continue;
    const spans = arrayOfObjects(entry.result.tables?.spans);
    const spanIDs = new Set(spans.map((span) => stringValue(span.span_id)).filter(Boolean));
    const missing = spans.filter((span) => {
      const parent = stringValue(span.parent_span_id);
      return parent && !spanIDs.has(parent);
    });
    if (missing.length > 0) {
      for (const [callee, rows] of groupRowsByCallee(missing).entries()) {
        findings.push({
          id: `svc-finding-${hashString(["missing-parent", entry.id, callee, String(rows.length)].join("\x00"))}`,
          severity: "warning",
          code: "SERVICE_FLOW_MISSING_PARENT",
          message: `${rows.length.toLocaleString()} spans in ${callee} reference parents missing from the trace export.`,
          caller: "missing-parent",
          callee,
          source_analyzers: [entry.result.type || entry.result_type],
          source_result_ids: [entry.id],
          evidence_refs: ["tables.spans"],
          payload: {
            callee,
            missing_parent_spans: rows.length,
            sample_span_ids: rows.slice(0, 5).map((row) => stringValue(row.span_id)).filter(Boolean),
            source_title: entry.title,
          },
        });
      }
      continue;
    }

    const summaryMissing = optionalNumber(entry.result.summary?.missing_parent_spans);
    if (summaryMissing !== undefined && summaryMissing > 0) {
      findings.push({
        id: `svc-finding-${hashString(["missing-parent-summary", entry.id, String(summaryMissing)].join("\x00"))}`,
        severity: "warning",
        code: "SERVICE_FLOW_MISSING_PARENT",
        message: `${summaryMissing.toLocaleString()} spans reference parents missing from the trace export.`,
        caller: "missing-parent",
        callee: "unknown-service",
        source_analyzers: [entry.result.type || entry.result_type],
        source_result_ids: [entry.id],
        evidence_refs: ["summary.missing_parent_spans"],
        payload: {
          missing_parent_spans: summaryMissing,
          source_title: entry.title,
        },
      });
    }
  }
  return findings;
}

function groupRowsByCallee(rows: Record<string, unknown>[]): Map<string, Record<string, unknown>[]> {
  const groups = new Map<string, Record<string, unknown>[]>();
  for (const row of rows) {
    const callee = normalizeServiceName(
      stringValue(row.service_name) || stringValue(row.remote_service) || "unknown-service",
    );
    const group = groups.get(callee);
    if (group) {
      group.push(row);
    } else {
      groups.set(callee, [row]);
    }
  }
  return groups;
}

function countFindingsBySeverity(findings: ServiceFlowFinding[]): Record<ServiceFlowFindingSeverity, number> {
  const counts: Record<ServiceFlowFindingSeverity, number> = {
    critical: 0,
    warning: 0,
    info: 0,
  };
  for (const finding of findings) counts[finding.severity] += 1;
  return counts;
}

function compareFindings(left: ServiceFlowFinding, right: ServiceFlowFinding): number {
  return (
    severityRank(right.severity) - severityRank(left.severity) ||
    left.code.localeCompare(right.code) ||
    (left.caller ?? "").localeCompare(right.caller ?? "") ||
    (left.callee ?? "").localeCompare(right.callee ?? "")
  );
}

function severityRank(severity: ServiceFlowFindingSeverity): number {
  switch (severity) {
    case "critical":
      return 3;
    case "warning":
      return 2;
    default:
      return 1;
  }
}

function formatMs(value: number): string {
  return `${round(value, 2).toLocaleString()} ms`;
}

function groupFindingsByEdge(findings: ServiceFlowFinding[]): Map<string, ServiceFlowFinding[]> {
  const groups = new Map<string, ServiceFlowFinding[]>();
  for (const finding of findings) {
    if (!finding.edge_id) continue;
    const group = groups.get(finding.edge_id);
    if (group) {
      group.push(finding);
    } else {
      groups.set(finding.edge_id, [finding]);
    }
  }
  return groups;
}

function edgeMessage(edge: ServiceEdge): string {
  const parts = [`${edge.call_count.toLocaleString()} calls`];
  if (edge.avg_latency_ms !== undefined) parts.push(`avg ${formatMs(edge.avg_latency_ms)}`);
  if (edge.error_count > 0) parts.push(`${edge.error_count.toLocaleString()} errors`);
  if (edge.avg_network_gap_ms !== undefined) parts.push(`gap ${formatMs(edge.avg_network_gap_ms)}`);
  if (edge.unmatched_call_count > 0) parts.push(`${edge.unmatched_call_count.toLocaleString()} unmatched`);
  return parts.join(", ");
}

function mermaidText(value: string): string {
  return JSON.stringify(value.replace(/\r?\n/g, " ").slice(0, 180));
}

function sum(values: number[]): number {
  return values.reduce((total, value) => total + value, 0);
}

function sumOptional(values: Array<number | undefined>): number | undefined {
  const present = values.filter((value): value is number => value !== undefined && Number.isFinite(value));
  return present.length > 0 ? sum(present) : undefined;
}

function avgOptional(values: Array<number | undefined>): number | undefined {
  const present = values.filter((value): value is number => value !== undefined && Number.isFinite(value));
  return present.length > 0 ? sum(present) / present.length : undefined;
}

function maxOptional(values: Array<number | undefined>): number | undefined {
  const present = values.filter((value): value is number => value !== undefined && Number.isFinite(value));
  return present.length > 0 ? Math.max(...present) : undefined;
}

function roundOptional(value: number | undefined, digits: number): number | undefined {
  return value === undefined ? undefined : round(value, digits);
}

function round(value: number, digits: number): number {
  if (!Number.isFinite(value)) return value;
  const factor = 10 ** digits;
  return Math.round(value * factor) / factor;
}

function uniqueSorted(values: string[]): string[] {
  return Array.from(new Set(values.filter(Boolean))).sort((left, right) => left.localeCompare(right));
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
