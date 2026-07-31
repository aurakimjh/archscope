import { messages } from "../i18n/messages.js";
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
  CAPTURE_COVERAGE_LABEL_KEYS,
  CAPTURE_DETAIL_STORAGE_LABEL_KEYS,
  CAPTURE_FIDELITY_LABEL_KEYS,
  CAPTURE_MODE_LABEL_KEYS,
  CAPTURE_OBSERVATION_LABEL_KEYS,
  CAPTURE_PROVENANCE_HINT_KEYS,
  availableFidelities,
  availableMethods,
  availableMimeTypes,
  buildProcessTree,
  captureFidelityHintKey,
  extractCaptureMeta,
  extractRedaction,
  resolveCaptureCoverage,
  resolveCaptureDetailStorage,
  resolveCaptureFidelity,
  resolveCaptureMode,
  resolveCaptureObservation,
  resolveCaptureProvenance,
  selectCaptureCoverageDistribution,
  selectCaptureFidelityDistribution,
  selectCaptureModeDistribution,
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
  activeUnattributedPolicy,
  buildLiveCoverageDisclosure,
  buildLiveProcessGroups,
  countLiveInFlight,
  DEFAULT_LIVE_CAPTURE_CONTRACT,
  initialLiveCaptureState,
  isDecodedLiveFidelity,
  isLiveCaptureContractSupported,
  isLiveSessionActive,
  isLiveTransactionInFlight,
  LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION,
  LIVE_TRANSACTION_ROW_CAP,
  liveFidelityTone,
  liveHttpCaptureReducer,
  resolveLiveAttribution,
  resolveLiveCAState,
  resolveLiveCaptureContract,
  resolveLiveFidelity,
  resolveLiveSessionState,
  resolveLiveTransactionState,
  type LiveCaptureTransaction,
} from "./liveHttpCapture.js";
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

// ── H-RG4 S1/S3/S4: the finalized fidelity card ─────────────────────
//
// The card explains its metadata in prose, labels its values, and states what
// it knows about redaction. Each of those three was wrong for a finalized live
// session: the prose was a fixed HAR sentence printed under an
// `observation_point: proxy` field (S1), the values were raw engine tokens in
// both locales (S4), and an absent redaction summary read as a clean sheet
// (S3).
const liveFinalized = httpResult({
  metadata: {
    http_capture: {
      dialect: "archscope-live",
      capture_mode: "mixed",
      capture_mode_counts: { proxy_mitm: 200, proxy_not_captured: 1 },
      observation_point: "proxy",
      fidelity: "unsupported",
      fidelity_counts: { decoded_wire: 200, unsupported: 1 },
      coverage_counts: { confirmed: 195, unknown: 6 },
      detail_storage: "bounded_inline",
      truncated: false,
      redaction: { applied: true, known: true, version: "capture_redaction_1.0.0", rules: ["query_value"], counts: { query_value: 2 } },
    },
  },
});
assert(
  resolveCaptureProvenance(extractCaptureMeta(liveFinalized)) === "live_proxy",
  "a proxy observation point is live provenance",
);
assert(
  resolveCaptureProvenance(extractCaptureMeta(redacted)) === "har_import",
  "a foreign-tool observation point is HAR provenance",
);
assert(
  resolveCaptureProvenance(extractCaptureMeta(httpResult({ metadata: { http_capture: { observation_point: "sidecar" } } }))) === "unknown",
  "an unrecognized observation point claims neither origin",
);
assert(
  captureFidelityHintKey(extractCaptureMeta(liveFinalized)) === "httpCaptureFidelityHintLive",
  "a finalized live session is explained as a live proxy capture",
);
assert(
  captureFidelityHintKey(extractCaptureMeta(redacted)) === "httpCaptureFidelityHintHar",
  "a HAR import keeps the imported-evidence explanation",
);
assert(
  captureFidelityHintKey(null) === "httpCaptureFidelityHintUnknown",
  "an unknown provenance asserts neither origin",
);
// The HAR sentence must be reachable only from the HAR path — an
// unconditional key would put it back on every result (S1).
assert(
  !("httpCaptureFidelityHint" in messages.en) && !("httpCaptureFidelityHint" in messages.ko),
  "no unconditional fidelity hint remains",
);
assert(
  !/HAR|foreign-tool/.test((messages.en as Record<string, string>).httpCaptureFidelityHintLive!),
  "the live explanation makes no HAR or foreign-tool claim",
);

// Raw engine tokens resolve through closed label maps, and anything the
// renderer does not recognize says so instead of leaking the wire value.
assert(
  resolveCaptureFidelity("decoded_wire") === "decoded_wire" &&
    resolveCaptureFidelity("proxy_passthrough") === "unknown" &&
    resolveCaptureFidelity("") === "unknown",
  "finalized fidelity resolves through a closed token set",
);
assert(
  resolveCaptureMode("mixed") === "mixed" && resolveCaptureMode("har_export") === "unknown",
  "finalized capture mode resolves through a closed token set",
);
assert(
  resolveCaptureObservation("proxy") === "proxy" && resolveCaptureObservation("cdp") === "unknown",
  "observation point resolves through a closed token set",
);
assert(
  resolveCaptureDetailStorage("bounded_inline") === "bounded_inline" &&
    resolveCaptureDetailStorage("full") === "unknown",
  "detail storage resolves through a closed token set",
);
assert(
  resolveCaptureCoverage("confirmed") === "confirmed" && resolveCaptureCoverage("inferred") === "unknown",
  "attribution coverage resolves through a closed token set",
);
const captureLabelKeys = [
  ...Object.values(CAPTURE_PROVENANCE_HINT_KEYS),
  ...Object.values(CAPTURE_FIDELITY_LABEL_KEYS),
  ...Object.values(CAPTURE_MODE_LABEL_KEYS),
  ...Object.values(CAPTURE_OBSERVATION_LABEL_KEYS),
  ...Object.values(CAPTURE_DETAIL_STORAGE_LABEL_KEYS),
  ...Object.values(CAPTURE_COVERAGE_LABEL_KEYS),
];
assert(
  captureLabelKeys.every(
    (key) =>
      typeof (messages.en as Record<string, string>)[key] === "string" &&
      (messages.en as Record<string, string>)[key]!.length > 0 &&
      typeof (messages.ko as Record<string, string>)[key] === "string" &&
      (messages.ko as Record<string, string>)[key]!.length > 0,
  ),
  "every finalized-card token is localized in both languages",
);

// The distributions are what make an aggregate of `mixed` / `unsupported`
// interpretable — 200 of 201 rows decoded is not the same session as none.
const liveMeta = extractCaptureMeta(liveFinalized);
const fidelitySplit = selectCaptureFidelityDistribution(liveMeta);
assert(
  fidelitySplit[0]?.token === "decoded_wire" && fidelitySplit[0]?.count === 200,
  "the fidelity distribution leads with the dominant grade",
);
assert(
  fidelitySplit[1]?.token === "unsupported" && fidelitySplit[1]?.count === 1,
  "the fidelity distribution keeps the weakest grade visible",
);
assert(
  selectCaptureModeDistribution(liveMeta).map((entry) => entry.token).join(",") ===
    "proxy_mitm,proxy_not_captured",
  "the capture-mode distribution is ordered by transaction count",
);
assert(
  selectCaptureCoverageDistribution(liveMeta).some(
    (entry) => entry.token === "unknown" && entry.count === 6,
  ),
  "unattributed rows stay visible in the coverage distribution",
);
// Unrecognized tokens merge into one honest `unknown` bucket rather than
// each printing its own wire value.
assert(
  selectCaptureFidelityDistribution(
    extractCaptureMeta(
      httpResult({ metadata: { http_capture: { fidelity_counts: { weird: 2, alsoweird: 3, decoded_wire: 1 } } } }),
    ),
  ).find((entry) => entry.token === "unknown")?.count === 5,
  "unrecognized grades merge into a single unknown bucket",
);
assert(
  selectCaptureFidelityDistribution(extractCaptureMeta(httpResult({ metadata: { http_capture: {} } }))).length === 0,
  "a result without per-token counts renders no distribution",
);

// S3: a session that never reached `Stop` has no persisted redaction summary.
// "Not recorded" and "nothing matched" are different claims.
const unknownRedaction = extractRedaction(
  httpResult({
    metadata: {
      http_capture: {
        redaction: { applied: false, known: false, version: "capture_redaction_1.0.0", rules: [], counts: {} },
      },
    },
  }),
);
assert(unknownRedaction?.known === false, "an unrecorded redaction summary surfaces as unknown");
assert(unknownRedaction?.applied === false, "an unrecorded summary asserts no redaction of its own");
assert(redaction?.known === true, "a determined redaction summary stays known");
assert(
  extractRedaction(
    httpResult({ metadata: { http_capture: { redaction: { applied: true, version: "v1", rules: [], counts: {} } } } }),
  )?.known === true,
  "payloads predating the known flag keep their determined summary",
);

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

