// ─────────────────────────────────────────────────────────────────────
// [한글] state/httpCapture.ts — 오프라인 HAR(HTTP capture) 페이지의
// 순수(DOM 비의존) 파생 로직.
//
// 책임/목적:
//   - engine `http_capture` 결과에서 요약, 타임라인, 트랜잭션, 유사(pseudo)
//     프로세스 트리, 필터/브러시 창, 리댁션·충실도(fidelity)·진단 메타를 계산.
//   - React/JSDOM 없이 Node 기반 regression 하니스(state/regression.test.ts)
//     로 검증 가능하도록 로직을 페이지 밖으로 분리 (browserCpuProfile.ts 와
//     동일 패턴).
//
// 유사 프로세스(pseudo-process) 트리:
//   HAR import 는 OS 프로세스 귀속 정보가 없으므로(process=null) 관측
//   원본(host/origin)을 최상위 "유사 프로세스" 노드로 합성한다. 실제
//   live-capture 결과가 process 메타를 담으면 그 노드를 우선 사용한다.
//   계층: process(또는 origin) → connection → transaction.
//
// 의존성 주의: bridge/types 의 와이어 형식만 참조하며 런타임 코드 없음.
// ─────────────────────────────────────────────────────────────────────

import type {
  CaptureHTTPMessage,
  CaptureTimingPhases,
  HttpCaptureAnalysisResult,
  HttpCaptureEndpointRow,
  HttpCaptureFinding,
  HttpCaptureHostRow,
  HttpCaptureMeta,
  HttpCaptureRedaction,
  HttpCaptureSummary,
  HttpCaptureTimelineBucket,
  HttpCaptureTransactionRow,
  ParserDiagnostics,
} from "../bridge/types";

type Result = HttpCaptureAnalysisResult | null;

// ── Basic selectors ─────────────────────────────────────────────────

export function selectHttpSummary(result: Result): HttpCaptureSummary | null {
  const summary = result?.summary as HttpCaptureSummary | undefined;
  return summary && typeof summary === "object" ? summary : null;
}

export function selectTransactions(result: Result): HttpCaptureTransactionRow[] {
  const tables = result?.tables as Record<string, unknown> | undefined;
  return (tables?.transactions as HttpCaptureTransactionRow[] | undefined) ?? [];
}

export function selectEndpoints(result: Result): HttpCaptureEndpointRow[] {
  const tables = result?.tables as Record<string, unknown> | undefined;
  return (tables?.endpoints as HttpCaptureEndpointRow[] | undefined) ?? [];
}

export function selectHosts(result: Result): HttpCaptureHostRow[] {
  const tables = result?.tables as Record<string, unknown> | undefined;
  return (tables?.hosts as HttpCaptureHostRow[] | undefined) ?? [];
}

export function selectTimeline(result: Result): HttpCaptureTimelineBucket[] {
  const series = result?.series as Record<string, unknown> | undefined;
  return (series?.timeline as HttpCaptureTimelineBucket[] | undefined) ?? [];
}

export function selectFindings(result: Result): HttpCaptureFinding[] {
  const meta = result?.metadata as Record<string, unknown> | undefined;
  return (meta?.findings as HttpCaptureFinding[] | undefined) ?? [];
}

// ── Capture metadata / fidelity ─────────────────────────────────────

export function extractCaptureMeta(result: Result): HttpCaptureMeta | null {
  const meta = result?.metadata as Record<string, unknown> | undefined;
  const capture = meta?.http_capture;
  if (!capture || typeof capture !== "object") return null;
  return capture as HttpCaptureMeta;
}

// ── Redaction ───────────────────────────────────────────────────────

export type RedactionInfo = HttpCaptureRedaction & { total: number };

export function extractRedaction(result: Result): RedactionInfo | null {
  const capture = extractCaptureMeta(result);
  const redaction = capture?.redaction;
  if (!redaction || typeof redaction !== "object") return null;
  const counts = redaction.counts ?? {};
  const total = Object.values(counts).reduce(
    (sum, value) => sum + (Number(value) || 0),
    0,
  );
  return {
    applied: redaction.applied === true,
    version: redaction.version ?? "",
    rules: Array.isArray(redaction.rules) ? redaction.rules : [],
    counts,
    total,
  };
}

// ── Diagnostics ─────────────────────────────────────────────────────

export function extractHttpDiagnostics(result: Result): ParserDiagnostics | null {
  const diags = (result?.metadata as Record<string, unknown> | undefined)?.diagnostics;
  if (!diags || typeof diags !== "object") return null;
  return diags as unknown as ParserDiagnostics;
}

