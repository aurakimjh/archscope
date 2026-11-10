import { httpColumnValue } from "./httpColumns.js";
// ─────────────────────────────────────────────────────────────────────
// [한글] state/httpAnalyzer.ts — HTTP Analyzer 스타일 패킷 그리드/상세
// 뷰의 순수(DOM 비의존) 파생 로직.
//
// 책임/목적:
//   - 트랜잭션 행을 리소스 종류(document/script/image/…)로 분류해 이미지·
//     CSS·폰트 호출이 목록에서 사라지지 않게 한다.
//   - 정규화된 semantic 모델에서 raw HTTP 프로토콜 텍스트를 재구성한다.
//     원본 바이트가 아니라 "재구성"임을 값으로 명시한다(raw_wire 는 설계상
//     범위 밖).
//   - 세션 전체 시간축 기준의 워터폴(phase bar) 좌표를 계산한다.
//   - 그리드 정렬을 결정적(deterministic)으로 만든다.
//
// httpCapture.ts 와 마찬가지로 React/JSDOM 없이 Node 하니스
// (state/regression.test.ts)에서 검증 가능해야 한다.
// ─────────────────────────────────────────────────────────────────────

import type { MessageKey } from "../i18n/messages";
import type {
  CaptureHTTPMessage,
  CaptureTimingPhases,
  HttpCaptureTransactionRow,
} from "../bridge/types";

// ── Resource kind ───────────────────────────────────────────────────
//
// HTTP Analyzer-style tools let an analyst see, at a glance, that a page
// pulled 40 images and 6 scripts. The engine reports a MIME type and a path,
// never a category, so the category is derived here — from the response type
// first, then the request type, then the path extension. Nothing is guessed
// from the host or the method.

export type ResourceKind =
  | "document"
  | "stylesheet"
  | "script"
  | "image"
  | "font"
  | "media"
  | "json"
  | "form"
  | "text"
  | "binary"
  | "connect"
  | "other";

export const RESOURCE_KIND_LABEL_KEYS: Record<ResourceKind, MessageKey> = {
  document: "httpAnalyzerKindDocument",
  stylesheet: "httpAnalyzerKindStylesheet",
  script: "httpAnalyzerKindScript",
  image: "httpAnalyzerKindImage",
  font: "httpAnalyzerKindFont",
  media: "httpAnalyzerKindMedia",
  json: "httpAnalyzerKindJson",
  form: "httpAnalyzerKindForm",
  text: "httpAnalyzerKindText",
  binary: "httpAnalyzerKindBinary",
  connect: "httpAnalyzerKindConnect",
  other: "httpAnalyzerKindOther",
};

/** All kinds in the order the filter dropdown offers them. */
export const RESOURCE_KINDS: ResourceKind[] = [
  "document",
  "stylesheet",
  "script",
  "image",
  "font",
  "media",
  "json",
  "form",
  "text",
  "binary",
  "connect",
  "other",
];

function baseMime(contentType: string | undefined | null): string {
  if (!contentType) return "";
  const semi = contentType.indexOf(";");
  return (semi >= 0 ? contentType.slice(0, semi) : contentType).trim().toLowerCase();
}

function kindFromMime(mime: string): ResourceKind | null {
  if (!mime) return null;
  if (mime === "text/html" || mime === "application/xhtml+xml") return "document";
  if (mime === "text/css") return "stylesheet";
  if (
    mime === "text/javascript" ||
    mime === "application/javascript" ||
    mime === "application/x-javascript" ||
    mime === "application/ecmascript" ||
    mime === "text/ecmascript"
  ) {
    return "script";
  }
  if (mime.startsWith("image/")) return "image";
  if (mime.startsWith("font/") || mime === "application/font-woff" || mime === "application/vnd.ms-fontobject") {
    return "font";
  }
  if (mime.startsWith("audio/") || mime.startsWith("video/")) return "media";
  if (mime === "application/json" || mime.endsWith("+json")) return "json";
  if (mime === "application/x-www-form-urlencoded" || mime === "multipart/form-data") return "form";
  if (mime.startsWith("text/") || mime === "application/xml" || mime.endsWith("+xml")) return "text";
  if (mime === "application/octet-stream" || mime === "application/pdf" || mime.startsWith("application/zip")) {
    return "binary";
  }
  return null;
}