// ── T-581: live-capture renderer state and recovery contract ─────────
const liveSession = {
  sessionId: "cap-t581",
  state: "running",
  listenAddress: "127.0.0.1:43123",
  storePath: "/capture/cap-t581",
  startedAt: "2026-07-28T00:00:00Z",
  retainUnattributedMetadata: true,
};
let live = liveHttpCaptureReducer(initialLiveCaptureState, {
  type: "started",
  session: liveSession,
});
assert(isLiveSessionActive(live.session), "running live session is active");
const pendingLiveRow: LiveCaptureTransaction = {
  id: "live-pending",
  sequence: 0,
  method: "GET",
  url: "https://example.test/pending",
  host: "example.test",
  path: "/pending",
  statusCode: 0,
  httpVersion: "HTTP/1.1",
  state: "request_sent",
  totalMs: 0,
  captureMode: "proxy_mitm",
  coverage: "confirmed",
  fidelity: "pending",
};
live = liveHttpCaptureReducer(live, {
  type: "progress",
  event: {
    sessionId: liveSession.sessionId,
    items: [pendingLiveRow],
  },
});
assert(
  live.transactions[0]?.state === "request_sent",
  "request progress appears before completion",
);

// H-RG4 L12: the panel dispatches "started" twice for one session (promise plus
// capture:started event). The second dispatch must not discard rows that landed
// in between, and it must not reset the user's follow preference.
live = liveHttpCaptureReducer(live, { type: "follow", follow: false });
live = liveHttpCaptureReducer(live, {
  type: "started",
  session: liveSession,
});
assert(
  live.transactions.length === 1 && live.follow === false,
  "a duplicate started dispatch keeps early rows and the follow preference",
);
live = liveHttpCaptureReducer(live, {
  type: "started",
  session: { ...liveSession, sessionId: "cap-t581-next" },
});
assert(
  live.transactions.length === 0 && live.follow === false,
  "a genuinely new session clears rows but carries the follow preference over",
);
live = liveHttpCaptureReducer(live, { type: "follow", follow: true });
live = liveHttpCaptureReducer(live, {
  type: "started",
  session: liveSession,
});
live = liveHttpCaptureReducer(live, {
  type: "progress",
  event: {
    sessionId: liveSession.sessionId,
    items: [pendingLiveRow],
  },
});

// H-RG4 L4: progress arrives batched, so one event carries many rows.
live = liveHttpCaptureReducer(live, {
  type: "progress",
  event: {
    sessionId: liveSession.sessionId,
    items: [
      { ...pendingLiveRow, id: "live-batch-a", url: "https://example.test/a" },
      { ...pendingLiveRow, id: "live-batch-b", url: "https://example.test/b" },
    ],
  },
});
assert(
  live.transactions.length === 3,
  "a batched progress event applies every row in one dispatch",
);

// H-RG4 L5: in-flight rows are identifiable and have no measured duration yet.
assert(
  countLiveInFlight(live.transactions) === 3 &&
    isLiveTransactionInFlight(pendingLiveRow),
  "rows without a terminal state are reported as in flight",
);
live = liveHttpCaptureReducer(live, {
  type: "progress",
  event: {
    sessionId: liveSession.sessionId,
    items: [
      {
        ...pendingLiveRow,
        id: "live-batch-a",
        state: "aborted",
        error: "capture stopped before the transaction completed",
      },
    ],
  },
});
assert(
  countLiveInFlight(live.transactions) === 2,
  "an engine-supplied terminal state resolves the in-flight row",
);

live = liveHttpCaptureReducer(live, {
  type: "transactions",
  event: {
    sessionId: liveSession.sessionId,
    sequence: 1,
    snapshotVersion: 1,
    items: [
      {
        ...pendingLiveRow,
        state: "complete",
        statusCode: 200,
        totalMs: 12,
        fidelity: "decoded_wire",
      },
    ],
  },
});
assert(
  live.transactions.length === 3 &&
    live.transactions[0]?.state === "complete" &&
    live.transactions[0]?.statusCode === 200,
  "completion replaces the stable in-progress row by transaction id",
);
// H-RG4 L8: the finalized row keeps the position its progress row held instead
// of jumping to the tail past rows that are still pending.
assert(
  live.transactions[0]?.id === "live-pending" &&
    live.transactions[1]?.id === "live-batch-a" &&
    live.transactions[2]?.id === "live-batch-b",
  "finalization replaces a row in place and does not reorder the live table",
);
live = liveHttpCaptureReducer(live, {
  type: "started",
  session: { ...liveSession, sessionId: "cap-t581-reset" },
});
live = liveHttpCaptureReducer(live, {
  type: "started",
  session: liveSession,
});

// H-RG4 L1: no in-flight, passthrough, or unrecognized grade may render as a
// completed semantic decode. Unknown values degrade to "not yet determined".
assert(
  resolveLiveFidelity("pending") === "pending" &&
    resolveLiveFidelity("proxy_passthrough") === "passthrough" &&
    resolveLiveFidelity("unsupported") === "unsupported" &&
    resolveLiveFidelity("decoded_wire") === "decoded_wire",
  "known fidelity grades resolve to their own token",
);
assert(
  resolveLiveFidelity("") === "unknown" &&
    resolveLiveFidelity("h2") === "unknown" &&
    resolveLiveFidelity("SEMANTIC") === "unknown",
  "an unrecognized fidelity degrades to unknown rather than a positive grade",
);
assert(
  !isDecodedLiveFidelity(resolveLiveFidelity("pending")) &&
    !isDecodedLiveFidelity(resolveLiveFidelity("proxy_passthrough")) &&
    !isDecodedLiveFidelity(resolveLiveFidelity("unsupported")) &&
    !isDecodedLiveFidelity(resolveLiveFidelity("h2")) &&
    isDecodedLiveFidelity(resolveLiveFidelity("decoded_wire")),
  "only genuinely decoded grades claim successful capture",
);
// H-RG4 R9: the claimed positive-capture gate must exist in the product path.
// The panel derives its caution emphasis from this tone, so a grade that never
// read the exchange cannot be styled like ordinary captured traffic.
assert(
  liveFidelityTone(resolveLiveFidelity("decoded_wire")) === "decoded" &&
    liveFidelityTone(resolveLiveFidelity("semantic")) === "decoded" &&
    liveFidelityTone(resolveLiveFidelity("pending")) === "pending" &&
    liveFidelityTone(resolveLiveFidelity("h2")) === "pending" &&
    liveFidelityTone(resolveLiveFidelity("proxy_passthrough")) === "limited" &&
    liveFidelityTone(resolveLiveFidelity("unsupported")) === "limited",
  "fidelity tone separates decoded grades from limited and undetermined ones",
);

// ── H-RG4 R11: raw engine enums must never reach the user ──────────────
// Every one of these was printed as its wire token, which left the columns
// added to explain unresolved rows untranslated in both locales.
assert(
  resolveLiveTransactionState("request_sent") === "request_sent" &&
    resolveLiveTransactionState("receiving") === "receiving" &&
    resolveLiveTransactionState("complete") === "complete" &&
    resolveLiveTransactionState("failed") === "failed" &&
    resolveLiveTransactionState("aborted") === "aborted" &&
    resolveLiveTransactionState("") === "unknown" &&
    resolveLiveTransactionState("COMPLETE") === "unknown",
  "transaction states resolve to a closed localizable token set",
);
assert(
  resolveLiveSessionState("running") === "running" &&
    resolveLiveSessionState("recoverable") === "recoverable" &&
    resolveLiveSessionState("idle") === "unknown",
  "session states resolve to a closed localizable token set",
);
assert(
  resolveLiveCAState("trusted") === "trusted" &&
    resolveLiveCAState("partial") === "partial" &&
    resolveLiveCAState("loading") === "loading" &&
    resolveLiveCAState("revoked") === "unknown",
  "CA states resolve to a closed localizable token set",
);
assert(
  resolveLiveAttribution("confirmed") === "confirmed" &&
    resolveLiveAttribution("inferred") === "inferred" &&
    resolveLiveAttribution("") === "unknown",
  "process attribution resolves to a closed localizable token set",
);