export function httpDiagnosticIssueCount(diags: ParserDiagnostics | null): number {
  if (!diags) return 0;
  return (diags.warning_count ?? 0) + (diags.error_count ?? 0);
}

// ── Status / error classification ───────────────────────────────────

export type StatusClass = "2xx" | "3xx" | "4xx" | "5xx" | "other";

export function statusClassOf(status: number): StatusClass {
  if (status >= 200 && status < 300) return "2xx";
  if (status >= 300 && status < 400) return "3xx";
  if (status >= 400 && status < 500) return "4xx";
  if (status >= 500 && status < 600) return "5xx";
  return "other";
}

export function isErrorTransaction(tx: HttpCaptureTransactionRow): boolean {
  return tx.status >= 400 || tx.state === "failed";
}

// ── Filtering ───────────────────────────────────────────────────────

export type TimeWindow = { start: string; end: string };

export type TransactionFilter = {
  /** substring over method / url / host / path / status */
  query: string;
  /** "" = all methods */
  method: string;
  /** "" = all classes */
  statusClass: "" | StatusClass;
  errorsOnly: boolean;
  /** inclusive-of-window; null = no time constraint */
  window: TimeWindow | null;
  /** "" = all content types; matched against the transaction base MIME */
  mime: string;
  /** "" = all fidelities */
  fidelity: string;
  /** inclusive lower bound on duration_ms; null = no lower bound */
  minDurationMs: number | null;
  /** inclusive upper bound on duration_ms; null = no upper bound */
  maxDurationMs: number | null;
};

export const emptyFilter: TransactionFilter = {
  query: "",
  method: "",
  statusClass: "",
  errorsOnly: false,
  window: null,
  mime: "",
  fidelity: "",
  minDurationMs: null,
  maxDurationMs: null,
};

/**
 * isFilterActive reports whether the user has narrowed the result at all.
 * When true the summary cards recompute over the same filtered rows the list
 * and tree use, so every panel shares one denominator (H-RG1 U1 contract).
 */
export function isFilterActive(filter: TransactionFilter): boolean {
  return (
    filter.query.trim() !== "" ||
    filter.method !== "" ||
    filter.statusClass !== "" ||
    filter.errorsOnly ||
    filter.window !== null ||
    filter.mime !== "" ||
    filter.fidelity !== "" ||
    filter.minDurationMs !== null ||
    filter.maxDurationMs !== null
  );
}

/** baseMime strips any `; charset=…` parameters and lower-cases the type. */
function baseMime(contentType: string | undefined): string {
  if (!contentType) return "";
  const semi = contentType.indexOf(";");
  return (semi >= 0 ? contentType.slice(0, semi) : contentType).trim().toLowerCase();
}

/**
 * transactionMime resolves the content type used for the MIME filter. The
 * response type is preferred (what the analyst usually filters on); the
 * request type is a fallback for uploads with no response body.
 */
export function transactionMime(tx: HttpCaptureTransactionRow): string {
  const response = baseMime((tx.response as CaptureHTTPMessage | undefined)?.contentType);
  if (response) return response;
  return baseMime((tx.request as CaptureHTTPMessage | undefined)?.contentType);
}

function matchesQuery(tx: HttpCaptureTransactionRow, query: string): boolean {
  if (!query) return true;
  const needle = query.trim().toLowerCase();
  if (!needle) return true;
  const haystack = `${tx.method} ${tx.url} ${tx.host} ${tx.path} ${tx.status}`.toLowerCase();
  return haystack.includes(needle);
}

function withinWindow(tx: HttpCaptureTransactionRow, window: TimeWindow | null): boolean {
  if (!window) return true;
  if (!tx.started_at) return false;
  const started = Date.parse(tx.started_at);
  if (Number.isNaN(started)) return false;
  const start = Date.parse(window.start);
  const end = Date.parse(window.end);
  if (Number.isNaN(start) || Number.isNaN(end)) return true;
  return started >= start && started < end;
}

export function filterTransactions(
  transactions: HttpCaptureTransactionRow[],
  filter: TransactionFilter,
): HttpCaptureTransactionRow[] {
  return transactions.filter((tx) => {
    if (filter.method && tx.method !== filter.method) return false;
    if (filter.statusClass && statusClassOf(tx.status) !== filter.statusClass) return false;
    if (filter.errorsOnly && !isErrorTransaction(tx)) return false;
    if (filter.mime && transactionMime(tx) !== filter.mime) return false;
    if (filter.fidelity && tx.fidelity !== filter.fidelity) return false;
    if (filter.minDurationMs !== null || filter.maxDurationMs !== null) {
      const duration = Number(tx.duration_ms);
      // Unknown/NaN durations drop out of an explicit duration range.
      if (!Number.isFinite(duration)) return false;
      if (filter.minDurationMs !== null && duration < filter.minDurationMs) return false;
      if (filter.maxDurationMs !== null && duration > filter.maxDurationMs) return false;
    }
    if (!matchesQuery(tx, filter.query)) return false;
    if (!withinWindow(tx, filter.window)) return false;
    return true;
  });
}

