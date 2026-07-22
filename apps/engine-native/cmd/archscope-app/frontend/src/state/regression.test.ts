import type { AnalysisWorkspaceEntry, WorkspaceAnalysisResult } from "./analysisWorkspace.js";
import { evaluateAiInterpretation, extractAiInterpretation } from "./aiInterpretation.js";
import {
  adaptProfileFlameNode,
  describeTimelineState,
  extractProfileDiagnostics,
  extractProfileFlamegraph,
  profileDiagnosticIssueCount,
  selectPartialResult,
} from "./browserCpuProfile.js";
import {
  browserAuditDiagnosticIssueCount,
  buildBrowserAuditEvidence,
  extractBrowserAuditDiagnostics,
  scorePctBand,
  scoreToPct,
  selectAuditRows,
  selectBrowserAuditContract,
  selectBrowserAuditFindings,
  selectCategoryScores,
  selectCoreMetrics,
  selectNetworkRequests,
  selectResourceDistribution,
  selectScoreProvenance,
} from "./browserAudit.js";
import {
  availableFidelities,
  availableMethods,
  availableMimeTypes,
  buildProcessTree,
  extractCaptureMeta,
  extractRedaction,
  filterTransactions,
  httpCaptureReducer,
  initialHttpCaptureState,
  isErrorTransaction,
  isFilterActive,
  projectSummary,
  statusClassOf,
  timelineWindow,
  timingBreakdown,
  transactionMime,
  emptyFilter,
} from "./httpCapture.js";
import {
  addWorkspaceResult,
  clearWorkspaceResults,
  getWorkspaceEntry,
} from "./analysisWorkspace.js";
import { buildIncidentTimelineEvents } from "./incidentTimeline.js";
import {
  buildServiceFlowAnalysis,
  buildServiceFlowMermaidSequence,
  type ServiceFlowAnalysis,
} from "./serviceFlow.js";
import {
  buildGoldenSignalInventory,
  buildSliMetricModel,
} from "./sloGoldenSignals.js";

function assert(condition: unknown, message: string): void {
  if (!condition) throw new Error(message);
}

function entry(id: string, type: string, result: Partial<WorkspaceAnalysisResult>): AnalysisWorkspaceEntry {
  return {
    id,
    title: `${type} fixture`,
    result_type: type,
    source_files: [`${type}.json`],
    created_at: "2026-05-16T00:00:00.000Z",
    recorded_at: "2026-05-16T00:00:00.000Z",
    summary_preview: [],
    result: {
      source_files: [`${type}.json`],
      created_at: "2026-05-16T00:00:00.000Z",
      summary: {},
      series: {},
      tables: {},
      charts: {},
      metadata: {},
      ...result,
      type,
    },
  };
}

const trace = entry("trace-1", "trace_import", {
  tables: {
    service_dependencies: [
      {
        caller: "OrderService",
        callee: "Payment_Service",
        call_count: 10,
        total_duration_ms: 100,
        avg_duration_ms: 10,
        error_count: 2,
        error_rate: 0.2,
      },
    ],
  },
});

const jennifer = entry("jennifer-1", "jennifer_profile", {
  tables: {
    msa_edges: [
      {
        caller_application: "order-service",
        callee_application: "payment service",
        external_call_elapsed_ms: 12,
        adjusted_network_gap_ms: 3,
        match_status: "matched",
        guid: "g-1",
      },
    ],
  },
  series: {
    service_call_network_summary: [
      {
        caller_application: "order-service",
        callee_application: "payment service",
        call_count: 10,
      },
    ],
  },
});

const serviceFlow = buildServiceFlowAnalysis([trace, jennifer]);
assert(serviceFlow.edge_model.edge_count === 1, "service aliases should group equivalent Trace/Jennifer edges");

const accessEdge = entry("access-edge-1", "access_log", {
  tables: {
    service_dependencies: [
      {
        caller: "edge-gateway",
        callee: "payment-service",
        call_count: 5,
        total_duration_ms: 250,
        avg_duration_ms: 50,
        max_duration_ms: 80,
        error_count: 1,
        error_rate: 0.2,
      },
    ],
  },
});
const accessServiceFlow = buildServiceFlowAnalysis([accessEdge]);
assert(accessServiceFlow.edge_model.edge_count === 1, "access edge dependencies should feed Service Flow");

const database = entry("db-1", "database_slow_query", {
  summary: { p95_query_ms: 1200, slow_query_count: 1, error_count: 1 },
  tables: {
    service_dependencies: [
      {
        caller: "application",
        callee: "database:shop",
        call_count: 2,
        total_duration_ms: 1800,
        avg_duration_ms: 900,
        error_count: 1,
        error_rate: 0.5,
      },
    ],
    queries: [
      {
        timestamp: "2026-05-16T10:00:00Z",
        fingerprint: "select * from orders where id = ?",
        duration_ms: 1200,
      },
    ],
  },
});
assert(buildServiceFlowAnalysis([database]).edge_model.edge_count === 1, "database dependencies should feed Service Flow");

const broker = entry("broker-1", "broker_log", {
  summary: { total_events: 2, queue_pressure_count: 1, replication_issue_count: 1 },
  tables: {
    service_dependencies: [{ caller: "application", callee: "broker:orders", call_count: 2, error_count: 0, error_rate: 0 }],
    events: [{ timestamp: "2026-05-16T10:00:00Z", severity: "WARN", event_type: "queue_pressure", message: "queue backlog" }],
  },
});
assert(buildServiceFlowAnalysis([broker]).edge_model.edge_count === 1, "broker dependencies should feed Service Flow");