const EXTENSION_KINDS: Record<string, ResourceKind> = {
  html: "document",
  htm: "document",
  css: "stylesheet",
  js: "script",
  mjs: "script",
  cjs: "script",
  ts: "script",
  png: "image",
  jpg: "image",
  jpeg: "image",
  gif: "image",
  webp: "image",
  avif: "image",
  svg: "image",
  ico: "image",
  bmp: "image",
  woff: "font",
  woff2: "font",
  ttf: "font",
  otf: "font",
  eot: "font",
  mp4: "media",
  webm: "media",
  mp3: "media",
  ogg: "media",
  wav: "media",
  m3u8: "media",
  json: "json",
  xml: "text",
  txt: "text",
  csv: "text",
  pdf: "binary",
  zip: "binary",
  wasm: "binary",
};

function kindFromPath(path: string | undefined): ResourceKind | null {
  if (!path) return null;
  const clean = path.split("?")[0]?.split("#")[0] ?? "";
  const dot = clean.lastIndexOf(".");
  if (dot < 0 || dot === clean.length - 1) return null;
  const extension = clean.slice(dot + 1).toLowerCase();
  return EXTENSION_KINDS[extension] ?? null;
}

/**
 * resourceKindOf classifies one transaction. A CONNECT tunnel is its own kind
 * rather than `other`, because a passthrough row is a fundamentally different
 * observation from an unclassifiable response.
 */
export function resourceKindOf(tx: HttpCaptureTransactionRow): ResourceKind {
  if ((tx.method ?? "").toUpperCase() === "CONNECT") return "connect";
  const response = tx.response as CaptureHTTPMessage | undefined;
  const request = tx.request as CaptureHTTPMessage | undefined;
  return (
    kindFromMime(baseMime(response?.contentType)) ??
    kindFromPath(tx.path) ??
    kindFromMime(baseMime(request?.contentType)) ??
    "other"
  );
}

/** Distinct kinds present, in the canonical order, for the filter dropdown. */
export function availableResourceKinds(
  transactions: HttpCaptureTransactionRow[],
): ResourceKind[] {
  const present = new Set<ResourceKind>();
  for (const tx of transactions) present.add(resourceKindOf(tx));
  return RESOURCE_KINDS.filter((kind) => present.has(kind));
}

/** Per-kind counts for the summary strip, ordered by the canonical kind order. */
export function resourceKindBreakdown(
  transactions: HttpCaptureTransactionRow[],
): { kind: ResourceKind; count: number; bytes: number }[] {
  const counts = new Map<ResourceKind, { count: number; bytes: number }>();
  for (const tx of transactions) {
    const kind = resourceKindOf(tx);
    const entry = counts.get(kind) ?? { count: 0, bytes: 0 };
    entry.count += 1;
    entry.bytes += Math.max(0, Number(tx.response_bytes) || 0);
    counts.set(kind, entry);
  }
  return RESOURCE_KINDS.filter((kind) => counts.has(kind)).map((kind) => ({
    kind,
    count: counts.get(kind)!.count,
    bytes: counts.get(kind)!.bytes,
  }));
}

// ── Body storage ────────────────────────────────────────────────────
//
// "No body shown" has several distinct causes and they must not read as one
// another: a body that was never recorded, one capped at the size limit, one
// whose Content-Encoding this tier cannot decode, one that is not text, and one
// the redaction policy suppressed. Each gets its own token and its own prose.

export type BodyStorageToken =
  | "image_preview"
  | "export_omitted"
  | "omitted"
  | "inline"
  | "truncated"
  | "encoded"
  | "binary"
  | "redacted"
  | "blob"
  | "unknown";

const BODY_STORAGE_TOKENS = new Set<BodyStorageToken>([
  "image_preview",
  "export_omitted",
  "omitted",
  "inline",
  "truncated",
  "encoded",
  "binary",
  "redacted",
  "blob",
]);

