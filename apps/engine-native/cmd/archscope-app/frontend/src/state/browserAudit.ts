// ─────────────────────────────────────────────────────────────────────
// [한글] state/browserAudit.ts — Lighthouse 브라우저 감사 페이지의
// 순수(DOM 비의존) 파생 로직.
//
// 책임/목적:
//   - engine `browser_audit_evidence` 결과에서 점수 출처 공시,
//     카테고리 점수, Core Web Vitals, 리소스 분포, 감사/네트워크 표,
//     발견 사항, 파서 진단을 계산.
//   - 가져온 Lighthouse 점수는 그대로 보존되며 ArchScope 가 재계산하지
//     않는다는 계약(`scores_recomputed === false`)을 UI 에 전달.
//   - React/JSDOM 없이 Node 기반 regression 하니스(state/regression.test.ts)
//     로 검증 가능하도록 로직을 페이지 밖으로 분리 (다른 analyzer state
//     모듈과 동일 패턴).
//
// 의존성 주의: bridge/types 의 와이어 형식만 참조하며 런타임 코드 없음.
// 백엔드 계약: apps/engine-native/internal/analyzers/lighthouse/analyzer.go.
// ─────────────────────────────────────────────────────────────────────

import type {
  BrowserAuditAnalysisResult,
  BrowserAuditContract,
  DiagnosticSample,
  ParserDiagnostics,
} from "../bridge/types";

type AnyResult = BrowserAuditAnalysisResult | null | undefined;
type Row = Record<string, unknown>;

function record(value: unknown): Row | undefined {
  return value && typeof value === "object" ? (value as Row) : undefined;
}

function rows(value: unknown): Row[] {
  return Array.isArray(value) ? (value as Row[]) : [];
}

/** selectBrowserAuditSummary returns the `summary` object or an empty record. */
export function selectBrowserAuditSummary(result: AnyResult): Row {
  return record(result?.summary) ?? {};
}

/** selectScoreProvenance surfaces the imported-score disclosure so the UI can
 * state, honestly, that Lighthouse scores are preserved and never recomputed. */
export function selectScoreProvenance(result: AnyResult): {
  scoreSource: string;
  scoreDisclosure: string;
  scoresRecomputed: boolean;
} {
  const summary = selectBrowserAuditSummary(result);
  const contract = selectBrowserAuditContract(result);
  return {
    scoreSource: String(summary.score_source ?? contract?.score_source ?? ""),
    scoreDisclosure: String(summary.score_disclosure ?? contract?.score_disclosure ?? ""),
    // Default to the honest `false` — the engine never recomputes imported
    // scores, so the UI must not imply otherwise on a malformed payload.
    scoresRecomputed: summary.scores_recomputed === true || contract?.scores_recomputed === true,
  };
}

/** selectBrowserAuditContract returns the frozen versioned view/evidence/export
 * contract the engine attaches under metadata.browser_audit_contract. */
export function selectBrowserAuditContract(result: AnyResult): BrowserAuditContract | null {
  const contract = record(result?.metadata)?.browser_audit_contract;
  return contract ? (contract as unknown as BrowserAuditContract) : null;
}

/** selectCategoryScores returns series.category_scores (id/title/score_pct). */
export function selectCategoryScores(result: AnyResult): Row[] {
  return rows(record(result?.series)?.category_scores);
}

/** selectCoreMetrics returns series.core_metrics (Core Web Vitals rows). */
export function selectCoreMetrics(result: AnyResult): Row[] {
  return rows(record(result?.series)?.core_metrics);
}

/** selectResourceDistribution returns series.resource_type_distribution. */
export function selectResourceDistribution(result: AnyResult): Row[] {
  return rows(record(result?.series)?.resource_type_distribution);
}

/** selectAuditRows returns the bounded tables.audits rows. */
export function selectAuditRows(result: AnyResult): Row[] {
  return rows(record(result?.tables)?.audits);
}

/** selectNetworkRequests returns the bounded tables.network_requests rows. */
export function selectNetworkRequests(result: AnyResult): Row[] {
  return rows(record(result?.tables)?.network_requests);
}

/** selectResourceSummary returns tables.resource_summary rows. */
export function selectResourceSummary(result: AnyResult): Row[] {
  return rows(record(result?.tables)?.resource_summary);
}

/** selectBrowserAuditFindings returns metadata.findings, the deterministic
 * engine findings that seed Evidence Board capture. */
export function selectBrowserAuditFindings(result: AnyResult): Row[] {
  return rows(record(result?.metadata)?.findings);
}

/** extractBrowserAuditDiagnostics returns metadata.diagnostics or null. */
export function extractBrowserAuditDiagnostics(result: AnyResult): ParserDiagnostics | null {
  const diags = record(result?.metadata)?.diagnostics;
  if (!diags || typeof diags !== "object") return null;
  return diags as unknown as ParserDiagnostics;
}

/** browserAuditDiagnosticIssueCount is warnings + errors, for a header badge. */
export function browserAuditDiagnosticIssueCount(diags: ParserDiagnostics | null): number {
  if (!diags) return 0;
  return (diags.warning_count ?? 0) + (diags.error_count ?? 0);
}

/** diagnosticSampleCodes collects emitted warning/error reason codes. */
export function browserAuditDiagnosticCodes(diags: ParserDiagnostics | null): Set<string> {
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

/** scorePctBand buckets a 0-100 score into good/average/poor so the UI can
 * color category and audit rows consistently with the Lighthouse legend. */
export type ScoreBand = "good" | "average" | "poor" | "unknown";

export function scorePctBand(pct: number | null | undefined): ScoreBand {
  if (pct == null || Number.isNaN(pct)) return "unknown";
  if (pct >= 90) return "good";
  if (pct >= 50) return "average";
  return "poor";
}

/** scoreToPct normalizes a 0-1 Lighthouse score into a 0-100 percentage,
 * preserving null for not-applicable audits. Uses the report's own numbers. */
export function scoreToPct(score: unknown): number | null {
  if (typeof score !== "number" || Number.isNaN(score)) return null;
  return Math.round(score * 1000) / 10;
}

/** buildBrowserAuditEvidence maps an engine finding into the Evidence Board
 * card shape, preserving the stable `source_ref` the backend contract froze. */
export function buildBrowserAuditEvidence(
  finding: Row,
  sourceFile: string | undefined,
): {
  analyzer: string;
  source_kind: "finding";
  title: string;
  summary?: string;
  severity?: string;
  source_file?: string;
  source_ref?: string;
  payload: Row;
} {
  const evidence = record(finding.evidence);
  const sourceRef = finding.source_ref ?? evidence?.source_ref;
  return {
    analyzer: "browser_audit",
    source_kind: "finding",
    title: String(finding.code ?? finding.title ?? "LIGHTHOUSE_FINDING"),
    summary: finding.message ? String(finding.message) : undefined,
    severity: finding.severity ? String(finding.severity) : undefined,
    source_file: sourceFile,
    source_ref: sourceRef != null ? String(sourceRef) : undefined,
    payload: finding,
  };
}