// ── H-RG4 R8: the renderer contract is consumed, not restated ──────────
// The engine publishes `capture.LiveCaptureContract`; the renderer derives its
// row cap and recovery behaviour from it, so the acceptance fixture describes
// what this component does instead of a literal that can drift from it.
assert(
  resolveLiveCaptureContract({
    schemaVersion: LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION,
    transactionRowCap: 3,
    resyncOnEventSkip: false,
    restoreCurrentSessionOnPageReentry: false,
    finalizedSessionUsesAnalysisResult: false,
  }).transactionRowCap === 3,
  "a well-formed engine contract is adopted verbatim",
);
assert(
  resolveLiveCaptureContract({
    schemaVersion: LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION + 1,
    transactionRowCap: 3,
    resyncOnEventSkip: false,
    restoreCurrentSessionOnPageReentry: false,
    finalizedSessionUsesAnalysisResult: false,
  }) === DEFAULT_LIVE_CAPTURE_CONTRACT &&
    resolveLiveCaptureContract({
      schemaVersion: LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION,
      transactionRowCap: 0,
      resyncOnEventSkip: true,
      restoreCurrentSessionOnPageReentry: true,
      finalizedSessionUsesAnalysisResult: true,
    }) === DEFAULT_LIVE_CAPTURE_CONTRACT &&
    resolveLiveCaptureContract(null) === DEFAULT_LIVE_CAPTURE_CONTRACT,
  "an unknown or malformed contract falls back to the built-in defaults",
);
assert(
  !isLiveCaptureContractSupported({
    schemaVersion: LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION + 1,
  }) &&
    isLiveCaptureContractSupported({
      schemaVersion: LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION,
    }),
  "a contract this build cannot honour is reported as a mismatch",
);
assert(
  LIVE_TRANSACTION_ROW_CAP === DEFAULT_LIVE_CAPTURE_CONTRACT.transactionRowCap,
  "the renderer fallback cap is the default contract's cap",
);

let contracted = liveHttpCaptureReducer(initialLiveCaptureState, {
  type: "contract",
  contract: {
    schemaVersion: LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION,
    transactionRowCap: 2,
    resyncOnEventSkip: false,
    restoreCurrentSessionOnPageReentry: true,
    finalizedSessionUsesAnalysisResult: true,
  },
});
assert(
  contracted.contract.transactionRowCap === 2 && !contracted.contractMismatch,
  "the reducer adopts the engine's contract",
);
contracted = liveHttpCaptureReducer(contracted, {
  type: "started",
  session: liveSession,
});
assert(
  contracted.contract.transactionRowCap === 2,
  "starting a session does not revert the renderer contract",
);
contracted = liveHttpCaptureReducer(contracted, {
  type: "transactions",
  event: {
    sessionId: liveSession.sessionId,
    sequence: 3,
    snapshotVersion: 3,
    items: ["c-1", "c-2", "c-3"].map((id, index) => ({
      ...pendingLiveRow,
      id,
      sequence: index + 1,
    })),
  },
});
assert(
  contracted.transactions.length === 2 &&
    contracted.transactions[0]?.id === "c-2",
  "the live row cap comes from the engine contract, not a renderer literal",
);
contracted = liveHttpCaptureReducer(contracted, {
  type: "stats",
  stats: {
    sessionId: liveSession.sessionId,
    state: "running",
    observed: 3,
    captured: 3,
    persisted: 3,
    bodyOmitted: 3,
    eventSkipped: 9,
    kernelDropped: 0,
    parseFailed: 0,
    unsupported: 0,
    passthrough: 0,
    unattributed: 0,
    dropped: 0,
    backpressured: false,
    snapshotVersion: 3,
    sequence: 3,
    storeBytes: 0,
  },
});
assert(
  !contracted.needsResync,
  "a contract that disables resync-on-skip disables the renderer recovery path",
);
const mismatched = liveHttpCaptureReducer(initialLiveCaptureState, {
  type: "contract",
  contract: { schemaVersion: LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION + 1 },
});
assert(
  mismatched.contractMismatch &&
    mismatched.contract === DEFAULT_LIVE_CAPTURE_CONTRACT,
  "an unhonourable contract is disclosed and the defaults stay in force",
);

// H-RG4 L6: the running session's SEC-17 policy is authoritative on re-entry,
// where the renderer checkbox is back to its unchecked default.
assert(
  activeUnattributedPolicy(live.session, false) === true,
  "page re-entry shows the running session's unattributed-retention policy",
);
assert(
  activeUnattributedPolicy(
    { ...liveSession, retainUnattributedMetadata: false },
    true,
  ) === false,
  "the local checkbox never overstates a running session's retention policy",
);
assert(
  activeUnattributedPolicy(null, true) === true &&
    activeUnattributedPolicy(
      { ...liveSession, state: "finalized", retainUnattributedMetadata: true },
      false,
    ) === false,
  "with no active session the local choice seeds the next start",
);

const liveRows: LiveCaptureTransaction[] = Array.from(
  { length: LIVE_TRANSACTION_ROW_CAP + 5 },
  (_, index) => ({
    id: `live-${index}`,
    sequence: index + 1,
    method: "GET",
    url: `https://example.test/${index}`,
    host: "example.test",
    path: `/${index}`,
    statusCode: index === 504 ? 500 : 200,
    httpVersion: "HTTP/1.1",
    state: "complete",
    totalMs: index,
    captureMode: "proxy_mitm",
    coverage: "confirmed",
    fidelity: "decoded_wire",
    process: {
      key: { pid: 4242, startTime: "2026-07-28T00:00:00Z" },
      name: "chrome.exe",
      attribution: "confirmed",
    },
  }),
);
live = liveHttpCaptureReducer(live, {
  type: "transactions",
  event: {
    sessionId: liveSession.sessionId,
    sequence: liveRows.length,
    snapshotVersion: liveRows.length,
    items: liveRows,
  },
});
assert(
  live.transactions.length === LIVE_TRANSACTION_ROW_CAP,
  "live renderer applies the 500-row metadata cap",
);
assert(
  live.transactions[0]?.id === "live-5",
  "live row cap keeps the newest stable rows",
);
live = liveHttpCaptureReducer(live, {
  type: "transactions",
  event: {
    sessionId: liveSession.sessionId,
    sequence: liveRows.length + 1,
    snapshotVersion: liveRows.length + 1,
    items: [liveRows[liveRows.length - 1]!],
  },
});
assert(
  live.transactions.length === LIVE_TRANSACTION_ROW_CAP,
  "duplicate transaction events do not duplicate live rows",
);

const liveStats = {
  sessionId: liveSession.sessionId,
  state: "running",
  observed: 1010,
  captured: 505,
  persisted: 505,
  bodyOmitted: 0,
  eventSkipped: 2,
  kernelDropped: 0,
  parseFailed: 0,
  unsupported: 0,
  passthrough: 0,
  unattributed: 505,
  dropped: 505,
  backpressured: false,
  snapshotVersion: 505,
  sequence: 505,
  storeBytes: 4096,
};
// H-RG4 L7: `captured` excludes deliberately dropped records while
// `unattributed` counts them, so `observed` is the only denominator that makes
// the tiles consistent — and mass drops must raise an explanatory warning.
const liveCoverage = buildLiveCoverageDisclosure(liveStats);
assert(
  liveCoverage?.observed === 1010 &&
    liveCoverage.droppedPercent === 50 &&
    liveCoverage.hasDrops &&
    liveCoverage.hasUnattributed,
  "coverage disclosure reports the observed denominator and the drop share",
);
assert(
  buildLiveCoverageDisclosure({ ...liveStats, observed: 0, dropped: 0 })
    ?.droppedPercent === null,
  "a zero denominator never produces a fabricated drop ratio",
);
assert(buildLiveCoverageDisclosure(null) === null, "no stats, no disclosure");
live = liveHttpCaptureReducer(live, { type: "stats", stats: liveStats });
assert(
  live.needsResync,
  "an advancing eventSkipped counter requires authoritative live-window resync",
);
live = liveHttpCaptureReducer(live, {
  type: "resynced",
  stats: liveStats,
  transactions: liveRows.slice(-20),
});
assert(
  !live.needsResync && live.transactions.length === 20,
  "successful resync replaces rows and clears the recovery flag",
);
const processGroups = buildLiveProcessGroups(live.transactions);
assert(
  processGroups.length === 1 &&
    processGroups[0]?.label === "chrome.exe" &&
    processGroups[0]?.errors === 1,
  "live process tree groups confirmed processes and retains error counts",
);
live = liveHttpCaptureReducer(live, { type: "follow", follow: false });
assert(live.follow === false, "manual scroll can suspend live follow mode");
live = liveHttpCaptureReducer(live, {
  type: "stopped",
  session: { ...liveSession, state: "finalized", endedAt: "2026-07-28T00:05:00Z" },
});
assert(
  !isLiveSessionActive(live.session),
  "finalized session exits the active capture state",
);