export function resolveBodyStorage(value: string | undefined): BodyStorageToken {
  const token = String(value ?? "").trim() as BodyStorageToken;
  return BODY_STORAGE_TOKENS.has(token) ? token : "unknown";
}

export const BODY_STORAGE_LABEL_KEYS: Record<BodyStorageToken, MessageKey> = {
  image_preview: "httpContentImageNotice",
  export_omitted: "httpFilesOmitted",
  omitted: "httpAnalyzerBodyOmitted",
  inline: "httpAnalyzerBodyInline",
  truncated: "httpAnalyzerBodyTruncated",
  encoded: "httpAnalyzerBodyEncoded",
  binary: "httpAnalyzerBodyBinary",
  redacted: "httpAnalyzerBodyRedacted",
  blob: "httpAnalyzerBodyBlob",
  unknown: "httpAnalyzerBodyUnknown",
};

/** Why a body is not displayed, or null when it is. */
export function bodyUnavailableKey(
  message: CaptureHTTPMessage | undefined,
): MessageKey | null {
  if (message?.bodyPreview) return null;
  const storage = resolveBodyStorage(message?.bodyStorage);
  if (storage === "inline" && (message?.bodySize ?? 0) === 0) {
    return "httpAnalyzerBodyEmpty";
  }
  return BODY_STORAGE_LABEL_KEYS[storage];
}

// ── Raw HTTP reconstruction ─────────────────────────────────────────
//
// ArchScope's live tier is graded `semantic`, not `raw_wire`: it keeps
// canonical header names, values, and duplicates, but not the original field
// order, casing, or framing. The raw view is therefore a faithful *
// reconstruction* of the exchange, and it says so — presenting it as captured
// bytes would be a claim the capture tier does not support.

export type RawHttpMessage = {
  /** The full reconstructed text, ready to render in a <pre>. */
  text: string;
  /** The start line alone, for a compact header. */
  startLine: string;
  /** True when a body is present in the text. */
  hasBody: boolean;
  /** Set when the body is absent; names the reason. */
  bodyNoticeKey: MessageKey | null;
  /** Always true for this tier — the bytes were not preserved verbatim. */
  reconstructed: true;
};

function normalizeVersion(value: string | undefined): string {
  const version = String(value ?? "").trim();
  if (!version) return "HTTP/1.1";
  // The engine reports `HTTP/1.1` from net/http and `http/1.1` from HAR
  // dialects; the start line is conventionally upper-case.
  return version.toUpperCase().startsWith("HTTP/") ? version.toUpperCase() : version;
}

function headerLines(fields: CaptureHTTPMessage["headers"] | undefined): string[] {
  return (fields ?? []).map((field) => `${field.name}: ${field.value}`);
}

function requestTarget(tx: HttpCaptureTransactionRow): string {
  const path = tx.path && tx.path !== "" ? tx.path : "/";
  const query = String(tx.query ?? "").trim();
  return query ? `${path}?${query}` : path;
}

export function buildRawRequest(tx: HttpCaptureTransactionRow): RawHttpMessage {
  const message = tx.request as CaptureHTTPMessage | undefined;
  const startLine = `${(tx.method || "GET").toUpperCase()} ${requestTarget(tx)} ${normalizeVersion(tx.http_version)}`;
  const lines = [startLine];
  // The Host header is part of the exchange but some capture paths carry it
  // only as the transaction host. Synthesize it only when it is genuinely
  // absent, so a captured Host value is never overwritten.
  const headers = headerLines(message?.headers);
  const hasHost = (message?.headers ?? []).some(
    (field) => field.name.toLowerCase() === "host",
  );
  if (!hasHost && tx.host) lines.push(`Host: ${tx.host}`);
  lines.push(...headers);
  return finishRawMessage(lines, message);
}

export function buildRawResponse(tx: HttpCaptureTransactionRow): RawHttpMessage {
  const message = tx.response as CaptureHTTPMessage | undefined;
  const status = Number(tx.status) || 0;
  const reason = String(tx.status_text ?? "").trim();
  const startLine = `${normalizeVersion(tx.http_version)} ${status || ""} ${reason}`.trim();
  const lines = [startLine, ...headerLines(message?.headers)];
  return finishRawMessage(lines, message);
}