const inventory = buildGoldenSignalInventory([trace, jennifer]);
const dependencyErrorRate = inventory.signals.find((signal) => signal.name === "Dependency error rate");
assert(dependencyErrorRate?.value === 20, "trace-import error_rate fractions should normalize to percent");
const trafficMetric = buildSliMetricModel(inventory).metrics.find((metric) =>
  metric.metric_key.includes("service_edge_traffic"),
);
assert(trafficMetric?.value === 10, "equivalent Trace/Jennifer edge traffic should not double count");
const accessInventory = buildGoldenSignalInventory([accessEdge]);
assert(
  accessInventory.signals.some((signal) => signal.name === "Access edge error rate" && signal.value === 20),
  "access edge error_rate fractions should normalize to percent",
);

const runtimeInventory = buildGoldenSignalInventory([
  entry("go-1", "go_panic", { summary: { total_records: 2, unique_signatures: 1 } }),
]);
assert(
  runtimeInventory.signals.some((signal) => signal.name === "Runtime stack record volume" && signal.value === 2),
  "runtime stack analyzers should emit Golden Signals",
);

const mermaid = buildServiceFlowMermaidSequence({
  generated_at: "2026-05-16T00:00:00.000Z",
  input_model: {
    generated_at: "2026-05-16T00:00:00.000Z",
    source_count: 0,
    input_edge_count: 0,
    by_source_type: {
      trace_import_dependency: 0,
      access_edge_dependency: 0,
      database_dependency: 0,
      broker_dependency: 0,
      stitched_dependency: 0,
      jennifer_msa_edge: 0,
      jennifer_unprofiled_external_call_group: 0,
    },
    edges: [],
  },
  edge_model: {
    generated_at: "2026-05-16T00:00:00.000Z",
    source_count: 0,
    input_edge_count: 0,
    edge_count: 0,
    total_call_count: 0,
    total_error_count: 0,
    total_unmatched_call_count: 0,
    edges: [],
  },
  findings: [
    {
      id: "f-1",
      severity: "warning",
      code: "SOURCE_ONLY",
      message: "No edge available",
      source_analyzers: ["trace_import"],
      source_result_ids: ["trace-1"],
      evidence_refs: ["summary.missing_parent_spans"],
      payload: {},
    },
  ],
  findings_by_severity: { critical: 0, warning: 1, info: 0 },
} satisfies ServiceFlowAnalysis);
assert(mermaid.includes("participant S1 as Findings"), "source-only Mermaid findings need a participant");

const timeline = buildIncidentTimelineEvents([
  entry("access-1", "access_log", {
    series: {
      error_rate_per_minute: [{ time: 1_715_731_200, value: 10, errors: 5, total: 10 }],
    },
  }),
]);
assert(timeline[0]?.timestamp === "2024-05-15T00:00:00.000Z", "Unix-second timestamps should parse");

const serverLog = entry("server-1", "server_log", {
  summary: { total_events: 2, error_count: 1, stuck_thread_count: 1 },
  tables: {
    events: [
      {
        timestamp: "2026-05-16T10:00:00Z",
        severity: "ERROR",
        event_type: "worker_error",
        message: "upstream connection failed",
        request_id: "req-1",
      },
    ],
  },
});
assert(
  buildIncidentTimelineEvents([serverLog]).some((event) => event.category === "worker_error"),
  "server-log events should feed Incident Timeline",
);
assert(
  buildGoldenSignalInventory([serverLog]).signals.some((signal) => signal.name === "Server log error count"),
  "server-log summaries should feed Golden Signals",
);

const metricsSnapshot = entry("metrics-1", "metrics_snapshot", {
  tables: {
    golden_signal_candidates: [
      { name: "http_request_duration_seconds", kind: "latency", value: 0.25, labels: { service: "api" } },
    ],
  },
});
assert(
  buildGoldenSignalInventory([metricsSnapshot]).signals.some((signal) => signal.name === "http_request_duration_seconds"),
  "metrics snapshot candidates should feed Golden Signals",
);

const observability = entry("obs-1", "observability_evidence", {
  summary: { error_count: 1, dashboard_panel_count: 1 },
  tables: {
    records: [
      {
        kind: "loki_log",
        timestamp: "2026-05-16T10:00:00Z",
        severity: "ERROR",
        message: "request failed",
        trace_id: "trace-obs",
      },
    ],
  },
});
assert(
  buildIncidentTimelineEvents([observability]).some((event) => event.category === "loki_log"),
  "observability evidence should feed Incident Timeline",
);
assert(
  buildGoldenSignalInventory([database]).signals.some((signal) => signal.name === "Database query p95 latency"),
  "database slow-query evidence should feed Golden Signals",
);
assert(
  buildIncidentTimelineEvents([broker]).some((event) => event.category === "queue_pressure"),
  "broker events should feed Incident Timeline",
);

const platform = entry("platform-1", "kubernetes_evidence", {
  summary: { restart_count: 2, oom_killed_count: 1, security_event_count: 1 },
  tables: {
    events: [{ timestamp: "2026-05-16T10:00:00Z", severity: "ERROR", kind: "kubernetes_event", reason: "OOMKilled", message: "container killed" }],
  },
});
assert(
  buildIncidentTimelineEvents([platform]).some((event) => event.label === "OOMKilled"),
  "platform evidence should feed Incident Timeline",
);
assert(
  buildGoldenSignalInventory([platform]).signals.some((signal) => signal.name === "Kubernetes OOMKilled count"),
  "platform evidence should feed Golden Signals",
);

