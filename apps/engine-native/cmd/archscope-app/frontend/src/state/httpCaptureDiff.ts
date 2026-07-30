// ─────────────────────────────────────────────────────────────────────
// [한글] state/httpCaptureDiff.ts — HTTP 세션 Diff(H-RG5 / T-582)의
// 순수(DOM 비의존) 파생 로직 + 페이지 간 공유 비교 스토어.
//
// 책임/목적:
//   - engine `http_capture_diff` 결과에서 요약/정렬 등급/비교 테이블/
//     프로세스 차원 가용성/findings 를 계산.
//   - 시간 정렬 등급(`aligned`/`duration_only`/`none`)에 따라 오버레이
//     허용/억제를 게이트: 등급은 backend 가 판정하고 renderer 는 그 판정을
//     그대로 따르며, 인식 못 하는 등급은 fail-closed 로 억제한다.
//   - 비교 대상 선택/실행 lifecycle 은 순수 reducer 로 관리하고, module
//     store 로 HttpCapturePage 와 Analysis Workspace 가 같은 선택을
//     공유한다 (분석 결과 provenance 불변식: 결과는 그것을 만든 쌍과만
//     함께 보인다).
//
// 의존성 주의: bridge/types 의 와이어 형식만 참조. Wails 호출은 컴포넌트
// 계층에서 수행하고 여기서는 순수 로직만 둔다 (browserCpuProfile 패턴).
// ─────────────────────────────────────────────────────────────────────

import { useSyncExternalStore } from "react";

import type { MessageKey } from "../i18n/messages";
import type {
  HttpCaptureDiffAnalysisResult,
  HttpCaptureDiffComparisonRow,
  HttpCaptureDiffContract,
  HttpCaptureDiffEnvelope,
  HttpCaptureDiffSummary,
  HttpCaptureDiffTimeAlignment,
  HttpCaptureFinding,
} from "../bridge/types";

type DiffResult = HttpCaptureDiffAnalysisResult | null;

/** The renderer implements exactly this diff contract schema version. */
export const HTTP_CAPTURE_DIFF_CONTRACT_SCHEMA_VERSION = 1;
export const HTTP_CAPTURE_DIFF_RESULT_TYPE = "http_capture_diff";
export const HTTP_CAPTURE_RESULT_TYPE = "http_capture";

// ── Contract adoption (mirrors the H-RG4 R8 pattern) ────────────────

/**
 * A contract this build does not implement must not be half-honored: the
 * compare action is disabled and the mismatch is disclosed instead of
 * guessing at unknown semantics.
 */
export function isDiffContractSupported(
  contract: HttpCaptureDiffContract | null | undefined,
): boolean {
  return (
    !!contract &&
    contract.schema_version === HTTP_CAPTURE_DIFF_CONTRACT_SCHEMA_VERSION &&
    contract.result_type === HTTP_CAPTURE_DIFF_RESULT_TYPE &&
    contract.source_result_type === HTTP_CAPTURE_RESULT_TYPE
  );
}

// ── Closed token sets (H-RG4 R11/S4 precedent) ──────────────────────

/** Time-alignment grades the backend may emit; anything else is unknown. */
export type DiffAlignmentGrade = "aligned" | "duration_only" | "none" | "unknown";

const DIFF_ALIGNMENT_GRADES = new Set<DiffAlignmentGrade>([
  "aligned",
  "duration_only",
  "none",
]);

export function resolveDiffAlignmentGrade(value: string): DiffAlignmentGrade {
  const token = String(value ?? "").trim() as DiffAlignmentGrade;
  return DIFF_ALIGNMENT_GRADES.has(token) ? token : "unknown";
}

export const DIFF_ALIGNMENT_LABEL_KEYS: Record<DiffAlignmentGrade, MessageKey> = {
  aligned: "httpCaptureDiffAlignmentAligned",
  duration_only: "httpCaptureDiffAlignmentDurationOnly",
  none: "httpCaptureDiffAlignmentNone",
  unknown: "httpCaptureDiffAlignmentUnknown",
};

export type DiffChangeToken = "changed" | "added" | "removed" | "unknown";

const DIFF_CHANGE_TOKENS = new Set<DiffChangeToken>(["changed", "added", "removed"]);

export function resolveDiffChange(value: string): DiffChangeToken {
  const token = String(value ?? "").trim() as DiffChangeToken;
  return DIFF_CHANGE_TOKENS.has(token) ? token : "unknown";
}

export const DIFF_CHANGE_LABEL_KEYS: Record<DiffChangeToken, MessageKey> = {
  changed: "httpCaptureDiffChangeChanged",
  added: "httpCaptureDiffChangeAdded",
  removed: "httpCaptureDiffChangeRemoved",
  unknown: "httpCaptureDiffChangeUnknown",
};

export type DiffRateUnavailableToken =
  | "timestamps_degenerate"
  | "capture_duration_unavailable"
  | "unknown";