// H-RG4 L1/L7/L14: every live-capture disclosure the panel can print must exist
// in both locales. The review found drop and unattributed states with no
// explanatory string in either language, which a key-parity check catches.
const enKeys = Object.keys(messages.en);
const koKeys = Object.keys(messages.ko);
assert(
  enKeys.length === koKeys.length &&
    enKeys.every((key) => koKeys.includes(key)),
  "en and ko message catalogs expose the identical key set",
);
const requiredLiveCaptureKeys = [
  "liveCaptureObserved",
  "liveCaptureDropped",
  "liveCaptureKernelDropped",
  "liveCaptureDropWarning",
  "liveCaptureUnattributedWarning",
  "liveCaptureKernelDroppedWarning",
  "liveCaptureCoverageDenominator",
  "liveCaptureDroppedShare",
  "liveCaptureUnknownOptInLocked",
  "liveCaptureUnknownOptInOn",
  "liveCaptureUnknownOptInOff",
  "liveCaptureColState",
  "liveCaptureInFlight",
  "liveCaptureInFlightCount",
  "liveCaptureInFlightHint",
  "liveCaptureFidelityPending",
  "liveCaptureFidelityDecodedWire",
  "liveCaptureFidelitySemantic",
  "liveCaptureFidelityUnsupported",
  "liveCaptureFidelityPassthrough",
  "liveCaptureFidelityUnknown",
  // H-RG4 R11: the raw engine enums the panel used to print verbatim.
  "liveCaptureTxStateRequestSent",
  "liveCaptureTxStateReceiving",
  "liveCaptureTxStateComplete",
  "liveCaptureTxStateFailed",
  "liveCaptureTxStateAborted",
  "liveCaptureTxStateUnknown",
  "liveCaptureSessionStateCreated",
  "liveCaptureSessionStateStarting",
  "liveCaptureSessionStateRunning",
  "liveCaptureSessionStateStopping",
  "liveCaptureSessionStateFinalized",
  "liveCaptureSessionStateFailed",
  "liveCaptureSessionStateRecoverable",
  "liveCaptureSessionStateUnknown",
  "liveCaptureCAStateLoading",
  "liveCaptureCAStateAbsent",
  "liveCaptureCAStateInstalling",
  "liveCaptureCAStateTrusted",
  "liveCaptureCAStatePartial",
  "liveCaptureCAStateFailed",
  "liveCaptureCAStateExpired",
  "liveCaptureCAStateUnknown",
  "liveCaptureAttributionConfirmed",
  "liveCaptureAttributionInferred",
  "liveCaptureAttributionUnknown",
  // H-RG4 R8: the row-cap hint is composed around the contract's value.
  "liveCaptureRowCapPrefix",
  "liveCaptureRowCapSuffix",
  "liveCaptureContractMismatch",
];
assert(
  requiredLiveCaptureKeys.every(
    (key) =>
      typeof (messages.en as Record<string, string>)[key] === "string" &&
      (messages.en as Record<string, string>)[key]!.length > 0 &&
      typeof (messages.ko as Record<string, string>)[key] === "string" &&
      (messages.ko as Record<string, string>)[key]!.length > 0,
  ),
  "live-capture honesty disclosures are localized in both languages",
);
// The two "dropped" counters mean opposite things — a privacy discard versus
// real data loss — so their labels must not be interchangeable (H-RG4 L14).
const enLabels = messages.en as Record<string, string>;
const koLabels = messages.ko as Record<string, string>;
assert(
  enLabels.liveCaptureDropped !== enLabels.liveCaptureKernelDropped &&
    koLabels.liveCaptureDropped !== koLabels.liveCaptureKernelDropped,
  "policy discards and pre-capture loss are labeled distinctly",
);

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

// ── HTTP capture session diff (H-RG5 / T-582) ───────────────────────

import {
  DIFF_ALIGNMENT_LABEL_KEYS,
  DIFF_CHANGE_LABEL_KEYS,
  DIFF_RATE_UNAVAILABLE_LABEL_KEYS,
  DIFF_SOURCE_KIND_LABEL_KEYS,
  HTTP_CAPTURE_DIFF_CONTRACT_SCHEMA_VERSION,
  diffCandidateEntries,
  diffHasChanges,
  diffOverlayPolicy,
  extractDiffEnvelope,
  hasDiffSourceProjection,
  httpCaptureDiffReducer,
  initialHttpCaptureDiffState,
  isDiffContractSupported,
  resolveDiffAlignmentGrade,
  resolveDiffChange,
  resolveDiffRateUnavailable,
  resolveDiffSourceKind,
  selectDiffFindings,
  selectDiffSummary,
  selectDiffTable,
} from "./httpCaptureDiff.js";
import type { HttpCaptureDiffAnalysisResult, HttpCaptureDiffContract } from "../bridge/types.js";

// Contract adoption: only the implemented schema is honored; anything else is
// disabled with a disclosure rather than half-honored (H-RG4 R8 precedent).
const supportedDiffContract: HttpCaptureDiffContract = {
  schema_version: HTTP_CAPTURE_DIFF_CONTRACT_SCHEMA_VERSION,
  source_result_type: "http_capture",
  result_type: "http_capture_diff",
  url_template_version: 1,
  supported_source_versions: [1],
  default_template_limit: 1000,
  max_template_limit: 1000,
  default_table_limit: 50,
  max_table_limit: 500,
  time_alignment_grades: ["aligned", "duration_only", "none"],
  workspace_route: "http_capture_diff",
  workspace_selection_count: 2,
  compare_method: "AnalyzeHttpCaptureDiff",
  legacy_diff_supported: false,
  requires_new_nav_key: false,
  store_rescan_on_diff_or_export: false,
  process_requires_real_sources: true,
};
assert(isDiffContractSupported(supportedDiffContract), "the v1 diff contract is supported");
assert(
  !isDiffContractSupported({ ...supportedDiffContract, schema_version: 2 }),
  "an unknown diff contract schema version is rejected, not half-honored",
);
assert(
  !isDiffContractSupported({ ...supportedDiffContract, result_type: "something_else" }),
  "a diff contract for another result type is rejected",
);
assert(!isDiffContractSupported(null), "a missing diff contract is not supported");

// Closed token sets: every unrecognized wire value resolves to `unknown`
// instead of leaking to the screen (H-RG4 R11 precedent).
assert(resolveDiffAlignmentGrade("aligned") === "aligned", "aligned grade resolves");
assert(resolveDiffAlignmentGrade("duration_only") === "duration_only", "duration_only grade resolves");
assert(resolveDiffAlignmentGrade("none") === "none", "none grade resolves");
assert(resolveDiffAlignmentGrade("brand_new_grade") === "unknown", "unrecognized grade resolves to unknown");
assert(resolveDiffChange("added") === "added" && resolveDiffChange("removed") === "removed", "change tokens resolve");
assert(resolveDiffChange("mutated") === "unknown", "unrecognized change resolves to unknown");
assert(
  resolveDiffRateUnavailable("timestamps_degenerate") === "timestamps_degenerate",
  "degenerate-timestamp rate code resolves",
);
assert(
  resolveDiffRateUnavailable("") === "unknown" && resolveDiffRateUnavailable("??") === "unknown",
  "unrecognized rate codes resolve to unknown",
);
assert(resolveDiffSourceKind("live_capture") === "live_capture", "live source kind resolves");
assert(resolveDiffSourceKind("har_import") === "har_import", "har source kind resolves");
assert(resolveDiffSourceKind("ftp") === "unknown", "unrecognized source kind resolves to unknown");

// Grade-aware overlay gating: the backend's verdict is followed exactly and
// an uninterpretable verdict suppresses every overlay (fail closed).
const alignedPolicy = diffOverlayPolicy({ grade: "aligned", overlay_allowed: true, reason: "" });
assert(alignedPolicy.durations && alignedPolicy.perMinute, "aligned grade enables duration and per-minute overlays");
const durationOnlyPolicy = diffOverlayPolicy({ grade: "duration_only", overlay_allowed: true, reason: "" });
assert(
  durationOnlyPolicy.durations && !durationOnlyPolicy.perMinute,
  "duration_only grade enables durations but suppresses per-minute rates",
);
const nonePolicy = diffOverlayPolicy({ grade: "none", overlay_allowed: false, reason: "" });
assert(!nonePolicy.durations && !nonePolicy.perMinute, "none grade suppresses every overlay");
const disallowedPolicy = diffOverlayPolicy({ grade: "aligned", overlay_allowed: false, reason: "" });
assert(
  !disallowedPolicy.durations && !disallowedPolicy.perMinute,
  "overlay_allowed=false suppresses overlays even for an aligned grade",
);
const unknownGradePolicy = diffOverlayPolicy({ grade: "novel", overlay_allowed: true, reason: "" });
assert(
  !unknownGradePolicy.durations && !unknownGradePolicy.perMinute,
  "an unrecognized grade fails closed: no overlay renders",
);
assert(!diffOverlayPolicy(null).durations, "a missing alignment block suppresses overlays");