function finishRawMessage(
  lines: string[],
  message: CaptureHTTPMessage | undefined,
): RawHttpMessage {
  const startLine = lines[0] ?? "";
  const head = lines.join("\r\n");
  const body = message?.bodyPreview ?? "";
  const noticeKey = bodyUnavailableKey(message);
  return {
    startLine,
    text: body ? `${head}\r\n\r\n${body}` : `${head}\r\n`,
    hasBody: body !== "",
    bodyNoticeKey: noticeKey,
    reconstructed: true,
  };
}

// ── Timing perspective + waterfall ──────────────────────────────────

export type TimingPerspective =
  | "clientProxy"
  | "proxyUpstream"
  | "proxyInternal"
  | "importedHar";

export const TIMING_PERSPECTIVE_LABEL_KEYS: Record<TimingPerspective, MessageKey> = {
  clientProxy: "httpAnalyzerPerspectiveClientProxy",
  proxyUpstream: "httpAnalyzerPerspectiveProxyUpstream",
  proxyInternal: "httpAnalyzerPerspectiveProxyInternal",
  importedHar: "httpAnalyzerPerspectiveImportedHar",
};

/**
 * A single TimingSet cannot hold two perspectives, so the ones present are
 * listed rather than merged. Summing across perspectives is invalid (INV-4),
 * which is why the UI shows one at a time and names it.
 */
export function availablePerspectives(
  tx: HttpCaptureTransactionRow | null,
): TimingPerspective[] {
  const timings = tx?.timings;
  if (!timings) return [];
  const order: TimingPerspective[] = [
    "clientProxy",
    "proxyUpstream",
    "proxyInternal",
    "importedHar",
  ];
  return order.filter((key) => Boolean(timings[key]));
}

export const TIMING_PHASE_ORDER: Array<keyof CaptureTimingPhases> = [
  "blocked",
  "dns",
  "connect",
  "tls",
  "send",
  "wait",
  "receive",
];

export const TIMING_PHASE_LABEL_KEYS: Record<keyof CaptureTimingPhases, MessageKey> = {
  blocked: "httpAnalyzerPhaseBlocked",
  dns: "httpAnalyzerPhaseDns",
  connect: "httpAnalyzerPhaseConnect",
  tls: "httpAnalyzerPhaseTls",
  send: "httpAnalyzerPhaseSend",
  wait: "httpAnalyzerPhaseWait",
  receive: "httpAnalyzerPhaseReceive",
};

export type WaterfallSegment = {
  phase: keyof CaptureTimingPhases;
  ms: number;
  state: string;
  /** Percent offset from the start of the transaction's own timeline. */
  offsetPct: number;
  /** Percent width; zero-length known phases still get a hairline. */
  widthPct: number;
};

export type Waterfall = {
  perspective: TimingPerspective | null;
  segments: WaterfallSegment[];
  /** Sum of the known phases only — never mixed across perspectives. */
  knownMs: number;
  /** True when at least one phase is `unknown`, so knownMs is a floor. */
  hasUnknown: boolean;
};

/**
 * buildWaterfall lays the phases of one perspective end to end. Phases are
 * cumulative within a perspective, so each segment starts where the previous
 * known one ended; `unknown` and `not_applicable` phases occupy no width but
 * stay in the segment list so the detail table can name them.
 */
