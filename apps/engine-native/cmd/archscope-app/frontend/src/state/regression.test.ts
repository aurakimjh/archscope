import type { AnalysisWorkspaceEntry, WorkspaceAnalysisResult } from "./analysisWorkspace.js";
import { evaluateAiInterpretation, extractAiInterpretation } from "./aiInterpretation.js";
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

const inventory = buildGoldenSignalInventory([trace, jennifer]);
const dependencyErrorRate = inventory.signals.find((signal) => signal.name === "Dependency error rate");
assert(dependencyErrorRate?.value === 20, "trace-import error_rate fractions should normalize to percent");
const trafficMetric = buildSliMetricModel(inventory).metrics.find((metric) =>
  metric.metric_key.includes("service_edge_traffic"),
);
assert(trafficMetric?.value === 10, "equivalent Trace/Jennifer edge traffic should not double count");

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