// Envelope selectors over a representative diff result.
const diffSideA = {
  count: 100,
  errors: 2,
  error_rate: { numerator: 2, denominator: 100, value: 0.02 },
  traffic_share: { numerator: 100, denominator: 100, value: 1 },
  count_per_minute: { numerator: 100, denominator_minutes: 10, value_per_minute: 10 },
  duration_p50_ms: 40,
  duration_p95_ms: 120,
  duration_p99_ms: 200,
  duration_samples: 98,
  request_bytes: 1000,
  response_bytes: 4000,
};
const diffSideB = {
  ...diffSideA,
  count: 110,
  errors: 9,
  error_rate: { numerator: 9, denominator: 110, value: 0.0818 },
  count_per_minute: { numerator: 110, denominator_minutes: 10, value_per_minute: 11 },
  duration_p95_ms: 240,
};
const diffFixture = {
  type: "http_capture_diff",
  source_files: [],
  created_at: "2026-07-30T00:00:00.000Z",
  summary: {
    before_session: { session_id: "har:before", source_kind: "har_import", transactions: 100 },
    after_session: { session_id: "s-after", source_kind: "live_capture", transactions: 110 },
    before: diffSideA,
    after: diffSideB,
    delta: {
      count: 10,
      errors: 7,
      error_rate: 0.0618,
      duration_p50_ms: 0,
      duration_p95_ms: 120,
      duration_p99_ms: 0,
      request_bytes: 0,
      response_bytes: 0,
      count_per_minute: 1,
    },
    time_alignment: { grade: "aligned", overlay_allowed: true, reason: "both trusted" },
    url_template_version: 1,
    table_limit: 50,
  },
  series: {},
  tables: {
    endpoints_changed: [
      { key: "GET api.test /orders/{id}", change: "changed", before: diffSideA, after: diffSideB, delta: { count: 10 } },
    ],
    endpoints_added: [
      { key: "POST api.test /new", change: "added", before: { ...diffSideA, count: 0 }, after: diffSideB, delta: { count: 110 } },
    ],
    endpoints_removed: [],
    hosts_changed: [],
    processes_changed: [],
  },
  charts: {},
  metadata: {
    findings: [
      { severity: "warning", code: "HTTP_DIFF_ERROR_RATE_UP", message: "HTTP error rate increased" },
    ],
    http_capture_diff: {
      contract: supportedDiffContract,
      before_session: { session_id: "har:before", source_kind: "har_import", transactions: 100 },
      after_session: { session_id: "s-after", source_kind: "live_capture", transactions: 110 },
      url_template_version: 1,
      source_projection_version: 1,
      table_limit: 50,
      template_limit: 1000,
      time_alignment: { grade: "aligned", overlay_allowed: true, reason: "both trusted" },
      process_dimension: {
        available: false,
        reason: "HAR pseudo-process sessions do not provide comparable process attribution",
      },
      dimension_totals: {
        before: { transactions: 100, endpoints: 100, hosts: 100, process_available: false, cross_check_passed: true },
        after: { transactions: 110, endpoints: 110, hosts: 110, processes: 110, process_available: true, cross_check_passed: true },
      },
      store_rescanned: false,
      export_projection: "analysis_result_envelope",
      workspace_route: { supported: true, route: "http_capture_diff", method: "AnalyzeHttpCaptureDiff", result_type: "http_capture_diff" },
      finding_thresholds: { traffic_share_delta: 0.1 },
    },
  },
} as unknown as HttpCaptureDiffAnalysisResult;

const diffSummary = selectDiffSummary(diffFixture);
assert(diffSummary?.before.error_rate.denominator === 100, "diff summary keeps the explicit error-rate denominator");
assert(diffSummary?.after.count_per_minute?.denominator_minutes === 10, "per-minute rates disclose their minute denominator");
const diffEnvelope = extractDiffEnvelope(diffFixture);
assert(diffEnvelope?.store_rescanned === false, "the envelope discloses that no store was rescanned");
assert(diffEnvelope?.process_dimension.available === false, "HAR pairs disable the process dimension");
assert(
  (diffEnvelope?.process_dimension.reason ?? "").length > 0,
  "a disabled process dimension carries its reason",
);
assert(selectDiffTable(diffFixture, "endpoints_changed").length === 1, "changed endpoints table is projected");
assert(selectDiffTable(diffFixture, "endpoints_added").length === 1, "added (unmatched) endpoints table is projected");
assert(selectDiffTable(diffFixture, "hosts_changed").length === 0, "empty host table projects as empty, not undefined");
assert(diffHasChanges(diffFixture), "a fixture with table rows reports changes");
assert(selectDiffFindings(diffFixture).length === 1, "HTTP_DIFF_* findings are projected");
assert(
  resolveDiffSourceKind(diffSummary!.before_session.source_kind) === "har_import" &&
    resolveDiffSourceKind(diffSummary!.after_session.source_kind) === "live_capture",
  "session refs resolve their source kinds through the closed set",
);

// Reordered-equivalent sessions produce empty tables (backend regression);
// the renderer must show an explicit no-difference state for that shape.
const equalDiffFixture = {
  ...diffFixture,
  tables: {
    endpoints_changed: [],
    endpoints_added: [],
    endpoints_removed: [],
    hosts_changed: [],
    processes_changed: [],
  },
  metadata: { ...(diffFixture.metadata as object), findings: [] },
} as unknown as HttpCaptureDiffAnalysisResult;
assert(!diffHasChanges(equalDiffFixture), "all-empty comparison tables report no changes");
assert(!diffHasChanges(null), "a missing diff result reports no changes");

// Comparison candidates and the source-projection precondition.
const withProjection = entry("hc-1", "http_capture", {
  metadata: { http_capture_diff_source: { schema_version: 1 } } as never,
});
const withoutProjection = entry("hc-2", "http_capture", { metadata: {} as never });
const otherType = entry("al-1", "access_log", {});
const diffCandidates = diffCandidateEntries([withProjection, withoutProjection, otherType]);
assert(diffCandidates.length === 2, "only http_capture entries are comparison candidates");
assert(hasDiffSourceProjection(withProjection.result), "a projected result satisfies the source precondition");
assert(
  !hasDiffSourceProjection(withoutProjection.result),
  "a result predating the projection fails the precondition and must be re-analyzed",
);
assert(!hasDiffSourceProjection(null), "a missing result fails the projection precondition");

// Comparison lifecycle provenance: a result is only visible with the pair
// that produced it.
let diffState = httpCaptureDiffReducer(initialHttpCaptureDiffState, {
  type: "contractLoaded",
  contract: supportedDiffContract,
});
assert(diffState.contractSupported && !diffState.contractMismatch, "a supported contract enables comparison");
const mismatchState = httpCaptureDiffReducer(initialHttpCaptureDiffState, {
  type: "contractLoaded",
  contract: { ...supportedDiffContract, schema_version: 99 },
});
assert(
  !mismatchState.contractSupported && mismatchState.contractMismatch,
  "an unimplemented contract version disables comparison and flags the mismatch",
);
diffState = httpCaptureDiffReducer(diffState, { type: "setBefore", id: "hc-1" });
diffState = httpCaptureDiffReducer(diffState, { type: "setAfter", id: "hc-2" });
diffState = httpCaptureDiffReducer(diffState, { type: "compareStart" });
diffState = httpCaptureDiffReducer(diffState, {
  type: "compareSuccess",
  result: diffFixture,
  pair: { beforeId: "hc-1", afterId: "hc-2" },
});
assert(diffState.result === diffFixture && diffState.resultPair?.beforeId === "hc-1", "a matching pair stores its result");
// Race: the user changes a selection while a comparison is in flight; the
// late success must not render under the new pair.
let racedState = httpCaptureDiffReducer(diffState, { type: "compareStart" });
racedState = httpCaptureDiffReducer(racedState, { type: "setBefore", id: "hc-3" });
racedState = httpCaptureDiffReducer(racedState, {
  type: "compareSuccess",
  result: diffFixture,
  pair: { beforeId: "hc-1", afterId: "hc-2" },
});
assert(
  racedState.result === null && racedState.resultPair === null,
  "a result that raced past a selection change never renders under the new pair",
);
const changedSelection = httpCaptureDiffReducer(diffState, { type: "setAfter", id: "hc-9" });
assert(changedSelection.result === null && changedSelection.resultPair === null, "changing a selection drops the rendered result");
const swapped = httpCaptureDiffReducer(diffState, { type: "swap" });
assert(
  swapped.beforeId === "hc-2" && swapped.afterId === "hc-1" && swapped.result === null,
  "swap exchanges the pair and drops the stale result",
);
const errored = httpCaptureDiffReducer(diffState, {
  type: "compareError",
  error: { code: "HTTP_DIFF_ROUTE_UNSUPPORTED", message: "nope" },
});
assert(errored.result === null && errored.error?.code === "HTTP_DIFF_ROUTE_UNSUPPORTED", "a failed comparison drops the result");
const resetState = httpCaptureDiffReducer(diffState, { type: "reset" });
assert(
  resetState.result === null && resetState.contractSupported,
  "reset clears the comparison but keeps the adopted contract",
);