const profileEvidence = entry("profile-1", "profile_evidence", {
  summary: { total_samples: 12, estimated_seconds: 1.2, native_samples: 4, async_frame_samples: 2 },
});
assert(
  buildGoldenSignalInventory([profileEvidence]).signals.some((signal) => signal.name === "Profile sample volume"),
  "profile evidence should feed Golden Signals",
);

const apiContract = entry("api-contract-1", "api_contract_analysis", {
  summary: {
    undocumented_route_count: 1,
    unused_operation_count: 1,
    slow_operation_count: 1,
    high_error_operation_count: 1,
    undocumented_event_channel_count: 1,
  },
  tables: {
    slow_operations: [{ method: "GET", path: "/orders/1001", evidence_ref: "tables.observed_routes[0]" }],
    high_error_operations: [{ method: "GET", path: "/orders/1001", evidence_ref: "tables.observed_routes[0]" }],
    undocumented_routes: [{ method: "GET", path: "/internal/debug", evidence_ref: "tables.observed_routes[1]" }],
  },
});
assert(
  buildIncidentTimelineEvents([apiContract]).some((event) => event.label === "HIGH_ERROR_API_OPERATION"),
  "API contract findings should feed Incident Timeline",
);
assert(
  buildGoldenSignalInventory([apiContract]).signals.some((signal) => signal.name === "Undocumented API routes"),
  "API contract summaries should feed Golden Signals",
);

const stitched = entry("stitched-1", "stitched_evidence", {
  summary: {
    matched_group_count: 2,
    advanced_match_count: 1,
    time_window_match_count: 1,
    trace_profile_link_count: 1,
    gap_count: 1,
  },
  tables: {
    gaps: [{ code: "UNMATCHED_DATABASE_CALL", severity: "warning", message: "db call unmatched", timestamp: "2026-05-16T10:00:00Z" }],
    matches: [
      { key_kind: "trace_id", match_reason: "trace_profile_label", confidence: 0.98, event_count: 3, first_seen: "2026-05-16T10:00:00Z", last_seen: "2026-05-16T10:00:01Z" },
      { key_kind: "time_window", match_reason: "time_window_service_alias", confidence: 0.76, event_count: 2, first_seen: "2026-05-16T10:00:10Z", last_seen: "2026-05-16T10:00:35Z" },
    ],
    service_dependencies: [{ caller: "order-service", callee: "database:shop", call_count: 1, match_status: "stitched_time_window" }],
  },
});
assert(
  buildIncidentTimelineEvents([stitched]).some((event) => event.label === "UNMATCHED_DATABASE_CALL"),
  "stitched evidence gaps should feed Incident Timeline",
);
assert(
  buildIncidentTimelineEvents([stitched]).some((event) => event.label === "TIME_WINDOW_STITCH"),
  "advanced stitched matches should feed Incident Timeline",
);
assert(buildServiceFlowAnalysis([stitched]).edge_model.edge_count === 1, "stitched dependencies should feed Service Flow");
assert(
  buildGoldenSignalInventory([stitched]).signals.some((signal) => signal.name === "Time-window stitched matches"),
  "advanced stitched summaries should feed Golden Signals",
);

const aiResult = entry("ai-1", "jfr_recording", {
  tables: {
    notable_events: [{ evidence_ref: "jfr:event:1", message: "GC pause 120ms" }],
  },
  metadata: {
    ai_interpretation: {
      schema_version: "0.1.0",
      findings: [
        {
          id: "ai-1",
          label: "GC pause",
          severity: "warning",
          generated_by: "ai",
          model: "test",
          summary: "GC pause detected",
          reasoning: "The evidence mentions the pause.",
          evidence_refs: ["jfr:event:1"],
          confidence: 0.8,
          limitations: [],
        },
      ],
    },
  },
}).result;
const gate = evaluateAiInterpretation(aiResult, extractAiInterpretation(aiResult));
assert(gate.issue_codes.includes("EVIDENCE_QUOTES_REQUIRED"), "AI gate should require evidence quotes");

// ── Browser CPU profile page (profile_evidence) derivations ──────────
//
// These guard the U1/U2/U4 UI contract on the render-logic side: engine
// diagnostics must surface, the flamegraph must adapt, and an empty
// sampled-run timeline must be explained (hitCount-only vs. downsampled)
// rather than shown as a silent empty table.

function profileResult(overrides: Record<string, unknown>): any {
  return {
    type: "profile_evidence",
    source_files: ["profile.cpuprofile"],
    created_at: "2026-07-21T00:00:00.000Z",
    summary: {},
    series: {},
    tables: {},
    charts: {},
    metadata: {},
    ...overrides,
  };
}

const flameResult = profileResult({
  charts: {
    flamegraph: {
      name: "root",
      samples: 100,
      category: "browser_application",
      color: "#0ea5e9",
      children: [
        { name: "render", samples: 60, category: null, color: null, children: [] },
      ],
    },
  },
});
const adaptedFlame = extractProfileFlamegraph(flameResult);
assert(adaptedFlame !== null, "flamegraph should adapt from charts.flamegraph");
assert(adaptedFlame?.value === 100, "flame root value should mirror engine samples");
assert(adaptedFlame?.children?.[0]?.name === "render", "flame children should adapt recursively");
assert(adaptProfileFlameNode(undefined) === null, "missing flame node should adapt to null");

