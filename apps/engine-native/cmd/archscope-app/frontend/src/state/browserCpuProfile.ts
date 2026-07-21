// ─────────────────────────────────────────────────────────────────────
// [한글] state/browserCpuProfile.ts — Browser CPU 프로파일 페이지의
// 순수(DOM 비의존) 파생 로직.
//
// 책임/목적:
//   - engine `profile_evidence` 결과에서 플레임그래프 노드, 파서 진단,
//     샘플 CPU run, 타임라인 억제 사유를 계산.
//   - React/JSDOM 없이 Node 기반 regression 하니스(state/regression.test.ts)
//     로 검증 가능하도록 로직을 페이지 밖으로 분리 (다른 analyzer state
//     모듈과 동일 패턴).
//
// 의존성 주의: bridge/types 의 와이어 형식만 참조하며 런타임 코드 없음.
// ─────────────────────────────────────────────────────────────────────

import type {
  DiagnosticSample,
  ParserDiagnostics,
  ProfileEvidenceAnalysisResult,
} from "../bridge/types";
import type { FlameGraphNode } from "../components/CanvasFlameGraph";

// RawFlameNode mirrors internal/profiler.FlameNode as serialized over IPC
// (see apps/engine-native/internal/profiler/types.go). `samples` is the
// cumulative weight; CanvasFlameGraph sums leaves only, matching the
// ProfilerAnalyzerPage adapter.
type RawFlameNode = {
  name?: string;
  samples?: number;
  category?: string | null;
  color?: string | null;
  children?: RawFlameNode[] | null;
};

/** adaptProfileFlameNode converts an engine flame node into the shape
 * CanvasFlameGraph consumes. Returns null when the node is absent. */
export function adaptProfileFlameNode(node: unknown): FlameGraphNode | null {
  if (!node || typeof node !== "object") return null;
  const raw = node as RawFlameNode;
  const children = (raw.children ?? [])
    .map((child) => adaptProfileFlameNode(child))
    .filter((child): child is FlameGraphNode => child !== null);
  return {
    name: raw.name ?? "",
    value: raw.samples ?? 0,
    category: raw.category ?? null,
    color: raw.color ?? null,
    children,
  };
}

/** extractProfileFlamegraph pulls charts.flamegraph and adapts it. */
export function extractProfileFlamegraph(
  result: ProfileEvidenceAnalysisResult | null,
): FlameGraphNode | null {
  const charts = result?.charts as Record<string, unknown> | undefined;
  return adaptProfileFlameNode(charts?.flamegraph);
}

/** extractProfileDiagnostics returns metadata.diagnostics (ParserDiagnostics)
 * or null. The engine attaches the same envelope every other analyzer uses. */
export function extractProfileDiagnostics(
  result: ProfileEvidenceAnalysisResult | null,
): ParserDiagnostics | null {
  const diags = (result?.metadata as Record<string, unknown> | undefined)?.diagnostics;
  if (!diags || typeof diags !== "object") return null;
  return diags as unknown as ParserDiagnostics;
}

/** profileDiagnosticIssueCount is warnings + errors, for a header badge. */
export function profileDiagnosticIssueCount(diags: ParserDiagnostics | null): number {
  if (!diags) return 0;
  return (diags.warning_count ?? 0) + (diags.error_count ?? 0);
}

/** diagnosticCodes collects every emitted warning/error reason code. */
export function diagnosticCodes(diags: ParserDiagnostics | null): Set<string> {
  const codes = new Set<string>();
  if (!diags) return codes;
  const push = (samples: DiagnosticSample[] | undefined) => {
    for (const sample of samples ?? []) {
      if (sample?.reason) codes.add(sample.reason);
    }
  };
  push(diags.warnings);
  push(diags.errors);
  return codes;
}

export function selectSampledRuns(
  result: ProfileEvidenceAnalysisResult | null,
): Array<Record<string, unknown>> {
  const tables = result?.tables as Record<string, unknown> | undefined;
  return (tables?.cpu_sample_runs as Array<Record<string, unknown>> | undefined) ?? [];
}

export function selectTopChildFrames(
  result: ProfileEvidenceAnalysisResult | null,
): Array<Record<string, unknown>> {
  const tables = result?.tables as Record<string, unknown> | undefined;
  return (tables?.top_child_frames as Array<Record<string, unknown>> | undefined) ?? [];
}

export function selectParserMetadata(
  result: ProfileEvidenceAnalysisResult | null,
): Record<string, unknown> | undefined {
  return (result?.metadata as Record<string, unknown> | undefined)?.parser_metadata as
    | Record<string, unknown>
    | undefined;
}

export type PartialResultInfo = {
  partial: boolean;
  fromSamples: number;
  toSamples: number;
};

export function selectPartialResult(
  result: ProfileEvidenceAnalysisResult | null,
): PartialResultInfo {
  const meta = selectParserMetadata(result);
  return {
    partial: meta?.partial_result === true,
    fromSamples: Number(meta?.downsampled_from_samples ?? 0),
    toSamples: Number(meta?.downsampled_to_samples ?? 0),
  };
}

// TimelineEmptyReason names *why* the sampled-run timeline is empty so the
// UI can explain it instead of rendering a silent empty table. Ordered by
// specificity — a hitCount-only profile makes no time claim at all, a
// downsampled one suppresses runs to keep totals honest.
export type TimelineEmptyReason =
  | "hitcount_only"
  | "downsampled"
  | "empty"
  | null;

export type TimelineState = {
  valueUnit: string;
  hasRuns: boolean;
  /** true when microsecond data exists but runs were intentionally withheld. */
  suppressed: boolean;
  reason: TimelineEmptyReason;
};

export function describeTimelineState(
  result: ProfileEvidenceAnalysisResult | null,
): TimelineState {
  const summary = (result?.summary as Record<string, unknown> | undefined) ?? {};
  const valueUnit = String(summary.value_unit ?? "");
  const runs = selectSampledRuns(result);
  const hasRuns = runs.length > 0;
  const codes = diagnosticCodes(extractProfileDiagnostics(result));

  let reason: TimelineEmptyReason = null;
  if (!hasRuns) {
    if (codes.has("PROFILE_HITCOUNT_ONLY") || valueUnit === "samples") {
      reason = "hitcount_only";
    } else if (
      codes.has("TIMELINE_SUPPRESSED_DOWNSAMPLED") ||
      codes.has("PROFILE_DOWNSAMPLED")
    ) {
      reason = "downsampled";
    } else if (result) {
      reason = "empty";
    }
  }

  return {
    valueUnit,
    hasRuns,
    suppressed: !hasRuns && (reason === "hitcount_only" || reason === "downsampled"),
    reason,
  };
}
