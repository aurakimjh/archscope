// ─────────────────────────────────────────────────────────────────────
// [한글] state/httpCorrelation.ts — HTTP 증거 교차 분석(X-RG1 / T-583)의
// 순수(DOM 비의존) 파생 로직 + 페이지 간 공유 스토어.
//
// 책임/목적:
//   - engine `http_evidence_correlation` 결과에서 요약, per-source 정렬
//     진단, 세 매치 테이블, provenance, findings 를 계산.
//   - 오버레이 게이트: 각 secondary source 의 `alignment_diagnostics` 행이
//     주는 `overlay_allowed`+grade 판정을 그대로 따르고, 인식하지 못하는
//     등급은 fail-closed 로 억제한다. renderer 는 시계 호환성을 절대
//     스스로 판정하지 않는다.
//   - 인과관계 금지: 모든 행이 `causal_claim_allowed: false` 를 갖는 시간
//     상관 증거이며, UI 문구도 인과 주장으로 읽히지 않도록 닫힌 라벨을
//     사용한다.
//   - 소스 선택/실행 lifecycle 은 순수 reducer + module store 로 관리해
//     HttpCapturePage 와 Analysis Workspace 가 같은 선택을 공유한다.
//
// 의존성 주의: bridge/types 의 와이어 형식만 참조. Wails 호출은 컴포넌트
// 계층에서 수행한다 (httpCaptureDiff 패턴과 동일).
// ─────────────────────────────────────────────────────────────────────

import { useSyncExternalStore } from "react";

import type { MessageKey } from "../i18n/messages";
import type {
  HttpCorrelationAccessRow,
  HttpCorrelationEnvelope,
  HttpCorrelationProvenanceRow,
  HttpCorrelationSourceDiagnostic,
  HttpCorrelationSummary,
  HttpCaptureFinding,
  HttpEvidenceCorrelationAnalysisResult,
  HttpEvidenceCorrelationContract,
} from "../bridge/types";

type CorrelationResult = HttpEvidenceCorrelationAnalysisResult | null;

/** The renderer implements exactly this correlation contract version. */
export const HTTP_CORRELATION_CONTRACT_SCHEMA_VERSION = 1;
export const HTTP_CORRELATION_RESULT_TYPE = "http_evidence_correlation";

/** Workspace result types accepted per source slot. */
export const CORRELATION_SLOT_RESULT_TYPES = {
  http: "http_capture",
  profile: "profile_evidence",
  jennifer: "jennifer_profile",
  accessLog: "access_log",
} as const;

export type CorrelationSlot = keyof typeof CORRELATION_SLOT_RESULT_TYPES;

// ── Closed token sets ───────────────────────────────────────────────

export type CorrelationAlignmentGrade = "aligned" | "duration_only" | "none" | "unknown";

const CORRELATION_ALIGNMENT_GRADES = new Set<CorrelationAlignmentGrade>([
  "aligned",
  "duration_only",
  "none",
]);

export function resolveCorrelationAlignment(value: string): CorrelationAlignmentGrade {
  const token = String(value ?? "").trim() as CorrelationAlignmentGrade;
  return CORRELATION_ALIGNMENT_GRADES.has(token) ? token : "unknown";
}

export const CORRELATION_ALIGNMENT_LABEL_KEYS: Record<CorrelationAlignmentGrade, MessageKey> = {
  aligned: "httpCorrelationAlignmentAligned",
  duration_only: "httpCorrelationAlignmentDurationOnly",
  none: "httpCorrelationAlignmentNone",
  unknown: "httpCorrelationAlignmentUnknown",
};

export type CorrelationConfidence = "high" | "medium" | "low" | "none" | "unknown";

const CORRELATION_CONFIDENCE_TOKENS = new Set<CorrelationConfidence>([
  "high",
  "medium",
  "low",
  "none",
]);

export function resolveCorrelationConfidence(value: string): CorrelationConfidence {
  const token = String(value ?? "").trim() as CorrelationConfidence;
  return CORRELATION_CONFIDENCE_TOKENS.has(token) ? token : "unknown";
}