const DIFF_RATE_UNAVAILABLE_TOKENS = new Set<DiffRateUnavailableToken>([
  "timestamps_degenerate",
  "capture_duration_unavailable",
]);

export function resolveDiffRateUnavailable(value: string): DiffRateUnavailableToken {
  const token = String(value ?? "").trim() as DiffRateUnavailableToken;
  return DIFF_RATE_UNAVAILABLE_TOKENS.has(token) ? token : "unknown";
}

export const DIFF_RATE_UNAVAILABLE_LABEL_KEYS: Record<DiffRateUnavailableToken, MessageKey> = {
  timestamps_degenerate: "httpCaptureDiffRateUnavailableDegenerate",
  capture_duration_unavailable: "httpCaptureDiffRateUnavailableNoDuration",
  unknown: "httpCaptureDiffRateUnavailableUnknown",
};

export type DiffSourceKindToken = "live_capture" | "har_import" | "unknown";

export function resolveDiffSourceKind(value: string): DiffSourceKindToken {
  const token = String(value ?? "").trim();
  return token === "live_capture" || token === "har_import" ? token : "unknown";
}

export const DIFF_SOURCE_KIND_LABEL_KEYS: Record<DiffSourceKindToken, MessageKey> = {
  live_capture: "httpCaptureDiffSourceLive",
  har_import: "httpCaptureDiffSourceHar",
  unknown: "httpCaptureDiffSourceUnknown",
};

// ── Envelope selectors ──────────────────────────────────────────────

export function extractDiffEnvelope(result: DiffResult): HttpCaptureDiffEnvelope | null {
  const meta = result?.metadata as Record<string, unknown> | undefined;
  const envelope = meta?.http_capture_diff;
  if (!envelope || typeof envelope !== "object") return null;
  return envelope as HttpCaptureDiffEnvelope;
}

export function selectDiffSummary(result: DiffResult): HttpCaptureDiffSummary | null {
  const summary = result?.summary as HttpCaptureDiffSummary | undefined;
  if (!summary || typeof summary !== "object") return null;
  return summary;
}

export type DiffTableKey =
  | "endpoints_changed"
  | "endpoints_added"
  | "endpoints_removed"
  | "hosts_changed"
  | "processes_changed";

export function selectDiffTable(
  result: DiffResult,
  key: DiffTableKey,
): HttpCaptureDiffComparisonRow[] {
  const tables = result?.tables as Record<string, unknown> | undefined;
  const rows = tables?.[key];
  return Array.isArray(rows) ? (rows as HttpCaptureDiffComparisonRow[]) : [];
}

export function selectDiffFindings(result: DiffResult): HttpCaptureFinding[] {
  const meta = result?.metadata as Record<string, unknown> | undefined;
  return (meta?.findings as HttpCaptureFinding[] | undefined) ?? [];
}

/**
 * diffHasChanges reports whether any comparison table carries a row. The
 * H-RG5 PASS criterion "reordered equivalent sessions yield no change" renders
 * through this: all-empty tables plus zero findings show an explicit
 * "no differences" state rather than an empty screen.
 */
export function diffHasChanges(result: DiffResult): boolean {
  const keys: DiffTableKey[] = [
    "endpoints_changed",
    "endpoints_added",
    "endpoints_removed",
    "hosts_changed",
    "processes_changed",
  ];
  return keys.some((key) => selectDiffTable(result, key).length > 0);
}

// ── Grade-aware overlay gating ──────────────────────────────────────

export type DiffOverlayPolicy = {
  /** Duration percentile before/after bars may render. */
  durations: boolean;
  /** Per-minute rate comparison may render (absolute clocks trusted). */
  perMinute: boolean;
};

/**
 * diffOverlayPolicy turns the backend's alignment verdict into what the
 * renderer may overlay. The decision is the backend's: `overlay_allowed`
 * enables anything at all, the grade decides how much. An unrecognized grade
 * suppresses every overlay (fail closed) — the renderer never upgrades a
 * verdict it cannot interpret.
 */
export function diffOverlayPolicy(
  alignment: HttpCaptureDiffTimeAlignment | null | undefined,
): DiffOverlayPolicy {
  if (!alignment || alignment.overlay_allowed !== true) {
    return { durations: false, perMinute: false };
  }
  switch (resolveDiffAlignmentGrade(alignment.grade)) {
    case "aligned":
      return { durations: true, perMinute: true };
    case "duration_only":
      return { durations: true, perMinute: false };
    default:
      return { durations: false, perMinute: false };
  }
}

// ── Comparison candidates ───────────────────────────────────────────

export type DiffCandidateEntry = {
  id: string;
  title: string;
  result_type: string;
  recorded_at: string;
};

/** Only http_capture results are comparison candidates. */
export function diffCandidateEntries<T extends { result_type: string }>(
  entries: T[],
): T[] {
  return entries.filter((entry) => entry.result_type === HTTP_CAPTURE_RESULT_TYPE);
}