const diagResult = profileResult({
  summary: { value_unit: "samples" },
  metadata: {
    diagnostics: {
      total_lines: 0,
      parsed_records: 10,
      skipped_lines: 0,
      skipped_by_reason: {},
      samples: [],
      warning_count: 1,
      error_count: 0,
      warnings: [
        {
          line_number: 0,
          reason: "PROFILE_HITCOUNT_ONLY",
          message: "profile has hitCount aggregates but no ordered samples",
          raw_preview: "",
        },
      ],
      errors: [],
    },
  },
});
const diags = extractProfileDiagnostics(diagResult);
assert(diags !== null, "metadata.diagnostics should be extracted for the UI");
assert(profileDiagnosticIssueCount(diags) === 1, "diagnostic issue badge should count warnings + errors");
assert(profileDiagnosticIssueCount(null) === 0, "null diagnostics should count as zero issues");

const hitCountState = describeTimelineState(diagResult);
assert(hitCountState.hasRuns === false, "hitCount-only profile has no sampled runs");
assert(hitCountState.reason === "hitcount_only", "hitCount-only empty timeline must be explained, not silent");
assert(hitCountState.suppressed === true, "hitCount-only timeline is a suppressed state");

const downsampledState = describeTimelineState(
  profileResult({
    summary: { value_unit: "microseconds" },
    metadata: {
      parser_metadata: {
        partial_result: true,
        downsampled_from_samples: 900000,
        downsampled_to_samples: 500000,
      },
      diagnostics: {
        total_lines: 0,
        parsed_records: 0,
        skipped_lines: 0,
        skipped_by_reason: {},
        samples: [],
        warning_count: 1,
        error_count: 0,
        warnings: [
          {
            line_number: 0,
            reason: "TIMELINE_SUPPRESSED_DOWNSAMPLED",
            message: "temporal outputs are disabled",
            raw_preview: "",
          },
        ],
        errors: [],
      },
    },
  }),
);
assert(downsampledState.reason === "downsampled", "downsampled empty timeline must be explained");
const partialInfo = selectPartialResult(
  profileResult({
    metadata: {
      parser_metadata: {
        partial_result: true,
        downsampled_from_samples: 900000,
        downsampled_to_samples: 500000,
      },
    },
  }),
);
assert(partialInfo.partial === true, "partial_result flag should surface for the partial card");
assert(partialInfo.fromSamples === 900000 && partialInfo.toSamples === 500000, "downsample counts should surface");

const runsState = describeTimelineState(
  profileResult({
    summary: { value_unit: "microseconds" },
    tables: { cpu_sample_runs: [{ stack: "a;b", duration_ms: 12, start_us: 1000 }] },
  }),
);
assert(runsState.hasRuns === true, "microsecond profile with runs should render the runs table");
assert(runsState.reason === null, "populated timeline has no empty-state reason");

// ── HTTP capture (offline HAR) page derivations — T-579 ──────────────
//
// These guard the Claude-owned H-RG1 UI contract on the render-logic
// side: the pseudo-process tree must group HAR transactions without OS
// process attribution, list/detail filters must compose, the timeline
// brush must map bucket indices to an inclusive time window, and the
// redaction/fidelity metadata must surface for the capture-honesty UX.

function harTx(overrides: Record<string, unknown>): any {
  return {
    id: "har-000001",
    connection_id: "c1",
    sequence: 0,
    started_at: "2026-07-20T05:00:00Z",
    ended_at: "2026-07-20T05:00:00.100Z",
    method: "GET",
    url: "https://shop.example.test/",
    host: "shop.example.test",
    path: "/",
    status: 200,
    status_text: "OK",
    http_version: "http/1.1",
    state: "complete",
    duration_ms: 100,
    request_bytes: 0,
    response_bytes: 1310,
    used_existing_connection: false,
    capture_mode: "har_import",
    coverage: "full",
    fidelity: "semantic",
    request: { headers: [], cookies: [], headerSize: 0, bodySize: 0, bodyDecoded: 0, transferSize: -1, bodyStorage: "omitted" },
    response: { headers: [], cookies: [], headerSize: 0, bodySize: 1024, bodyDecoded: 2048, transferSize: 1310, bodyStorage: "inline" },
    timings: {},
    process: null,
    ...overrides,
  };
}

function httpResult(overrides: Record<string, unknown>): any {
  return {
    type: "http_capture",
    source_files: ["chrome.har"],
    created_at: "2026-07-21T00:00:00.000Z",
    summary: {},
    series: {},
    tables: {},
    charts: {},
    metadata: {},
    ...overrides,
  };
}

const harTransactions = [
  harTx({ id: "t1", host: "shop.example.test", connection_id: "c1", method: "GET", path: "/", status: 200, duration_ms: 120, started_at: "2026-07-20T05:00:00Z" }),
  harTx({ id: "t2", host: "shop.example.test", connection_id: "c1", method: "POST", path: "/cart", status: 500, state: "failed", duration_ms: 300, started_at: "2026-07-20T05:01:00Z" }),
  harTx({ id: "t3", host: "cdn.example.test", connection_id: "c2", method: "GET", path: "/app.js", status: 404, duration_ms: 40, started_at: "2026-07-20T05:02:00Z" }),
];