/** Distinct HTTP methods present, for the method filter dropdown. */
export function availableMethods(transactions: HttpCaptureTransactionRow[]): string[] {
  const methods = new Set<string>();
  for (const tx of transactions) {
    if (tx.method) methods.add(tx.method);
  }
  return Array.from(methods).sort();
}

/** Distinct base MIME types present, for the content-type filter dropdown. */
export function availableMimeTypes(transactions: HttpCaptureTransactionRow[]): string[] {
  const mimes = new Set<string>();
  for (const tx of transactions) {
    const mime = transactionMime(tx);
    if (mime) mimes.add(mime);
  }
  return Array.from(mimes).sort();
}

/** Distinct fidelity labels present, for the fidelity filter dropdown. */
export function availableFidelities(transactions: HttpCaptureTransactionRow[]): string[] {
  const values = new Set<string>();
  for (const tx of transactions) {
    if (tx.fidelity) values.add(tx.fidelity);
  }
  return Array.from(values).sort();
}

// ── Selection projection (shared denominator) ───────────────────────
//
// When a brush window or any filter is active, the summary cards must be
// recomputed over the exact same rows the list and tree render, so all
// three panels agree on one numerator/denominator. The engine `summary`
// stays authoritative only for the unfiltered, whole-session view.

export type SummaryProjection = {
  transactions: number;
  errorTransactions: number;
  errorRate: number;
  uniqueHosts: number;
  uniqueEndpoints: number;
  durationP95Ms: number;
  responseBytes: number;
};

/** nearest-rank percentile over non-negative finite durations. */
function percentile(values: number[], p: number): number {
  if (values.length === 0) return 0;
  const sorted = [...values].sort((a, b) => a - b);
  const rank = Math.ceil((p / 100) * sorted.length);
  const index = Math.min(sorted.length - 1, Math.max(0, rank - 1));
  return sorted[index] ?? 0;
}

/**
 * projectSummary re-aggregates a transaction subset into the same six
 * headline metrics the summary cards show. Callers pass the filtered rows so
 * the cards match the visible list/tree exactly (H-RG1 U1 contract). Because
 * the detail table is bounded, this is a floor over the visible rows, not a
 * substitute for the engine's whole-session summary — the UI labels it so.
 */
export function projectSummary(
  transactions: HttpCaptureTransactionRow[],
): SummaryProjection {
  let errorTransactions = 0;
  let responseBytes = 0;
  const hosts = new Set<string>();
  const endpoints = new Set<string>();
  const durations: number[] = [];
  for (const tx of transactions) {
    if (isErrorTransaction(tx)) errorTransactions += 1;
    responseBytes += Math.max(0, Number(tx.response_bytes) || 0);
    if (tx.host) hosts.add(tx.host);
    endpoints.add(`${tx.method} ${tx.host}${tx.path}`);
    const duration = Number(tx.duration_ms);
    if (Number.isFinite(duration) && duration >= 0) durations.push(duration);
  }
  const count = transactions.length;
  return {
    transactions: count,
    errorTransactions,
    errorRate: count > 0 ? errorTransactions / count : 0,
    uniqueHosts: hosts.size,
    uniqueEndpoints: endpoints.size,
    durationP95Ms: percentile(durations, 95),
    responseBytes,
  };
}

// ── Timeline brush → time window ────────────────────────────────────

const MINUTE_MS = 60_000;

/**
 * timelineWindow maps a [startIdx, endIdx] bucket selection to an inclusive
 * ISO time window. Bucket `end` is the last minute's start instant, so the
 * window end is pushed one minute forward to cover that whole minute.
 * Indices are clamped and order-normalized so a right-to-left brush works.
 */
export function timelineWindow(
  buckets: HttpCaptureTimelineBucket[],
  startIdx: number,
  endIdx: number,
): TimeWindow | null {
  if (buckets.length === 0) return null;
  let lo = Math.max(0, Math.min(startIdx, endIdx));
  let hi = Math.min(buckets.length - 1, Math.max(startIdx, endIdx));
  if (lo > hi) [lo, hi] = [hi, lo];
  const start = buckets[lo]?.start;
  const endBucket = buckets[hi]?.end;
  if (!start || !endBucket) return null;
  const endMs = Date.parse(endBucket);
  const end = Number.isNaN(endMs)
    ? endBucket
    : new Date(endMs + MINUTE_MS).toISOString();
  return { start, end };
}