/**
 * hasDiffSourceProjection reports whether an http_capture result carries the
 * bounded diff source projection the analyzer requires. Results analyzed
 * before the H-RG5 backend handoff lack it and must be re-analyzed — the UI
 * says so up front instead of surfacing the backend error after a click.
 */
export function hasDiffSourceProjection(result: unknown): boolean {
  if (!result || typeof result !== "object") return false;
  const metadata = (result as { metadata?: unknown }).metadata;
  if (!metadata || typeof metadata !== "object") return false;
  const projection = (metadata as Record<string, unknown>).http_capture_diff_source;
  return !!projection && typeof projection === "object";
}

// ── Comparison lifecycle reducer ────────────────────────────────────
//
// Provenance invariant (H-RG1 U3 precedent): a diff result is only ever
// visible together with the before/after pair that produced it. Changing
// either selection, starting a new comparison, or a failed comparison drops
// the prior result.

export type HttpCaptureDiffError = { code: string; message: string };

export type HttpCaptureDiffPair = { beforeId: string; afterId: string };

export type HttpCaptureDiffState = {
  /** null until GetHttpCaptureDiffContract resolves (or fails). */
  contract: HttpCaptureDiffContract | null;
  /** true only when a loaded contract matches the implemented schema. */
  contractSupported: boolean;
  /** true when a contract loaded but could not be honored, or failed to load. */
  contractMismatch: boolean;
  beforeId: string | null;
  afterId: string | null;
  running: boolean;
  result: HttpCaptureDiffAnalysisResult | null;
  /** the pair that produced `result`; null whenever result is null. */
  resultPair: HttpCaptureDiffPair | null;
  error: HttpCaptureDiffError | null;
};

export const initialHttpCaptureDiffState: HttpCaptureDiffState = {
  contract: null,
  contractSupported: false,
  contractMismatch: false,
  beforeId: null,
  afterId: null,
  running: false,
  result: null,
  resultPair: null,
  error: null,
};

export type HttpCaptureDiffAction =
  | { type: "contractLoaded"; contract: HttpCaptureDiffContract }
  | { type: "contractUnavailable" }
  | { type: "setBefore"; id: string | null }
  | { type: "setAfter"; id: string | null }
  | { type: "swap" }
  | { type: "compareStart" }
  | { type: "compareSuccess"; result: HttpCaptureDiffAnalysisResult; pair: HttpCaptureDiffPair }
  | { type: "compareError"; error: HttpCaptureDiffError }
  | { type: "reset" };

export function httpCaptureDiffReducer(
  state: HttpCaptureDiffState,
  action: HttpCaptureDiffAction,
): HttpCaptureDiffState {
  switch (action.type) {
    case "contractLoaded": {
      const supported = isDiffContractSupported(action.contract);
      return {
        ...state,
        contract: action.contract,
        contractSupported: supported,
        contractMismatch: !supported,
      };
    }
    case "contractUnavailable":
      return { ...state, contract: null, contractSupported: false, contractMismatch: true };
    case "setBefore":
      if (action.id === state.beforeId) return state;
      // A different input invalidates the rendered comparison.
      return { ...state, beforeId: action.id, result: null, resultPair: null, error: null };
    case "setAfter":
      if (action.id === state.afterId) return state;
      return { ...state, afterId: action.id, result: null, resultPair: null, error: null };
    case "swap":
      if (state.beforeId === state.afterId) return state;
      return {
        ...state,
        beforeId: state.afterId,
        afterId: state.beforeId,
        result: null,
        resultPair: null,
        error: null,
      };
    case "compareStart":
      return { ...state, running: true, result: null, resultPair: null, error: null };
    case "compareSuccess":
      // A result that raced past a selection change must not render under the
      // new pair.
      if (action.pair.beforeId !== state.beforeId || action.pair.afterId !== state.afterId) {
        return { ...state, running: false };
      }
      return { ...state, running: false, result: action.result, resultPair: action.pair, error: null };
    case "compareError":
      return { ...state, running: false, result: null, resultPair: null, error: action.error };
    case "reset":
      return {
        ...initialHttpCaptureDiffState,
        contract: state.contract,
        contractSupported: state.contractSupported,
        contractMismatch: state.contractMismatch,
      };
    default:
      return state;
  }
}

// ── Shared module store ─────────────────────────────────────────────
//
// HttpCapturePage's compare action and the Analysis Workspace comparison
// entry operate on the same selection, so the reducer state lives in a
// module store (analysisWorkspace pattern) rather than per-page useReducer.

const listeners = new Set<() => void>();
let diffState: HttpCaptureDiffState = initialHttpCaptureDiffState;

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getSnapshot(): HttpCaptureDiffState {
  return diffState;
}

export function useHttpCaptureDiff(): HttpCaptureDiffState {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

export function dispatchHttpCaptureDiff(action: HttpCaptureDiffAction): void {
  const next = httpCaptureDiffReducer(diffState, action);
  if (next === diffState) return;
  diffState = next;
  for (const listener of listeners) listener();
}

export function getHttpCaptureDiffState(): HttpCaptureDiffState {
  return diffState;
}