export const CORRELATION_CONFIDENCE_LABEL_KEYS: Record<CorrelationConfidence, MessageKey> = {
  high: "httpCorrelationConfidenceHigh",
  medium: "httpCorrelationConfidenceMedium",
  low: "httpCorrelationConfidenceLow",
  none: "httpCorrelationConfidenceNone",
  unknown: "httpCorrelationConfidenceUnknown",
};

export type CorrelationMatchBasis =
  | "request_id"
  | "method+path_template+status+time"
  | "explicit_wall_clock_anchor+time_overlap"
  | "target_host+nearest_duration"
  | "unknown";

const CORRELATION_MATCH_BASIS_TOKENS = new Set<CorrelationMatchBasis>([
  "request_id",
  "method+path_template+status+time",
  "explicit_wall_clock_anchor+time_overlap",
  "target_host+nearest_duration",
]);

export function resolveCorrelationMatchBasis(value: string): CorrelationMatchBasis {
  const token = String(value ?? "").trim() as CorrelationMatchBasis;
  return CORRELATION_MATCH_BASIS_TOKENS.has(token) ? token : "unknown";
}

export const CORRELATION_MATCH_BASIS_LABEL_KEYS: Record<CorrelationMatchBasis, MessageKey> = {
  request_id: "httpCorrelationBasisRequestId",
  "method+path_template+status+time": "httpCorrelationBasisShapeTime",
  "explicit_wall_clock_anchor+time_overlap": "httpCorrelationBasisAnchorOverlap",
  "target_host+nearest_duration": "httpCorrelationBasisHostDuration",
  unknown: "httpCorrelationBasisUnknown",
};

export type CorrelationSourceToken =
  | "http_capture"
  | "profile_evidence"
  | "jennifer_profile"
  | "access_log"
  | "unknown";

const CORRELATION_SOURCE_TOKENS = new Set<CorrelationSourceToken>([
  "http_capture",
  "profile_evidence",
  "jennifer_profile",
  "access_log",
]);

export function resolveCorrelationSource(value: string): CorrelationSourceToken {
  const token = String(value ?? "").trim() as CorrelationSourceToken;
  return CORRELATION_SOURCE_TOKENS.has(token) ? token : "unknown";
}

export const CORRELATION_SOURCE_LABEL_KEYS: Record<CorrelationSourceToken, MessageKey> = {
  http_capture: "httpCorrelationSourceHttp",
  profile_evidence: "httpCorrelationSourceProfile",
  jennifer_profile: "httpCorrelationSourceJennifer",
  access_log: "httpCorrelationSourceAccessLog",
  unknown: "httpCorrelationSourceUnknown",
};

// ── Contract adoption (H-RG4 R8 pattern) ────────────────────────────
//
// Declared after the closed token sets because adoption also cross-checks the
// contract's vocabularies against them: a contract that advertises a grade or
// confidence this renderer cannot name would be rendered as `unknown` rows
// with no way for the reader to tell an unimplemented token from a broken
// one. Refusing the contract outright discloses that honestly instead.

export function isCorrelationContractSupported(
  contract: HttpEvidenceCorrelationContract | null | undefined,
): boolean {
  return (
    !!contract &&
    contract.schema_version === HTTP_CORRELATION_CONTRACT_SCHEMA_VERSION &&
    contract.result_type === HTTP_CORRELATION_RESULT_TYPE &&
    contract.http_result_type === CORRELATION_SLOT_RESULT_TYPES.http &&
    contract.causal_claims_allowed === false &&
    // The renderer states "no source file or capture store was reopened" as a
    // safety disclosure; a contract that reopens sources must not be adopted
    // silently behind that sentence.
    contract.store_or_file_rescan === false &&
    Array.isArray(contract.alignment_grades) &&
    contract.alignment_grades.every((grade) => resolveCorrelationAlignment(grade) !== "unknown") &&
    Array.isArray(contract.confidence_grades) &&
    contract.confidence_grades.every(
      (confidence) => resolveCorrelationConfidence(confidence) !== "unknown",
    )
  );
}