export function buildWaterfall(
  tx: HttpCaptureTransactionRow | null,
  perspective?: TimingPerspective | null,
): Waterfall {
  const available = availablePerspectives(tx);
  const chosen =
    perspective && available.includes(perspective) ? perspective : available[0] ?? null;
  if (!tx || !chosen) {
    return { perspective: null, segments: [], knownMs: 0, hasUnknown: false };
  }
  const phases = tx.timings?.[chosen];
  if (!phases) {
    return { perspective: null, segments: [], knownMs: 0, hasUnknown: false };
  }

  let knownMs = 0;
  let hasUnknown = false;
  for (const phase of TIMING_PHASE_ORDER) {
    const duration = phases[phase];
    if (duration?.state === "known") knownMs += Math.max(0, duration.ms);
    else if (duration?.state !== "not_applicable") hasUnknown = true;
  }

  const scale = knownMs > 0 ? knownMs : 1;
  let cursor = 0;
  const segments: WaterfallSegment[] = TIMING_PHASE_ORDER.map((phase) => {
    const duration = phases[phase];
    const state = duration?.state ?? "unknown";
    const ms = state === "known" ? Math.max(0, duration!.ms) : 0;
    const offsetPct = (cursor / scale) * 100;
    const widthPct = state === "known" ? (ms / scale) * 100 : 0;
    if (state === "known") cursor += ms;
    return { phase, ms, state, offsetPct, widthPct };
  });

  return { perspective: chosen, segments, knownMs, hasUnknown };
}

// ── Session-relative placement (the grid's timeline column) ─────────

export type SessionSpan = { startMs: number; endMs: number; durationMs: number };

/**
 * sessionSpan is the wall-clock extent of the visible rows. The grid's timeline
 * column places every request inside this span, which is what makes a page
 * load's request cascade legible: the shape of the column *is* the load order.
 * Rows with no parseable start time are excluded rather than pinned to zero.
 */
export function sessionSpan(
  transactions: HttpCaptureTransactionRow[],
): SessionSpan | null {
  let startMs = Number.POSITIVE_INFINITY;
  let endMs = Number.NEGATIVE_INFINITY;
  for (const tx of transactions) {
    const started = Date.parse(tx.started_at ?? "");
    if (Number.isNaN(started)) continue;
    const duration = Number(tx.duration_ms);
    const ended = started + (Number.isFinite(duration) && duration > 0 ? duration : 0);
    if (started < startMs) startMs = started;
    if (ended > endMs) endMs = ended;
  }
  if (!Number.isFinite(startMs) || !Number.isFinite(endMs)) return null;
  return { startMs, endMs, durationMs: Math.max(0, endMs - startMs) };
}

export type TimelinePlacement = { offsetPct: number; widthPct: number } | null;

/**
 * placeOnSession maps one transaction onto the session span. A row with no
 * start time returns null so the UI can leave its lane empty instead of drawing
 * it at the origin, which would read as "this happened first".
 */
export function placeOnSession(
  tx: HttpCaptureTransactionRow,
  span: SessionSpan | null,
): TimelinePlacement {
  if (!span) return null;
  const started = Date.parse(tx.started_at ?? "");
  if (Number.isNaN(started)) return null;
  const total = span.durationMs > 0 ? span.durationMs : 1;
  const duration = Number(tx.duration_ms);
  const width = Number.isFinite(duration) && duration > 0 ? duration : 0;
  const offsetPct = ((started - span.startMs) / total) * 100;
  const widthPct = (width / total) * 100;
  return {
    offsetPct: Math.max(0, Math.min(100, offsetPct)),
    // A sub-pixel request still needs to be visible, so a positive duration
    // never collapses to nothing.
    widthPct: Math.max(width > 0 ? 0.4 : 0, Math.min(100 - offsetPct, widthPct)),
  };
}

// ── Grid sorting ────────────────────────────────────────────────────

export type SortColumn = string;

export type SortDirection = "asc" | "desc";

export type GridSort = { column: SortColumn; direction: SortDirection };

/** The packet list opens in capture order, like every tool of this shape. */
export const defaultGridSort: GridSort = { column: "sequence", direction: "asc" };

function sortValue(tx: HttpCaptureTransactionRow, column: SortColumn): number | string {
  switch (column) {
    case "sequence":
      return Number(tx.sequence) || 0;
    case "started": {
      const started = Date.parse(tx.started_at ?? "");
      return Number.isNaN(started) ? Number.NEGATIVE_INFINITY : started;
    }
    case "method":
      return tx.method ?? "";
    case "status":
      return Number(tx.status) || 0;
    case "host":
      return tx.host ?? "";
    case "path":
      return tx.path ?? "";
    case "kind":
      return resourceKindOf(tx);
    case "size":
      return Number(tx.response_bytes) || 0;
    case "duration": {
      const duration = Number(tx.duration_ms);
      return Number.isFinite(duration) ? duration : Number.NEGATIVE_INFINITY;
    }
    case "process":
      return tx.process?.name ?? "";
    default:
      return httpColumnValue(tx,column);
  }
}