// ── Pseudo-process tree ─────────────────────────────────────────────

export type ProcessTreeKind = "process" | "connection" | "transaction";

export type ProcessTreeNode = {
  id: string;
  kind: ProcessTreeKind;
  label: string;
  sublabel: string;
  /** true when a process node is synthesized from origin (no OS attribution). */
  pseudo: boolean;
  attribution: string;
  count: number;
  errorCount: number;
  totalDurationMs: number;
  requestBytes: number;
  responseBytes: number;
  /** set for transaction leaves so the UI can open the detail panel. */
  transactionId?: string;
  children: ProcessTreeNode[];
};

type MutableNode = ProcessTreeNode & {
  childIndex?: Map<string, MutableNode>;
  order: number;
};

function ensureChild(
  parent: MutableNode,
  key: string,
  create: () => MutableNode,
): MutableNode {
  if (!parent.childIndex) parent.childIndex = new Map();
  let node = parent.childIndex.get(key);
  if (!node) {
    node = create();
    parent.childIndex.set(key, node);
    parent.children.push(node);
  }
  return node;
}

function accumulate(node: MutableNode, tx: HttpCaptureTransactionRow): void {
  node.count += 1;
  if (isErrorTransaction(tx)) node.errorCount += 1;
  node.totalDurationMs += Number(tx.duration_ms) || 0;
  node.requestBytes += Math.max(0, Number(tx.request_bytes) || 0);
  node.responseBytes += Math.max(0, Number(tx.response_bytes) || 0);
}

/**
 * buildProcessTree groups transactions into process → connection →
 * transaction. When a transaction carries OS process attribution it roots
 * under that process; otherwise a pseudo-process node is synthesized from
 * the observation host so HAR imports still get a meaningful hierarchy.
 */
export function buildProcessTree(
  transactions: HttpCaptureTransactionRow[],
): ProcessTreeNode[] {
  const roots: MutableNode[] = [];
  const rootIndex = new Map<string, MutableNode>();
  let counter = 0;

  const makeNode = (
    partial: Pick<ProcessTreeNode, "id" | "kind" | "label" | "sublabel" | "pseudo" | "attribution"> &
      Partial<Pick<ProcessTreeNode, "transactionId">>,
  ): MutableNode => ({
    ...partial,
    count: 0,
    errorCount: 0,
    totalDurationMs: 0,
    requestBytes: 0,
    responseBytes: 0,
    children: [],
    order: counter++,
  });

  for (const tx of transactions) {
    const host = tx.host || "(unknown host)";
    const process = tx.process;

    const processKey = process
      ? `p:${process.pid}:${process.start_time}:${process.name}`
      : `origin:${host}`;

    let root = rootIndex.get(processKey);
    if (!root) {
      root = makeNode({
        id: `proc-${processKey}`,
        kind: "process",
        label: process ? process.name || "(process)" : host,
        sublabel: process ? `pid ${process.pid}` : "origin · pseudo-process",
        pseudo: !process,
        attribution: process?.attribution || "har_origin",
      });
      rootIndex.set(processKey, root);
      roots.push(root);
    }
    accumulate(root, tx);

    const connKey = tx.connection_id || "(no connection)";
    const conn = ensureChild(root, connKey, () =>
      makeNode({
        id: `conn-${processKey}-${connKey}`,
        kind: "connection",
        label: tx.connection_id ? `conn ${tx.connection_id}` : "(no connection)",
        sublabel: tx.used_existing_connection ? "reused" : "new",
        pseudo: root!.pseudo,
        attribution: root!.attribution,
      }),
    );
    accumulate(conn, tx);

    const leaf = makeNode({
      id: `tx-${tx.id}`,
      kind: "transaction",
      label: `${tx.method || "GET"} ${tx.path || "/"}`,
      sublabel: `${tx.status || 0} · ${tx.http_version || ""}`.trim(),
      pseudo: root!.pseudo,
      attribution: root!.attribution,
      transactionId: tx.id,
    });
    accumulate(leaf, tx);
    conn.children.push(leaf);
  }

  return sortTree(roots);
}