/** Top-N bound check against the adopted contract; `empty` uses the default. */
export type CorrelationTopNState = "empty" | "valid" | "invalid";

export function correlationTopNState(
  topN: number | null,
  contract: HttpEvidenceCorrelationContract | null | undefined,
): CorrelationTopNState {
  if (topN === null) return "empty";
  if (!Number.isInteger(topN) || topN < 1) return "invalid";
  const max = contract?.max_top_n;
  if (typeof max === "number" && max > 0 && topN > max) return "invalid";
  return "valid";
}

// ── Envelope selectors ──────────────────────────────────────────────

export function extractCorrelationEnvelope(
  result: CorrelationResult,
): HttpCorrelationEnvelope | null {
  const meta = result?.metadata as Record<string, unknown> | undefined;
  const envelope = meta?.http_evidence_correlation;
  if (!envelope || typeof envelope !== "object") return null;
  return envelope as HttpCorrelationEnvelope;
}

export function selectCorrelationSummary(
  result: CorrelationResult,
): HttpCorrelationSummary | null {
  const summary = result?.summary as HttpCorrelationSummary | undefined;
  if (!summary || typeof summary !== "object") return null;
  return summary;
}

export type CorrelationTableKey =
  | "http_profile_overlaps"
  | "jennifer_network_gap_checks"
  | "access_log_matches"
  | "alignment_diagnostics";

export function selectCorrelationTable<Row>(
  result: CorrelationResult,
  key: CorrelationTableKey,
): Row[] {
  const tables = result?.tables as Record<string, unknown> | undefined;
  const rows = tables?.[key];
  return Array.isArray(rows) ? (rows as Row[]) : [];
}

export function selectCorrelationDiagnostics(
  result: CorrelationResult,
): HttpCorrelationSourceDiagnostic[] {
  return selectCorrelationTable<HttpCorrelationSourceDiagnostic>(result, "alignment_diagnostics");
}

export function selectCorrelationFindings(result: CorrelationResult): HttpCaptureFinding[] {
  const meta = result?.metadata as Record<string, unknown> | undefined;
  return (meta?.findings as HttpCaptureFinding[] | undefined) ?? [];
}

export function selectCorrelationProvenance(
  result: CorrelationResult,
): HttpCorrelationProvenanceRow[] {
  const meta = result?.metadata as Record<string, unknown> | undefined;
  const rows = meta?.source_provenance;
  return Array.isArray(rows) ? (rows as HttpCorrelationProvenanceRow[]) : [];
}

/** Diagnostic row for one secondary source, if that source participated. */
export function diagnosticForSource(
  diagnostics: HttpCorrelationSourceDiagnostic[],
  source: CorrelationSourceToken,
): HttpCorrelationSourceDiagnostic | null {
  return diagnostics.find((row) => resolveCorrelationSource(row.source) === source) ?? null;
}

/**
 * The primary HTTP timeline's own grade. The summary counters cover secondary
 * sources only, so the renderer reads this separately: a secondary source may
 * never be presented as better-aligned than the timeline it was compared to
 * without the reader being told what the primary grade was.
 */
export function correlationPrimaryAlignment(
  diagnostics: HttpCorrelationSourceDiagnostic[],
): CorrelationAlignmentGrade {
  const primary = diagnosticForSource(diagnostics, "http_capture");
  return primary ? resolveCorrelationAlignment(primary.alignment_grade) : "unknown";
}

// ── Access-log clock comparison ─────────────────────────────────────

export type CorrelationClockComparison =
  | { compared: true; deltaMs: number; reason: string | null }
  | { compared: false; deltaMs: null; reason: string | null };

/**
 * A request-ID pairing identifies the same request on both sides; it does not
 * compare clocks. The engine reports that as `clock_compared: false` and omits
 * `timestamp_delta_ms` entirely, so the renderer must show the delta as
 * unavailable rather than as a measured zero (X-RG1 B1). A row that claims a
 * delta without the comparison flag fails closed to unavailable.
 */