// Pseudo-process tree: two hosts → two synthesized process roots.
const tree = buildProcessTree(harTransactions);
assert(tree.length === 2, "each HAR host should synthesize its own pseudo-process root");
assert(tree.every((node) => node.pseudo === true), "HAR roots are pseudo-processes (no OS attribution)");
const shopRoot = tree.find((node) => node.label === "shop.example.test");
assert(shopRoot !== undefined, "host label should name the pseudo-process root");
assert(shopRoot?.count === 2, "pseudo-process root should aggregate its transactions");
assert(shopRoot?.errorCount === 1, "pseudo-process error aggregation should count the failed POST");
assert(shopRoot?.children.length === 1, "shared connection id should collapse under one connection node");
assert(shopRoot?.children[0]?.children.length === 2, "connection node should hold both transaction leaves");
assert(shopRoot?.children[0]?.children[0]?.kind === "transaction", "leaves should be transaction nodes");
// Roots sort by total duration desc → shop (420ms) before cdn (40ms).
assert(tree[0]?.label === "shop.example.test", "pseudo-process roots should sort by total duration");

// Real OS process attribution should root under the process, not the host.
const liveTree = buildProcessTree([
  harTx({ id: "p1", host: "api.example.test", process: { pid: 4242, start_time: "2026-07-20T04:59:00Z", name: "chrome.exe", attribution: "etw_pid" } }),
]);
assert(liveTree[0]?.pseudo === false, "OS process attribution should not be marked pseudo");
assert(liveTree[0]?.label === "chrome.exe", "process node should be labeled by process name");
assert(liveTree[0]?.attribution === "etw_pid", "process attribution should carry through");

// Status classification + error predicate.
assert(statusClassOf(200) === "2xx" && statusClassOf(404) === "4xx" && statusClassOf(503) === "5xx", "status classes should bucket by hundreds");
assert(isErrorTransaction(harTransactions[1]!) === true, "failed state should be an error transaction");
assert(isErrorTransaction(harTransactions[0]!) === false, "2xx complete should not be an error");

// Filter composition.
assert(availableMethods(harTransactions).join(",") === "GET,POST", "method dropdown should list distinct sorted methods");
assert(filterTransactions(harTransactions, { ...emptyFilter, errorsOnly: true }).length === 2, "errorsOnly should keep 5xx + 4xx");
assert(filterTransactions(harTransactions, { ...emptyFilter, method: "POST" }).length === 1, "method filter should narrow to POST");
assert(filterTransactions(harTransactions, { ...emptyFilter, statusClass: "4xx" })[0]?.id === "t3", "status-class filter should select the 404");
assert(filterTransactions(harTransactions, { ...emptyFilter, query: "cart" }).length === 1, "query filter should match the path substring");
assert(filterTransactions(harTransactions, emptyFilter).length === 3, "empty filter should pass everything");

// Timeline brush → inclusive window (bucket end pushed one minute forward).
const buckets = [
  { start: "2026-07-20T05:00:00Z", end: "2026-07-20T05:00:00Z", bucket_minutes: 1, count: 1, errors: 0, error_rate: 0, request_bytes: 0, response_bytes: 0, total_duration_ms: 120 },
  { start: "2026-07-20T05:01:00Z", end: "2026-07-20T05:01:00Z", bucket_minutes: 1, count: 1, errors: 1, error_rate: 1, request_bytes: 0, response_bytes: 0, total_duration_ms: 300 },
];
const window = timelineWindow(buckets, 0, 0);
assert(window?.start === "2026-07-20T05:00:00Z", "brush window should start at the first bucket");
assert(window?.end === "2026-07-20T05:01:00.000Z", "brush window end should cover the whole selected minute");
const windowed = filterTransactions(harTransactions, { ...emptyFilter, window });
assert(windowed.length === 1 && windowed[0]?.id === "t1", "brush window should filter transactions to the selected minute");
// Reversed (right-to-left) brush should normalize to the same span.
assert(timelineWindow(buckets, 1, 0)?.start === "2026-07-20T05:00:00Z", "reversed brush indices should normalize");
assert(timelineWindow([], 0, 0) === null, "empty timeline yields no brush window");

// Redaction + capture-fidelity metadata surfacing.
const redacted = httpResult({
  metadata: {
    http_capture: {
      dialect: "chrome",
      capture_mode: "har_import",
      observation_point: "foreign_tool",
      fidelity: "semantic",
      detail_storage: "bounded_inline",
      truncated: false,
      redaction: { applied: true, version: "har_redaction_1.0.0", rules: ["header_value", "query_value"], counts: { header_value: 3, query_value: 1 } },
    },
  },
});
const redaction = extractRedaction(redacted);
assert(redaction?.applied === true, "redaction applied flag should surface");
assert(redaction?.total === 4, "redaction total should sum per-rule counts");
assert(redaction?.rules.includes("query_value"), "redaction rule list should surface");
const captureMeta = extractCaptureMeta(redacted);
assert(captureMeta?.fidelity === "semantic", "capture fidelity should surface for the honesty banner");
assert(captureMeta?.observation_point === "foreign_tool", "observation point should surface");
assert(extractRedaction(httpResult({ metadata: {} })) === null, "missing capture metadata yields null redaction");

// Timing breakdown flattens the imported HAR phases in order.
const timed = harTx({
  timings: {
    importedHar: {
      blocked: { ms: 1.4, state: "known" },
      dns: { ms: 6.2, state: "known" },
      connect: { ms: 11.4, state: "known" },
      tls: { ms: 18.7, state: "known" },
      send: { ms: 0.2, state: "known" },
      wait: { ms: 84.9, state: "known" },
      receive: { ms: 5.1, state: "unknown" },
    },
  },
});
const phases = timingBreakdown(timed);
assert(phases.length === 7, "timing breakdown should list all seven phases");
assert(phases[0]?.phase === "blocked" && phases[5]?.phase === "wait", "timing phases should be in wire order");
assert(phases[5]?.ms === 84.9, "known phase durations should surface");
assert(phases[6]?.ms === 0 && phases[6]?.state === "unknown", "unknown phase should read zero ms with unknown state");
assert(timingBreakdown(null).length === 0, "null transaction yields no timing rows");