function sortTree(nodes: MutableNode[]): ProcessTreeNode[] {
  const sorted = [...nodes].sort((a, b) => {
    if (b.totalDurationMs !== a.totalDurationMs) return b.totalDurationMs - a.totalDurationMs;
    if (b.count !== a.count) return b.count - a.count;
    return a.order - b.order;
  });
  return sorted.map((node) => {
    const children =
      node.kind === "connection"
        ? // leaves keep source order (sequence) for readability
          [...node.children].sort((a, b) => (a as MutableNode).order - (b as MutableNode).order)
        : sortTree(node.children as MutableNode[]);
    // strip the internal bookkeeping fields from the returned shape
    const { childIndex: _childIndex, order: _order, ...clean } = node;
    void _childIndex;
    void _order;
    return { ...clean, children };
  });
}

// ── Timing breakdown (for the detail panel) ─────────────────────────

export type TimingPhaseRow = {
  phase: string;
  ms: number;
  state: string;
};

const TIMING_PHASE_ORDER: Array<keyof CaptureTimingPhases> = [
  "blocked",
  "dns",
  "connect",
  "tls",
  "send",
  "wait",
  "receive",
];

/**
 * timingBreakdown flattens the most detailed available timing set into an
 * ordered phase list. clientProxy/proxyUpstream (live capture) take
 * precedence over importedHar (HAR) when present.
 */
export function timingBreakdown(tx: HttpCaptureTransactionRow | null): TimingPhaseRow[] {
  const timings = tx?.timings;
  if (!timings) return [];
  const phases =
    timings.clientProxy ??
    timings.proxyUpstream ??
    timings.proxyInternal ??
    timings.importedHar;
  if (!phases) return [];
  return TIMING_PHASE_ORDER.map((phase) => {
    const duration = phases[phase];
    return {
      phase,
      ms: duration?.state === "known" ? duration.ms : 0,
      state: duration?.state ?? "unknown",
    };
  });
}

// ── Page state / provenance reducer ─────────────────────────────────
//
// A single pure reducer owns the analysis lifecycle so result provenance is
// deterministic and testable (H-RG1 U3). The invariant it protects: a result
// is only ever visible together with the source that produced it. Selecting a
// new file, starting a new analysis, or a failed analysis all drop the prior
// result so a stale success can never render under a new source or an error.

export type HttpCaptureError = { code: string; message: string };

export type HttpCaptureAnalysisState = {
  running: boolean;
  result: HttpCaptureAnalysisResult | null;
  /** name of the source that produced `result`; null whenever result is null */
  resultSource: string | null;
  error: HttpCaptureError | null;
  filter: TransactionFilter;
  selectedId: string | null;
};

export const initialHttpCaptureState: HttpCaptureAnalysisState = {
  running: false,
  result: null,
  resultSource: null,
  error: null,
  filter: emptyFilter,
  selectedId: null,
};

export type HttpCaptureAction =
  | { type: "reset" }
  | { type: "analyzeStart" }
  | { type: "analyzeSuccess"; result: HttpCaptureAnalysisResult; source: string }
  | { type: "analyzeError"; error: HttpCaptureError }
  | { type: "setFilter"; filter: TransactionFilter }
  | { type: "patchFilter"; patch: Partial<TransactionFilter> }
  | { type: "select"; id: string }
  | { type: "closeDetail" };

export function httpCaptureReducer(
  state: HttpCaptureAnalysisState,
  action: HttpCaptureAction,
): HttpCaptureAnalysisState {
  switch (action.type) {
    case "reset":
      // File change / clear: forget everything from the previous source.
      return { ...initialHttpCaptureState };
    case "analyzeStart":
      // Drop the prior result up front so a pending or failed re-analysis can
      // never leave a stale success rendered under the new run.
      return { ...initialHttpCaptureState, running: true };
    case "analyzeSuccess":
      return {
        ...state,
        running: false,
        error: null,
        result: action.result,
        resultSource: action.source,
      };
    case "analyzeError":
      return {
        ...state,
        running: false,
        error: action.error,
        result: null,
        resultSource: null,
      };
    case "setFilter":
      return { ...state, filter: action.filter };
    case "patchFilter":
      return { ...state, filter: { ...state.filter, ...action.patch } };
    case "select":
      return { ...state, selectedId: action.id };
    case "closeDetail":
      return { ...state, selectedId: null };
    default:
      return state;
  }
}

// ── Byte formatting (local — formatters.ts has no byte helper) ───────

export function formatBytes(value: number | null | undefined): string {
  if (typeof value !== "number" || Number.isNaN(value)) return "-";
  if (value < 1024) return `${value} B`;
  const units = ["KiB", "MiB", "GiB", "TiB"];
  let size = value / 1024;
  let unit = 0;
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024;
    unit += 1;
  }
  return `${size.toFixed(size >= 100 ? 0 : 1)} ${units[unit]}`;
}