export function accessClockComparison(row: HttpCorrelationAccessRow): CorrelationClockComparison {
  const delta = row.timestamp_delta_ms;
  if (row.clock_compared === true && typeof delta === "number" && Number.isFinite(delta)) {
    return { compared: true, deltaMs: delta, reason: row.timestamp_alignment_reason ?? null };
  }
  return { compared: false, deltaMs: null, reason: row.timestamp_delta_unavailable_reason ?? null };
}

// ── Overlay gating ──────────────────────────────────────────────────

/**
 * correlationOverlayAllowed decides whether a time overlay may render for one
 * source. The verdict is the backend's: `overlay_allowed` must be true AND
 * the grade must resolve to `aligned`. An unrecognized grade fails closed —
 * the renderer never overlays clocks the engine did not certify compatible.
 */
export function correlationOverlayAllowed(
  diagnostic: HttpCorrelationSourceDiagnostic | null | undefined,
): boolean {
  return (
    !!diagnostic &&
    diagnostic.overlay_allowed === true &&
    resolveCorrelationAlignment(diagnostic.alignment_grade) === "aligned"
  );
}

// ── Anchor validation (advisory; the backend still fails closed) ────

export type CorrelationAnchorState = "empty" | "valid" | "invalid";

/**
 * The V8 profile wall-clock anchor must be RFC3339. This check is advisory —
 * a malformed anchor still fails closed in the engine (grade `none`) — but
 * the UI can warn before the run instead of after.
 */
export function correlationAnchorState(value: string): CorrelationAnchorState {
  const trimmed = String(value ?? "").trim();
  if (trimmed === "") return "empty";
  const rfc3339 =
    /^\d{4}-\d{2}-\d{2}[Tt]\d{2}:\d{2}:\d{2}(\.\d+)?([Zz]|[+-]\d{2}:\d{2})$/;
  if (!rfc3339.test(trimmed)) return "invalid";
  return Number.isNaN(Date.parse(trimmed)) ? "invalid" : "valid";
}

// ── Comparison candidates ───────────────────────────────────────────

/** Workspace entries eligible for a given correlation slot. */
export function correlationCandidates<T extends { result_type: string }>(
  entries: T[],
  slot: CorrelationSlot,
): T[] {
  return entries.filter((entry) => entry.result_type === CORRELATION_SLOT_RESULT_TYPES[slot]);
}

// ── Selection / run lifecycle reducer ───────────────────────────────
//
// Provenance invariant: a correlation result is only ever visible together
// with the exact inputs (four slot ids + anchor + tolerance) that produced
// it. Changing any input, starting a new run, or a failed run drops it.

export type HttpCorrelationError = { code: string; message: string };

export type HttpCorrelationInputs = {
  httpId: string | null;
  profileId: string | null;
  jenniferId: string | null;
  accessLogId: string | null;
  anchor: string;
  toleranceMs: number | null;
  /** null means "use the contract default"; any explicit value bounds output. */
  topN: number | null;
};

export type HttpCorrelationState = HttpCorrelationInputs & {
  contract: HttpEvidenceCorrelationContract | null;
  contractSupported: boolean;
  contractMismatch: boolean;
  running: boolean;
  result: HttpEvidenceCorrelationAnalysisResult | null;
  /** inputs that produced `result`; null whenever result is null. */
  resultInputs: HttpCorrelationInputs | null;
  error: HttpCorrelationError | null;
};

export const initialHttpCorrelationState: HttpCorrelationState = {
  contract: null,
  contractSupported: false,
  contractMismatch: false,
  httpId: null,
  profileId: null,
  jenniferId: null,
  accessLogId: null,
  anchor: "",
  toleranceMs: null,
  topN: null,
  running: false,
  result: null,
  resultInputs: null,
  error: null,
};

export function correlationInputsOf(state: HttpCorrelationInputs): HttpCorrelationInputs {
  return {
    httpId: state.httpId,
    profileId: state.profileId,
    jenniferId: state.jenniferId,
    accessLogId: state.accessLogId,
    anchor: state.anchor,
    toleranceMs: state.toleranceMs,
    topN: state.topN,
  };
}