// ── H-RG1 U1: shared-denominator summary projection ─────────────────
//
// When a brush window or filter is active the summary cards must recompute
// over the same rows the list/tree render, so every panel agrees on one
// numerator/denominator instead of the cards showing whole-session totals.
const projectionRows = [
  harTx({ id: "p1", host: "a.example.test", path: "/", status: 200, duration_ms: 10, response_bytes: 100 }),
  harTx({ id: "p2", host: "a.example.test", path: "/x", status: 500, state: "failed", duration_ms: 50, response_bytes: 200 }),
  harTx({ id: "p3", host: "b.example.test", path: "/y", status: 404, duration_ms: 90, response_bytes: 300 }),
  harTx({ id: "p4", host: "b.example.test", path: "/z", status: 200, duration_ms: 130, response_bytes: 400 }),
];
const fullProjection = projectSummary(projectionRows);
assert(fullProjection.transactions === 4, "projection counts every row in scope");
assert(fullProjection.errorTransactions === 2, "projection counts 5xx + 4xx as errors");
assert(Math.abs(fullProjection.errorRate - 0.5) < 1e-9, "projection error rate uses the in-scope denominator");
assert(fullProjection.uniqueHosts === 2, "projection counts distinct hosts in scope");
assert(fullProjection.uniqueEndpoints === 4, "projection counts distinct method+host+path endpoints");
assert(fullProjection.responseBytes === 1000, "projection sums response bytes in scope");
assert(fullProjection.durationP95Ms === 130, "projection p95 uses nearest-rank over in-scope durations");
// The projected denominator must equal the filtered list length exactly.
const scopedFilter = { ...emptyFilter, statusClass: "2xx" as const };
const scopedRows = filterTransactions(projectionRows, scopedFilter);
const scopedProjection = projectSummary(scopedRows);
assert(
  scopedProjection.transactions === scopedRows.length && scopedProjection.transactions === 2,
  "cards and list share one denominator under an active filter",
);
assert(scopedProjection.errorTransactions === 0, "2xx-only scope recomputes zero errors");
assert(projectSummary([]).durationP95Ms === 0 && projectSummary([]).errorRate === 0, "empty scope projects zeros");

// isFilterActive detects every contracted axis and ignores the empty filter.
assert(isFilterActive(emptyFilter) === false, "the empty filter is inactive");
assert(isFilterActive({ ...emptyFilter, mime: "application/json" }) === true, "mime makes the filter active");
assert(isFilterActive({ ...emptyFilter, minDurationMs: 5 }) === true, "duration lower bound makes the filter active");
assert(isFilterActive({ ...emptyFilter, window: { start: "a", end: "b" } }) === true, "a brush window makes the filter active");

// ── H-RG1 U2: MIME / duration / fidelity filters ────────────────────
const mimeRows = [
  harTx({ id: "m1", duration_ms: 20, fidelity: "semantic", response: { headers: [], cookies: [], headerSize: 0, bodySize: 10, bodyDecoded: 10, transferSize: 10, bodyStorage: "inline", contentType: "application/json; charset=utf-8" } }),
  harTx({ id: "m2", duration_ms: 200, fidelity: "structural", response: { headers: [], cookies: [], headerSize: 0, bodySize: 10, bodyDecoded: 10, transferSize: 10, bodyStorage: "inline", contentType: "text/html" } }),
  harTx({ id: "m3", duration_ms: 800, fidelity: "semantic", response: { headers: [], cookies: [], headerSize: 0, bodySize: 10, bodyDecoded: 10, transferSize: 10, bodyStorage: "inline", contentType: "application/json" } }),
];
assert(transactionMime(mimeRows[0]!) === "application/json", "content-type params are stripped for the MIME axis");
assert(availableMimeTypes(mimeRows).join(",") === "application/json,text/html", "MIME dropdown lists distinct sorted base types");
assert(availableFidelities(mimeRows).join(",") === "semantic,structural", "fidelity dropdown lists distinct sorted values");
assert(filterTransactions(mimeRows, { ...emptyFilter, mime: "application/json" }).length === 2, "MIME filter keeps matching content types");
assert(filterTransactions(mimeRows, { ...emptyFilter, fidelity: "structural" })[0]?.id === "m2", "fidelity filter narrows to the fidelity");
assert(filterTransactions(mimeRows, { ...emptyFilter, minDurationMs: 100 }).length === 2, "min duration keeps the slower requests");
assert(filterTransactions(mimeRows, { ...emptyFilter, maxDurationMs: 100 })[0]?.id === "m1", "max duration keeps the faster request");
assert(filterTransactions(mimeRows, { ...emptyFilter, minDurationMs: 100, maxDurationMs: 500 })[0]?.id === "m2", "a duration range brackets both bounds");
// MIME filter composed with the timeline brush window (crossed axes).
const crossRows = [
  harTx({ id: "c1", started_at: "2026-07-20T05:00:00Z", duration_ms: 10, response: { headers: [], cookies: [], headerSize: 0, bodySize: 0, bodyDecoded: 0, transferSize: 0, bodyStorage: "inline", contentType: "application/json" } }),
  harTx({ id: "c2", started_at: "2026-07-20T05:01:00Z", duration_ms: 10, response: { headers: [], cookies: [], headerSize: 0, bodySize: 0, bodyDecoded: 0, transferSize: 0, bodyStorage: "inline", contentType: "text/html" } }),
  harTx({ id: "c3", started_at: "2026-07-20T05:00:30Z", duration_ms: 10, response: { headers: [], cookies: [], headerSize: 0, bodySize: 0, bodyDecoded: 0, transferSize: 0, bodyStorage: "inline", contentType: "application/json" } }),
];
const crossWindow = { start: "2026-07-20T05:00:00Z", end: "2026-07-20T05:01:00.000Z" };
const crossed = filterTransactions(crossRows, { ...emptyFilter, mime: "application/json", window: crossWindow });
assert(crossed.length === 2 && crossed.every((tx) => tx.id !== "c2"), "MIME + window compose to intersect both axes");