// Every diff label key must exist with a non-empty string in both locales.
const diffLabelKeys = [
  ...Object.values(DIFF_ALIGNMENT_LABEL_KEYS),
  ...Object.values(DIFF_CHANGE_LABEL_KEYS),
  ...Object.values(DIFF_RATE_UNAVAILABLE_LABEL_KEYS),
  ...Object.values(DIFF_SOURCE_KIND_LABEL_KEYS),
  "httpCaptureDiffTitle",
  "httpCaptureDiffDescription",
  "httpCaptureDiffContractMismatch",
  "httpCaptureDiffNoCandidates",
  "httpCaptureDiffBeforeLabel",
  "httpCaptureDiffAfterLabel",
  "httpCaptureDiffSelectPlaceholder",
  "httpCaptureDiffSwap",
  "httpCaptureDiffRun",
  "httpCaptureDiffNeedTwo",
  "httpCaptureDiffMissingProjection",
  "httpCaptureDiffCompareCurrent",
  "httpCaptureDiffSetBaseline",
  "httpCaptureDiffSetTarget",
  "httpCaptureDiffSlotBaseline",
  "httpCaptureDiffSlotTarget",
  "httpCaptureDiffSessionsTitle",
  "httpCaptureDiffSessionBefore",
  "httpCaptureDiffSessionAfter",
  "httpCaptureDiffTransactions",
  "httpCaptureDiffAlignmentTitle",
  "httpCaptureDiffOverlaySuppressed",
  "httpCaptureDiffOverlayPerMinuteSuppressed",
  "httpCaptureDiffSummaryTitle",
  "httpCaptureDiffDelta",
  "httpCaptureDiffMetricCount",
  "httpCaptureDiffMetricErrors",
  "httpCaptureDiffMetricErrorRate",
  "httpCaptureDiffMetricPerMinute",
  "httpCaptureDiffMetricP50",
  "httpCaptureDiffMetricP95",
  "httpCaptureDiffMetricP99",
  "httpCaptureDiffMetricRequestBytes",
  "httpCaptureDiffMetricResponseBytes",
  "httpCaptureDiffDurationSamples",
  "httpCaptureDiffDetailSamplesNote",
  "httpCaptureDiffDurationOverlayTitle",
  "httpCaptureDiffTableChangedTitle",
  "httpCaptureDiffTableAddedTitle",
  "httpCaptureDiffTableRemovedTitle",
  "httpCaptureDiffTableHostsTitle",
  "httpCaptureDiffTableProcessesTitle",
  "httpCaptureDiffProcessUnavailable",
  "httpCaptureDiffNoChanges",
  "httpCaptureDiffColKey",
  "httpCaptureDiffColChange",
  "httpCaptureDiffColCountAB",
  "httpCaptureDiffColErrorRate",
  "httpCaptureDiffColP95",
  "httpCaptureDiffRowOpen",
  "httpCaptureDiffDetailTitle",
  "httpCaptureDiffSideAbsent",
  "httpCaptureDiffTrafficShare",
  "httpCaptureDiffFindingsTitle",
  "httpCaptureDiffBoundsNote",
  "httpCaptureDiffCrossCheckFailed",
] as const;
for (const key of diffLabelKeys) {
  assert(
    typeof (messages.en as Record<string, string>)[key] === "string" &&
      (messages.en as Record<string, string>)[key]!.length > 0 &&
      typeof (messages.ko as Record<string, string>)[key] === "string" &&
      (messages.ko as Record<string, string>)[key]!.length > 0,
    `diff message key ${key} must exist in both locales`,
  );
}

// ── HTTP evidence correlation (X-RG1 / T-583) ───────────────────────

import {
  CORRELATION_ALIGNMENT_LABEL_KEYS,
  CORRELATION_CONFIDENCE_LABEL_KEYS,
  CORRELATION_MATCH_BASIS_LABEL_KEYS,
  CORRELATION_SOURCE_LABEL_KEYS,
  HTTP_CORRELATION_CONTRACT_SCHEMA_VERSION,
  accessClockComparison,
  correlationAnchorState,
  correlationCandidates,
  correlationInputsOf,
  correlationOverlayAllowed,
  correlationPrimaryAlignment,
  correlationTopNState,
  diagnosticForSource,
  extractCorrelationEnvelope,
  hasCorrelationSecondary,
  httpCorrelationReducer,
  initialHttpCorrelationState,
  isCorrelationContractSupported,
  resolveCorrelationAlignment,
  resolveCorrelationConfidence,
  resolveCorrelationMatchBasis,
  resolveCorrelationSource,
  selectCorrelationDiagnostics,
  selectCorrelationFindings,
  selectCorrelationSummary,
  selectCorrelationTable,
  sameCorrelationInputs,
} from "./httpCorrelation.js";
import type {
  HttpCorrelationAccessRow,
  HttpCorrelationSourceDiagnostic,
  HttpEvidenceCorrelationAnalysisResult,
  HttpEvidenceCorrelationContract,
} from "../bridge/types.js";

// Contract adoption mirrors the diff/R8 pattern and additionally rejects a
// contract that would permit causal claims.
const supportedCorrelationContract: HttpEvidenceCorrelationContract = {
  schema_version: HTTP_CORRELATION_CONTRACT_SCHEMA_VERSION,
  result_type: "http_evidence_correlation",
  http_result_type: "http_capture",
  optional_result_types: ["profile_evidence", "jennifer_profile", "access_log"],
  alignment_grades: ["aligned", "duration_only", "none"],
  confidence_grades: ["high", "medium", "low", "none"],
  default_top_n: 50,
  max_top_n: 500,
  default_time_tolerance_ms: 1000,
  max_time_tolerance_ms: 60000,
  analyze_method: "AnalyzeHttpEvidenceCorrelation",
  requires_profile_wall_clock_anchor: true,
  store_or_file_rescan: false,
  causal_claims_allowed: false,
};
assert(isCorrelationContractSupported(supportedCorrelationContract), "the v1 correlation contract is supported");
assert(
  !isCorrelationContractSupported({ ...supportedCorrelationContract, schema_version: 2 }),
  "an unknown correlation contract version is rejected",
);
assert(
  !isCorrelationContractSupported({ ...supportedCorrelationContract, causal_claims_allowed: true }),
  "a contract permitting causal claims is rejected outright",
);
assert(!isCorrelationContractSupported(null), "a missing correlation contract is not supported");
// X-RG1 O3: the renderer states "no source file or capture store was reopened"
// and renders grades/confidence through closed token sets, so a contract that
// contradicts either must be refused rather than adopted behind that wording.
assert(
  !isCorrelationContractSupported({ ...supportedCorrelationContract, store_or_file_rescan: true }),
  "a contract that reopens sources is rejected, not adopted behind the no-rescan disclosure",
);
assert(
  !isCorrelationContractSupported({
    ...supportedCorrelationContract,
    alignment_grades: ["aligned", "duration_only", "none", "probably_aligned"],
  }),
  "a contract advertising a grade the renderer cannot name is rejected",
);
assert(
  !isCorrelationContractSupported({
    ...supportedCorrelationContract,
    confidence_grades: ["high", "medium", "low", "none", "certain"],
  }),
  "a contract advertising an unknown confidence token is rejected",
);

// X-RG1 O5: top-N is bounded by the adopted contract before the run.
assert(correlationTopNState(null, supportedCorrelationContract) === "empty", "an empty top-N uses the default");
assert(correlationTopNState(50, supportedCorrelationContract) === "valid", "an in-range top-N is valid");
assert(correlationTopNState(500, supportedCorrelationContract) === "valid", "the contract maximum is valid");
assert(correlationTopNState(501, supportedCorrelationContract) === "invalid", "top-N above max_top_n is invalid");
assert(correlationTopNState(0, supportedCorrelationContract) === "invalid", "a zero top-N is invalid");
assert(correlationTopNState(12.5, supportedCorrelationContract) === "invalid", "a fractional top-N is invalid");