/**
 * sortTransactions is a stable, total order: equal keys fall back to the
 * capture sequence and then the id, so the grid never reshuffles rows that
 * compare equal when the data is refreshed.
 */
export function sortTransactions(
  transactions: HttpCaptureTransactionRow[],
  sort: GridSort,
): HttpCaptureTransactionRow[] {
  const factor = sort.direction === "asc" ? 1 : -1;
  return [...transactions].sort((left, right) => {
    const a = sortValue(left, sort.column);
    const b = sortValue(right, sort.column);
    let comparison = 0;
    if (typeof a === "number" && typeof b === "number") comparison = a - b;
    else comparison = String(a).localeCompare(String(b),undefined,{sensitivity:"base"});
    if (comparison !== 0) return comparison * factor;
    const sequence = (Number(left.sequence) || 0) - (Number(right.sequence) || 0);
    if (sequence !== 0) return sequence;
    return String(left.id).localeCompare(String(right.id));
  });
}

/** Clicking the active column flips it; a new column starts ascending. */
export function toggleSort(current: GridSort, column: SortColumn): GridSort {
  if (current.column !== column) return { column, direction: "asc" };
  return { column, direction: current.direction === "asc" ? "desc" : "asc" };
}

// ── Keyboard navigation over the grid ───────────────────────────────

/**
 * nextSelection resolves an arrow-key move over the visible rows. It returns
 * the id to select, or null when the move is a no-op, so the caller can let the
 * key event through instead of swallowing it at the list boundary.
 */
export function nextSelection(
  transactions: HttpCaptureTransactionRow[],
  selectedId: string | null,
  delta: number,
): string | null {
  if (transactions.length === 0) return null;
  const index = transactions.findIndex((tx) => tx.id === selectedId);
  if (index < 0) return transactions[delta > 0 ? 0 : transactions.length - 1]?.id ?? null;
  const next = index + delta;
  if (next < 0 || next >= transactions.length) return null;
  return transactions[next]?.id ?? null;
}

// A process instance is stable across PID reuse. HAR origins are explicit
// pseudo-processes so imported evidence never implies observed OS attribution.
export function httpProcessKey(row: HttpCaptureTransactionRow): string {
  if (row.process && row.process.pid === 0 && row.process.attribution === "foreign_tool") return "foreign-process:" + row.process.name;
  return row.process
    ? JSON.stringify([row.process.pid, row.process.start_time])
    : "origin:" + row.host;
}

export type HttpGridEntry =
  | { kind: "request"; row: HttpCaptureTransactionRow; index: number }
  | { kind: "process"; key: string; name: string; count: number; sent: number; received: number };

export function httpGroupedEntries(
  rows: HttpCaptureTransactionRow[],
  collapsed: ReadonlySet<string>,
  groupBy = "process",
): HttpGridEntry[] {
  if (groupBy==="none") return rows.map((row,index) => ({kind:"request",row,index}));
  const groups = new Map<string, { header: Extract<HttpGridEntry, { kind: "process" }>; requests: HttpGridEntry[] }>();
  rows.forEach((row, index) => {
    const key = groupBy==="process" ? httpProcessKey(row) : groupBy+":"+httpColumnValue(row,groupBy);
    let group = groups.get(key);
    if (!group) {
      group = {
        header: { kind:"process", key,
          name: groupBy==="process" ? (row.process ? `${row.process.name} [${row.process.pid}] · ${row.process.start_time}` : row.host) : String(httpColumnValue(row,groupBy)),
          count:0, sent:0, received:0 }, requests:[],
      };
      groups.set(key, group);
    }
    group.header.count++;
    group.header.sent += row.request_bytes;
    group.header.received += row.response_bytes;
    group.requests.push({kind:"request",row,index});
  });
  return [...groups.values()].flatMap(({header,requests}) =>
    collapsed.has(header.key) ? [header] : [header,...requests]);
}