// ── H-RG1 U3: stale-result provenance reducer ───────────────────────
//
// A result must never render under a different source. Selecting a new file,
// starting a new run, and a failed run all drop the prior result.
const resultA: any = httpResult({ source_files: ["a.har"], summary: { total_transactions: 3 } });
const resultB: any = httpResult({ source_files: ["b.har"], summary: { total_transactions: 9 } });
let s = initialHttpCaptureState;
assert(s.result === null && s.resultSource === null, "initial state carries no result");
s = httpCaptureReducer(s, { type: "analyzeStart" });
assert(s.running === true, "analyzeStart marks the run in progress");
s = httpCaptureReducer(s, { type: "analyzeSuccess", result: resultA, source: "a.har" });
assert(s.result === resultA && s.resultSource === "a.har" && s.running === false, "successful analysis binds result to its source");
// User selects file B → prior result A must be dropped before any B analysis.
s = httpCaptureReducer(s, { type: "reset" });
assert(s.result === null && s.resultSource === null && s.error === null, "selecting a new file clears the previous result");
// Analyze B, then B fails → no stale A result may remain under the error.
s = httpCaptureReducer(s, { type: "analyzeStart" });
assert(s.result === null, "analyzeStart clears any prior result up front");
s = httpCaptureReducer(s, { type: "analyzeError", error: { code: "X", message: "boom" } });
assert(s.result === null && s.resultSource === null, "a failed analysis leaves no result");
assert(s.error?.code === "X", "a failed analysis surfaces its error");
// A→B success replaces provenance cleanly (no bleed-through of A).
s = httpCaptureReducer(initialHttpCaptureState, { type: "analyzeSuccess", result: resultA, source: "a.har" });
s = httpCaptureReducer(s, { type: "analyzeStart" });
s = httpCaptureReducer(s, { type: "analyzeSuccess", result: resultB, source: "b.har" });
assert(s.result === resultB && s.resultSource === "b.har", "a fresh success replaces the previous source binding");
// Filter/selection actions are preserved across their own reducers.
s = httpCaptureReducer(s, { type: "patchFilter", patch: { method: "POST" } });
assert(s.filter.method === "POST", "patchFilter merges into the active filter");
s = httpCaptureReducer(s, { type: "select", id: "tx-9" });
assert(s.selectedId === "tx-9", "select records the open transaction id");
s = httpCaptureReducer(s, { type: "closeDetail" });
assert(s.selectedId === null, "closeDetail clears the open transaction id");

// ── H-RG1 U6: Workspace registration of a populated http_capture result ─
clearWorkspaceResults();
const workspaceResult: any = httpResult({
  source_files: ["chrome.har"],
  summary: { total_transactions: 42, error_rate: 0.1 },
});
const workspaceEntry = addWorkspaceResult({
  result: workspaceResult,
  title: "http_capture: chrome.har",
  sourceLabel: "chrome.har",
});
const reloaded = getWorkspaceEntry(workspaceEntry.id);
assert(reloaded?.result_type === "http_capture", "Workspace preserves the http_capture result type");
assert(reloaded?.source_files.includes("chrome.har"), "Workspace preserves the source label/files");
assert((reloaded?.result as any)?.summary?.total_transactions === 42, "Workspace retains the populated summary");
clearWorkspaceResults();