// Closed token sets fail to `unknown` instead of leaking wire values.
assert(resolveCorrelationAlignment("aligned") === "aligned", "aligned correlation grade resolves");
assert(resolveCorrelationAlignment("brand_new") === "unknown", "unrecognized correlation grade resolves to unknown");
assert(resolveCorrelationConfidence("medium") === "medium", "medium confidence resolves");
assert(resolveCorrelationConfidence("certain") === "unknown", "unrecognized confidence resolves to unknown");
assert(resolveCorrelationMatchBasis("request_id") === "request_id", "request_id basis resolves");
assert(
  resolveCorrelationMatchBasis("explicit_wall_clock_anchor+time_overlap") ===
    "explicit_wall_clock_anchor+time_overlap",
  "anchor+overlap basis resolves",
);
assert(resolveCorrelationMatchBasis("vibes") === "unknown", "unrecognized basis resolves to unknown");
assert(resolveCorrelationSource("jennifer_profile") === "jennifer_profile", "jennifer source resolves");
assert(resolveCorrelationSource("mystery") === "unknown", "unrecognized source resolves to unknown");

// Overlay gating: only a backend-certified aligned overlay may render.
const alignedDiag: HttpCorrelationSourceDiagnostic = {
  source: "profile_evidence", result_type: "profile_evidence",
  alignment_grade: "aligned", confidence: "high", overlay_allowed: true, reason: "",
  input_rows: 1, rows_used: 1, candidate_rows: 1, output_rows: 1,
  source_truncated: false, output_truncated: false,
};
assert(correlationOverlayAllowed(alignedDiag), "aligned + overlay_allowed enables the overlay");
assert(
  !correlationOverlayAllowed({ ...alignedDiag, overlay_allowed: false }),
  "overlay_allowed=false suppresses the overlay even when aligned",
);
assert(
  !correlationOverlayAllowed({ ...alignedDiag, alignment_grade: "duration_only" }),
  "duration_only never renders a time overlay",
);
assert(
  !correlationOverlayAllowed({ ...alignedDiag, alignment_grade: "novel", overlay_allowed: true }),
  "an unrecognized grade fails closed: no overlay renders",
);
assert(!correlationOverlayAllowed(null), "a missing diagnostic suppresses the overlay");

// X-RG1 B1: a request-ID pairing identifies the same request without comparing
// clocks. The renderer must show that as an unavailable delta — never as a
// measured 0.0ms — and must fail closed when a row is internally inconsistent.
const accessRowBase: HttpCorrelationAccessRow = {
  http_id: "tx-9", endpoint: "GET api.test /orders/{id}", access_uri: "/orders/42",
  request_id: "rid-42", http_duration_ms: 120, server_response_ms: 90, outside_server_ms: 30,
  clock_compared: false, alignment_grade: "duration_only", confidence: "high",
  match_basis: "request_id", causal_claim_allowed: false,
  timestamp_delta_unavailable_reason:
    "request identity matched, but both observations did not provide parseable absolute timestamps",
};
const identityOnlyClock = accessClockComparison(accessRowBase);
assert(!identityOnlyClock.compared, "a request-ID-only pairing reports no clock comparison");
assert(identityOnlyClock.deltaMs === null, "an unmeasured pairing exposes no timestamp delta to render");
assert(
  identityOnlyClock.reason?.includes("request identity") === true,
  "the engine's unavailable reason reaches the renderer",
);
const measuredClock = accessClockComparison({
  ...accessRowBase, clock_compared: true, timestamp_delta_ms: 12.5,
  alignment_grade: "aligned", timestamp_delta_unavailable_reason: undefined,
});
assert(
  measuredClock.compared && measuredClock.deltaMs === 12.5,
  "a measured in-tolerance delta renders as measured",
);
assert(
  !accessClockComparison({ ...accessRowBase, timestamp_delta_ms: 0 }).compared,
  "a delta without the comparison flag fails closed to unavailable",
);
assert(
  !accessClockComparison({ ...accessRowBase, clock_compared: true }).compared,
  "the comparison flag alone, with no delta, still renders as unavailable",
);
assert(
  !accessClockComparison({
    ...accessRowBase, clock_compared: true, timestamp_delta_ms: Number.NaN,
  }).compared,
  "a non-finite delta never renders as a measured alignment",
);

// Anchor validation is advisory but must reject non-RFC3339 text up front.
assert(correlationAnchorState("") === "empty", "an empty anchor is empty, not invalid");
assert(
  correlationAnchorState("2026-07-30T12:00:00.000Z") === "valid",
  "an RFC3339 UTC anchor validates",
);
assert(
  correlationAnchorState("2026-07-30T12:00:00+09:00") === "valid",
  "an RFC3339 offset anchor validates",
);
assert(correlationAnchorState("yesterday noon") === "invalid", "prose anchors are invalid");
assert(correlationAnchorState("2026-07-30 12:00:00") === "invalid", "space-separated timestamps are invalid");

// Selectors over a representative correlation result.
const correlationFixture = {
  type: "http_evidence_correlation",
  source_files: [],
  created_at: "2026-07-30T00:00:00.000Z",
  summary: {
    http_transaction_rows: 3,
    profile_overlap_count: 1,
    jennifer_check_count: 1,
    access_log_match_count: 0,
    aligned_source_count: 1,
    duration_only_source_count: 1,
    incompatible_source_count: 1,
    causal_claims_allowed: false,
    store_or_file_rescanned: false,
  },
  series: {},
  tables: {
    http_profile_overlaps: [
      {
        http_id: "tx-1", endpoint: "GET api.test /orders/{id}",
        http_started_at: "2026-07-30T00:00:00.000Z", http_ended_at: "2026-07-30T00:00:00.200Z",
        cpu_stack: "app.js;renderList", cpu_started_at: "2026-07-30T00:00:00.050Z",
        cpu_ended_at: "2026-07-30T00:00:00.150Z",
        overlap_ms: 100, overlap_ratio: 1, alignment_grade: "aligned", confidence: "high",
        match_basis: "explicit_wall_clock_anchor+time_overlap", causal_claim_allowed: false,
      },
    ],
    jennifer_network_gap_checks: [
      {
        http_id: "tx-2", endpoint: "GET api.test /users/{id}", jennifer_guid: "g-1",
        target_host: "api.test", external_call_elapsed_ms: 120, http_duration_ms: 110,
        duration_delta_ms: 10, jennifer_network_gap_ms: 40, observed_network_phase_count: 0,
        network_gap_unavailable_reason: "HTTP transaction has no known DNS/connect/TLS/send/receive timing phases",
        alignment_grade: "duration_only", confidence: "medium",
        match_basis: "target_host+nearest_duration", causal_claim_allowed: false,
      },
    ],
    access_log_matches: [],
    alignment_diagnostics: [
      {
        source: "http_capture", result_type: "http_capture", alignment_grade: "aligned",
        confidence: "high", overlay_allowed: false, reason: "primary observation source",
        input_rows: 3, rows_used: 3, candidate_rows: 0, output_rows: 0,
        source_truncated: false, output_truncated: false,
      },
      alignedDiag,
      {
        source: "jennifer_profile", result_type: "jennifer_profile", alignment_grade: "duration_only",
        confidence: "low", overlay_allowed: false,
        reason: "Jennifer edge timestamps are ms-since-midnight without a date/offset",
        input_rows: 5, rows_used: 5, candidate_rows: 1, output_rows: 1,
        source_truncated: false, output_truncated: false,
      },
    ],
  },
  charts: {},
  metadata: {
    findings: [
      {
        severity: "info", code: "HTTP_CORRELATION_PROFILE_CLOCK_INCOMPATIBLE",
        message: "CPU profile timestamps were not overlaid because no compatible wall-clock anchor was available.",
      },
    ],
    http_evidence_correlation: {
      contract: supportedCorrelationContract,
      top_n: 50, time_tolerance_ms: 1000,
      input_projection: "analysis_result_envelope",
      store_or_file_rescanned: false, causal_claims_allowed: false,
      profile_wall_clock_anchor: true,
    },
    source_provenance: [
      { result_type: "http_capture", created_at: "2026-07-30T00:00:00.000Z", schema_version: "0.1.0" },
    ],
  },
} as unknown as HttpEvidenceCorrelationAnalysisResult;