export function sameCorrelationInputs(
  a: HttpCorrelationInputs,
  b: HttpCorrelationInputs,
): boolean {
  return (
    a.httpId === b.httpId &&
    a.profileId === b.profileId &&
    a.jenniferId === b.jenniferId &&
    a.accessLogId === b.accessLogId &&
    a.anchor === b.anchor &&
    a.toleranceMs === b.toleranceMs &&
    a.topN === b.topN
  );
}

/** At least one secondary source is required alongside HTTP. */
export function hasCorrelationSecondary(inputs: HttpCorrelationInputs): boolean {
  return !!(inputs.profileId || inputs.jenniferId || inputs.accessLogId);
}

export type HttpCorrelationAction =
  | { type: "contractLoaded"; contract: HttpEvidenceCorrelationContract }
  | { type: "contractUnavailable" }
  | { type: "setSlot"; slot: CorrelationSlot; id: string | null }
  | { type: "setAnchor"; anchor: string }
  | { type: "setTolerance"; toleranceMs: number | null }
  | { type: "setTopN"; topN: number | null }
  | { type: "runStart" }
  | {
      type: "runSuccess";
      result: HttpEvidenceCorrelationAnalysisResult;
      inputs: HttpCorrelationInputs;
    }
  | { type: "runError"; error: HttpCorrelationError }
  | { type: "reset" };

const SLOT_STATE_KEYS: Record<CorrelationSlot, "httpId" | "profileId" | "jenniferId" | "accessLogId"> = {
  http: "httpId",
  profile: "profileId",
  jennifer: "jenniferId",
  accessLog: "accessLogId",
};

export function httpCorrelationReducer(
  state: HttpCorrelationState,
  action: HttpCorrelationAction,
): HttpCorrelationState {
  switch (action.type) {
    case "contractLoaded": {
      const supported = isCorrelationContractSupported(action.contract);
      return {
        ...state,
        contract: action.contract,
        contractSupported: supported,
        contractMismatch: !supported,
      };
    }
    case "contractUnavailable":
      return { ...state, contract: null, contractSupported: false, contractMismatch: true };
    case "setSlot": {
      const key = SLOT_STATE_KEYS[action.slot];
      if (state[key] === action.id) return state;
      return { ...state, [key]: action.id, result: null, resultInputs: null, error: null };
    }
    case "setAnchor":
      if (state.anchor === action.anchor) return state;
      return { ...state, anchor: action.anchor, result: null, resultInputs: null, error: null };
    case "setTolerance":
      if (state.toleranceMs === action.toleranceMs) return state;
      return {
        ...state,
        toleranceMs: action.toleranceMs,
        result: null,
        resultInputs: null,
        error: null,
      };
    case "setTopN":
      if (state.topN === action.topN) return state;
      return { ...state, topN: action.topN, result: null, resultInputs: null, error: null };
    case "runStart":
      return { ...state, running: true, result: null, resultInputs: null, error: null };
    case "runSuccess":
      // A run that raced past an input change must not render under the new
      // inputs.
      if (!sameCorrelationInputs(action.inputs, correlationInputsOf(state))) {
        return { ...state, running: false };
      }
      return {
        ...state,
        running: false,
        result: action.result,
        resultInputs: action.inputs,
        error: null,
      };
    case "runError":
      return { ...state, running: false, result: null, resultInputs: null, error: action.error };
    case "reset":
      return {
        ...initialHttpCorrelationState,
        contract: state.contract,
        contractSupported: state.contractSupported,
        contractMismatch: state.contractMismatch,
      };
    default:
      return state;
  }
}

// ── Shared module store ─────────────────────────────────────────────

const listeners = new Set<() => void>();
let correlationState: HttpCorrelationState = initialHttpCorrelationState;

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getSnapshot(): HttpCorrelationState {
  return correlationState;
}

export function useHttpCorrelation(): HttpCorrelationState {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

export function dispatchHttpCorrelation(action: HttpCorrelationAction): void {
  const next = httpCorrelationReducer(correlationState, action);
  if (next === correlationState) return;
  correlationState = next;
  for (const listener of listeners) listener();
}

export function getHttpCorrelationState(): HttpCorrelationState {
  return correlationState;
}