// ── T-586: populated browser_audit_evidence page derivations ───────────
// Mirrors the frozen engine emit shape
// (apps/engine-native/internal/analyzers/lighthouse/analyzer.go). The page
// consumes only this contract and must never fold audit evidence into the
// CPU-profile sample model.
const browserAudit: any = {
  type: "browser_audit_evidence",
  source_files: ["report.json"],
  created_at: "2026-07-22T00:00:00.000Z",
  summary: {
    source_format: "lighthouse-json",
    score_source: "imported_lighthouse_report",
    score_disclosure:
      "Scores are preserved from the imported Lighthouse report; ArchScope does not recompute them.",
    scores_recomputed: false,
    lighthouse_version: "11.0.0",
    final_url: "https://example.test/",
    form_factor: "mobile",
    performance_score_pct: 42,
    audit_count: 3,
    network_request_count: 2,
    transfer_size_bytes: 2048,
  },
  series: {
    category_scores: [
      { id: "performance", title: "Performance", score: 0.42, score_pct: 42, source_ref: "category:performance" },
      { id: "seo", title: "SEO", score: 0.95, score_pct: 95, source_ref: "category:seo" },
    ],
    core_metrics: [
      {
        id: "largest-contentful-paint",
        title: "Largest Contentful Paint",
        value: 4200,
        unit: "millisecond",
        display_value: "4.2 s",
        score: 0.3,
        source_ref: "audit:largest-contentful-paint",
      },
    ],
    resource_type_distribution: [
      { resource_type: "script", request_count: 1, transfer_size_bytes: 1500, source_ref: "resource_type:script" },
    ],
  },
  tables: {
    audits: [
      { id: "unused-css-rules", title: "Reduce unused CSS", score: 0.1, score_pct: 10, display_value: "1 KiB", source_ref: "audit:unused-css-rules" },
      { id: "viewport", title: "Has viewport", score: 1, score_pct: 100, source_ref: "audit:viewport" },
    ],
    network_requests: [
      { url: "https://example.test/app.js", resource_type: "Script", transfer_size_bytes: 1500, duration_ms: 120, source_ref: "network_request:1" },
    ],
    resource_summary: [
      { resource_type: "Script", request_count: 1, transfer_size_bytes: 1500, source_ref: "resource_type:script" },
    ],
  },
  charts: {},
  metadata: {
    schema_version: "0.1.0",
    format: "lighthouse-json",
    browser_audit_contract: {
      version: "1.0.0",
      result_type: "browser_audit_evidence",
      score_source: "imported_lighthouse_report",
      score_disclosure:
        "Scores are preserved from the imported Lighthouse report; ArchScope does not recompute them.",
      scores_recomputed: false,
      view_keys: {
        series: ["category_scores", "core_metrics", "resource_type_distribution"],
        tables: ["audits", "network_requests", "resource_summary"],
      },
      export_formats: ["json", "html", "pptx", "csv", "csv_dir"],
    },
    diagnostics: { warning_count: 1, error_count: 0, warnings: [{ reason: "LIGHTHOUSE_TRUNCATED" }] },
    findings: [
      {
        severity: "warning",
        code: "LIGHTHOUSE_PERFORMANCE_POOR",
        message: "The imported Lighthouse report rated performance below 50.",
        evidence: { score_pct: 42, source_ref: "category:performance", score_source: "imported_lighthouse_report" },
      },
    ],
  },
};

const provenance = selectScoreProvenance(browserAudit);
assert(provenance.scoresRecomputed === false, "browser audit must disclose scores are NOT recomputed");
assert(
  provenance.scoreSource === "imported_lighthouse_report",
  "browser audit provenance must name the imported-report source",
);
assert(provenance.scoreDisclosure.length > 0, "browser audit must carry a human-readable score disclosure");
// A malformed payload must never imply recomputation.
assert(selectScoreProvenance(null).scoresRecomputed === false, "missing browser audit defaults to not-recomputed");

const contract = selectBrowserAuditContract(browserAudit);
assert(contract?.version === "1.0.0", "browser audit contract version is surfaced to the UI");
assert(
  Array.isArray(contract?.export_formats) && contract!.export_formats!.includes("html"),
  "browser audit contract declares HTML export coverage",
);

const auditCategories = selectCategoryScores(browserAudit);
assert(auditCategories.length === 2, "category scores are projected for the UI");
assert(scorePctBand(42) === "poor", "sub-50 category scores band as poor");
assert(scorePctBand(95) === "good", "90+ category scores band as good");
assert(scorePctBand(null) === "unknown", "missing score bands as unknown, not poor");

const vitals = selectCoreMetrics(browserAudit);
assert(vitals.length === 1 && vitals[0]?.id === "largest-contentful-paint", "Core Web Vitals rows are projected");
assert(scoreToPct(0.3) === 30, "0-1 audit scores normalize to a 0-100 percentage");
assert(scoreToPct("x") === null, "non-numeric audit scores normalize to null, not 0");

const browserAuditRows = selectAuditRows(browserAudit);
assert(browserAuditRows.length === 2, "bounded audits table is projected");
assert(browserAuditRows[0]?.source_ref === "audit:unused-css-rules", "audit rows keep their stable source_ref");
assert(selectNetworkRequests(browserAudit).length === 1, "network requests table is projected");
assert(selectResourceDistribution(browserAudit).length === 1, "resource distribution is projected");

const auditDiags = extractBrowserAuditDiagnostics(browserAudit);
assert(browserAuditDiagnosticIssueCount(auditDiags) === 1, "browser audit diagnostics issue badge counts warnings + errors");
assert(browserAuditDiagnosticIssueCount(null) === 0, "null browser audit diagnostics count as zero issues");

// Evidence Board capture must preserve the frozen finding source_ref so the
// card links back to the exact category/audit in the imported report.
const auditFindings = selectBrowserAuditFindings(browserAudit);
assert(auditFindings.length === 1, "engine findings are projected for Evidence Board capture");
const evidence = buildBrowserAuditEvidence(auditFindings[0]!, "report.json");
assert(evidence.analyzer === "browser_audit", "browser audit evidence is tagged with its own analyzer id");
assert(evidence.source_ref === "category:performance", "evidence capture preserves the finding source_ref");
assert(evidence.source_file === "report.json", "evidence capture records the source file");
assert(evidence.severity === "warning", "evidence capture preserves finding severity");

// Workspace registration must keep browser_audit_evidence distinct from the
// CPU-profile sample model (profile_evidence).
clearWorkspaceResults();
const browserAuditEntry = addWorkspaceResult({
  result: browserAudit,
  title: "browser_audit: report.json",
  sourceLabel: "report.json",
});
const browserAuditReloaded = getWorkspaceEntry(browserAuditEntry.id);
assert(
  browserAuditReloaded?.result_type === "browser_audit_evidence",
  "Workspace preserves the browser_audit_evidence result type, separate from profile_evidence",
);
assert(
  (browserAuditReloaded?.result as any)?.summary?.performance_score_pct === 42,
  "Workspace retains the imported performance score",
);
clearWorkspaceResults();