const correlationSummary = selectCorrelationSummary(correlationFixture);
assert(correlationSummary?.causal_claims_allowed === false, "the summary discloses that causal claims are forbidden");
const correlationEnvelope = extractCorrelationEnvelope(correlationFixture);
assert(correlationEnvelope?.store_or_file_rescanned === false, "the envelope discloses no source rescan");
const correlationDiags = selectCorrelationDiagnostics(correlationFixture);
assert(correlationDiags.length === 3, "alignment diagnostics rows are projected");
const profileDiagRow = diagnosticForSource(correlationDiags, "profile_evidence");
assert(profileDiagRow !== null && correlationOverlayAllowed(profileDiagRow), "the profile diagnostic gates its overlay");
const jenniferDiagRow = diagnosticForSource(correlationDiags, "jennifer_profile");
assert(
  jenniferDiagRow !== null && !correlationOverlayAllowed(jenniferDiagRow),
  "the duration-only Jennifer diagnostic never overlays",
);
assert(
  selectCorrelationTable(correlationFixture, "http_profile_overlaps").length === 1,
  "profile overlap rows are projected",
);
assert(
  selectCorrelationTable(correlationFixture, "access_log_matches").length === 0,
  "an empty access table projects as empty, not undefined",
);
assert(selectCorrelationFindings(correlationFixture).length === 1, "correlation findings are projected");
// The summary's alignment counters exclude the primary source, so the renderer
// reads the HTTP timeline's own grade separately (X-RG1 B1 tile disclosure).
assert(
  correlationPrimaryAlignment(correlationDiags) === "aligned",
  "the primary HTTP timeline grade is readable next to the secondary counters",
);
assert(
  correlationPrimaryAlignment([
    { ...correlationDiags[0], alignment_grade: "duration_only" },
    ...correlationDiags.slice(1),
  ]) === "duration_only",
  "a duration-only primary timeline is reported as such, not hidden behind the counters",
);
assert(
  correlationPrimaryAlignment([alignedDiag]) === "unknown",
  "a missing primary diagnostic fails closed to unknown",
);

// Candidate filtering per slot.
const correlationEntries = [
  entry("hc-corr", "http_capture", {}),
  entry("pe-corr", "profile_evidence", {}),
  entry("jp-corr", "jennifer_profile", {}),
  entry("al-corr", "access_log", {}),
  entry("gc-corr", "gc_log", {}),
];
assert(correlationCandidates(correlationEntries, "http").length === 1, "http slot accepts only http_capture");
assert(correlationCandidates(correlationEntries, "profile").length === 1, "profile slot accepts only profile_evidence");
assert(correlationCandidates(correlationEntries, "jennifer").length === 1, "jennifer slot accepts only jennifer_profile");
assert(correlationCandidates(correlationEntries, "accessLog").length === 1, "accessLog slot accepts only access_log");

// Lifecycle provenance: any input change drops the rendered result; a raced
// success never renders under changed inputs.
let corrState = httpCorrelationReducer(initialHttpCorrelationState, {
  type: "contractLoaded",
  contract: supportedCorrelationContract,
});
assert(corrState.contractSupported, "a supported correlation contract enables the run");
corrState = httpCorrelationReducer(corrState, { type: "setSlot", slot: "http", id: "hc-corr" });
assert(!hasCorrelationSecondary(correlationInputsOf(corrState)), "http alone is not enough to run");
corrState = httpCorrelationReducer(corrState, { type: "setSlot", slot: "profile", id: "pe-corr" });
assert(hasCorrelationSecondary(correlationInputsOf(corrState)), "one secondary source satisfies the minimum");
corrState = httpCorrelationReducer(corrState, { type: "setAnchor", anchor: "2026-07-30T12:00:00Z" });
const corrInputs = correlationInputsOf(corrState);
corrState = httpCorrelationReducer(corrState, { type: "runStart" });
corrState = httpCorrelationReducer(corrState, {
  type: "runSuccess",
  result: correlationFixture,
  inputs: corrInputs,
});
assert(corrState.result === correlationFixture, "a matching input snapshot stores its result");
const corrChanged = httpCorrelationReducer(corrState, { type: "setAnchor", anchor: "" });
assert(corrChanged.result === null, "changing the anchor drops the rendered correlation");
const corrTopNChanged = httpCorrelationReducer(corrState, { type: "setTopN", topN: 25 });
assert(
  corrTopNChanged.result === null && corrTopNChanged.topN === 25,
  "changing top-N drops the rendered correlation: the result is only shown with the inputs that produced it",
);
assert(
  !sameCorrelationInputs(corrInputs, correlationInputsOf(corrTopNChanged)),
  "top-N is part of the correlation input identity",
);
let corrRaced = httpCorrelationReducer(corrState, { type: "runStart" });
corrRaced = httpCorrelationReducer(corrRaced, { type: "setSlot", slot: "jennifer", id: "jp-corr" });
corrRaced = httpCorrelationReducer(corrRaced, {
  type: "runSuccess",
  result: correlationFixture,
  inputs: corrInputs,
});
assert(
  corrRaced.result === null,
  "a correlation that raced past an input change never renders under the new inputs",
);
const corrErr = httpCorrelationReducer(corrState, {
  type: "runError",
  error: { code: "HTTP_CORRELATION_FAILED", message: "boom" },
});
assert(corrErr.result === null && corrErr.error?.code === "HTTP_CORRELATION_FAILED", "a failed run drops the result");
const corrReset = httpCorrelationReducer(corrState, { type: "reset" });
assert(corrReset.result === null && corrReset.contractSupported, "reset keeps the adopted correlation contract");

// Every correlation label key must exist non-empty in both locales.
const correlationLabelKeys = [
  ...Object.values(CORRELATION_ALIGNMENT_LABEL_KEYS),
  ...Object.values(CORRELATION_CONFIDENCE_LABEL_KEYS),
  ...Object.values(CORRELATION_MATCH_BASIS_LABEL_KEYS),
  ...Object.values(CORRELATION_SOURCE_LABEL_KEYS),
  "httpCorrelationTitle",
  "httpCorrelationDescription",
  "httpCorrelationContractMismatch",
  "httpCorrelationNoHttp",
  "httpCorrelationSlotHttp",
  "httpCorrelationSlotProfile",
  "httpCorrelationSlotJennifer",
  "httpCorrelationSlotAccessLog",
  "httpCorrelationSlotEmpty",
  "httpCorrelationSlotPlaceholder",
  "httpCorrelationAnchorLabel",
  "httpCorrelationAnchorHint",
  "httpCorrelationAnchorInvalid",
  "httpCorrelationToleranceLabel",
  "httpCorrelationTopNLabel",
  "httpCorrelationTopNInvalid",
  "httpCorrelationRun",
  "httpCorrelationNeedSecondary",
  "httpCorrelationUseAsHttp",
  "httpCorrelationNoCausalityNote",
  "httpCorrelationMetricHttpRows",
  "httpCorrelationMetricProfileOverlaps",
  "httpCorrelationMetricJenniferChecks",
  "httpCorrelationMetricAccessMatches",
  "httpCorrelationMetricAligned",
  "httpCorrelationMetricDurationOnly",
  "httpCorrelationMetricIncompatible",
  "httpCorrelationPrimaryTimelineNote",
  "httpCorrelationDiagnosticsTitle",
  "httpCorrelationConfidenceLabel",
  "httpCorrelationOverlayEnabled",
  "httpCorrelationOverlaySuppressed",
  "httpCorrelationRowsUsed",
  "httpCorrelationOutputRows",
  "httpCorrelationSourceTruncated",
  "httpCorrelationOutputTruncated",
  "httpCorrelationProfileTitle",
  "httpCorrelationProfileSuppressed",
  "httpCorrelationJenniferTitle",
  "httpCorrelationJenniferDurationOnlyNote",
  "httpCorrelationAccessTitle",
  "httpCorrelationAccessIdentityOnlyNote",
  "httpCorrelationClockCompared",
  "httpCorrelationClockNotCompared",
  "httpCorrelationNoMatches",
  "httpCorrelationColEndpoint",
  "httpCorrelationColStack",
  "httpCorrelationColOverlapMs",
  "httpCorrelationColOverlapRatio",
  "httpCorrelationColConfidence",
  "httpCorrelationColTargetHost",
  "httpCorrelationColDurations",
  "httpCorrelationColGapDelta",
  "httpCorrelationColClientServer",
  "httpCorrelationColOutsideServer",
  "httpCorrelationColBasis",
  "httpCorrelationFindingsTitle",
  "httpCorrelationBoundsNote",
  "httpCorrelationRowOpen",
  "httpCorrelationDetailTitle",
  "httpCorrelationDetailAlignment",
  "httpCorrelationDetailJenniferGap",
  "httpCorrelationDetailObservedNetwork",
  "httpCorrelationDetailObservedUnavailable",
  "httpCorrelationDetailAccessUri",
  "httpCorrelationDetailRequestId",
  "httpCorrelationDetailTimestampDelta",
  "httpCorrelationDetailClockBasis",
  "httpCorrelationOverlayTitle",
  "httpCorrelationOverlayUnavailable",
] as const;
for (const key of correlationLabelKeys) {
  assert(
    typeof (messages.en as Record<string, string>)[key] === "string" &&
      (messages.en as Record<string, string>)[key]!.length > 0 &&
      typeof (messages.ko as Record<string, string>)[key] === "string" &&
      (messages.ko as Record<string, string>)[key]!.length > 0,
    `correlation message key ${key} must exist in both locales`,
  );
}
