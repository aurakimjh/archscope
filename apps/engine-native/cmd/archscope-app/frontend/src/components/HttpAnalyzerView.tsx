// ─────────────────────────────────────────────────────────────────────
// [한글] HttpAnalyzerView.tsx — HTTP Analyzer / Wireshark 계열 도구와
// 같은 형태의 "상단 패킷 그리드 + 하단 상세 패널" 뷰.
//
// 책임/목적:
//   - 캡처된 모든 트랜잭션(이미지·CSS·스크립트·폰트 포함)을 캡처 순서대로
//     한 그리드에 나열하고, 행 단위 네트워크 타임 워터폴을 함께 보여준다.
//   - 선택한 행의 raw HTTP 프로토콜 텍스트(요청/응답), 헤더, 쿠키, 본문,
//     타이밍 구간, 프로세스 귀속을 하단 탭에서 노출한다.
//   - 목록/상세 분할은 드래그와 키보드 양쪽으로 조절 가능하다.
//
// 순수 파생 로직은 state/httpAnalyzer.ts 에 있고 이 파일은 표현/상호작용만
// 담당한다(httpCapture 페이지와 동일한 분리 규칙).
//
// 정직성 계약: raw 탭은 "재구성"임을 항상 명시한다. 이 캡처 tier 는
// semantic 등급이며 원본 바이트/필드 순서를 보존하지 않는다. 본문이 없을
// 때는 "없었다"와 "기록하지 않았다"를 서로 다른 문장으로 구분한다.
// ─────────────────────────────────────────────────────────────────────
import { Check, Copy, Maximize2, Minimize2 } from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import type {
  CaptureHTTPMessage,
  HttpCaptureTransactionRow,
} from "@/bridge/types";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import {
  CAPTURE_FIDELITY_LABEL_KEYS,
  formatBytes,
  resolveCaptureFidelity,
  statusClassOf,
  type StatusClass,
  type TransactionFilter,
} from "@/state/httpCapture";
import {
  BODY_STORAGE_LABEL_KEYS,
  RESOURCE_KIND_LABEL_KEYS,
  TIMING_PERSPECTIVE_LABEL_KEYS,
  TIMING_PHASE_LABEL_KEYS,
  availablePerspectives,
  availableResourceKinds,
  buildRawRequest,
  buildRawResponse,
  buildWaterfall,
  bodyUnavailableKey,
  defaultGridSort,
  nextSelection,
  placeOnSession,
  resolveBodyStorage,
  resourceKindBreakdown,
  resourceKindOf,
  sessionSpan,
  sortTransactions,
  toggleSort,
  type GridSort,
  type ResourceKind,
  type SortColumn,
  type TimingPerspective,
} from "@/state/httpAnalyzer";
import { formatNumber } from "@/utils/formatters";

const STATUS_CLASSES: StatusClass[] = ["2xx", "3xx", "4xx", "5xx", "other"];

/** Fixed row height keeps the virtual window arithmetic exact. */
const ROW_HEIGHT = 26;
/** Rows rendered above and below the viewport so scrolling never shows gaps. */
const OVERSCAN = 12;

const MIN_DETAIL_HEIGHT = 160;
const MIN_GRID_HEIGHT = 160;
const DEFAULT_DETAIL_HEIGHT = 320;

type Translate = (key: MessageKey) => string;

function statusColor(status: number): string {
  switch (statusClassOf(status)) {
    case "2xx":
      return "text-emerald-600 dark:text-emerald-400";
    case "3xx":
      return "text-sky-600 dark:text-sky-400";
    case "4xx":
      return "text-amber-600 dark:text-amber-400";
    case "5xx":
      return "text-red-600 dark:text-red-400";
    default:
      return "text-muted-foreground";
  }
}

/** Row tint by status class, so a wall of requests still reads at a glance. */
function rowTint(tx: HttpCaptureTransactionRow): string {
  if (tx.state === "failed" || tx.state === "aborted") {
    return "bg-red-500/10";
  }
  switch (statusClassOf(tx.status)) {
    case "4xx":
      return "bg-amber-500/[0.07]";
    case "5xx":
      return "bg-red-500/[0.09]";
    case "3xx":
      return "bg-sky-500/[0.06]";
    default:
      return "";
  }
}

const KIND_DOT: Record<ResourceKind, string> = {
  document: "bg-sky-500",
  stylesheet: "bg-violet-500",
  script: "bg-amber-500",
  image: "bg-emerald-500",
  font: "bg-pink-500",
  media: "bg-orange-500",
  json: "bg-teal-500",
  form: "bg-lime-500",
  text: "bg-slate-400",
  binary: "bg-zinc-500",
  connect: "bg-indigo-400",
  other: "bg-muted-foreground",
};

export type HttpAnalyzerViewProps = {
  /** Rows after filtering, in engine order. */
  transactions: HttpCaptureTransactionRow[];
  /** Unfiltered row count, for the "shown / total" readout. */
  total: number;
  methods: string[];
  mimes: string[];
  fidelities: string[];
  filter: TransactionFilter;
  filterActive: boolean;
  onFilter: (patch: Partial<TransactionFilter>) => void;
  onReset: () => void;
  selectedId: string | null;
  onSelect: (id: string) => void;
  /** True when the engine bounded the row list; drives the truncation note. */
  bounded?: boolean;
  boundedNote?: string;
};

export function HttpAnalyzerView({
  transactions,
  total,
  methods,
  mimes,
  fidelities,
  filter,
  filterActive,
  onFilter,
  onReset,
  selectedId,
  onSelect,
  bounded = false,
  boundedNote,
}: HttpAnalyzerViewProps): React.JSX.Element {
  const { t } = useI18n();
  const [sort, setSort] = useState<GridSort>(defaultGridSort);
  const [detailHeight, setDetailHeight] = useState(DEFAULT_DETAIL_HEIGHT);
  const [maximized, setMaximized] = useState(false);

  const rows = useMemo(() => sortTransactions(transactions, sort), [transactions, sort]);
  const span = useMemo(() => sessionSpan(rows), [rows]);
  const kinds = useMemo(() => availableResourceKinds(transactions), [transactions]);
  const breakdown = useMemo(() => resourceKindBreakdown(transactions), [transactions]);
  const selected = useMemo(
    () => rows.find((row) => row.id === selectedId) ?? null,
    [rows, selectedId],
  );

  const onSort = useCallback(
    (column: SortColumn) => setSort((current) => toggleSort(current, column)),
    [],
  );

  return (
    <section
      className={
        maximized
          ? "fixed inset-0 z-50 flex flex-col bg-background p-3"
          : "flex h-[78dvh] min-h-[520px] flex-col rounded-lg border border-border bg-card"
      }
      aria-label={t("httpAnalyzerTitle")}
    >
      <AnalyzerToolbar
        shown={rows.length}
        total={total}
        methods={methods}
        mimes={mimes}
        fidelities={fidelities}
        kinds={kinds}
        breakdown={breakdown}
        filter={filter}
        filterActive={filterActive}
        onFilter={onFilter}
        onReset={onReset}
        maximized={maximized}
        onToggleMaximize={() => setMaximized((value) => !value)}
        t={t}
      />

      {bounded && boundedNote && (
        <p
          className="border-b border-amber-500/40 bg-amber-500/10 px-3 py-1.5 text-[11px] text-amber-800 dark:text-amber-300"
          role="status"
        >
          {boundedNote}
        </p>
      )}

      <TransactionGrid
        rows={rows}
        span={span}
        sort={sort}
        onSort={onSort}
        selectedId={selectedId}
        onSelect={onSelect}
        t={t}
      />

      <SplitHandle
        height={detailHeight}
        onResize={setDetailHeight}
        t={t}
      />

      <div
        className="shrink-0 overflow-auto border-t border-border"
        style={{ height: detailHeight }}
      >
        {selected ? (
          <TransactionDetailPane transaction={selected} t={t} />
        ) : (
          <p className="p-6 text-center text-sm text-muted-foreground" role="status">
            {t("httpAnalyzerNoSelection")}
          </p>
        )}
      </div>
    </section>
  );
}

// ── Toolbar: filters + resource-kind facets ─────────────────────────

function AnalyzerToolbar({
  shown,
  total,
  methods,
  mimes,
  fidelities,
  kinds,
  breakdown,
  filter,
  filterActive,
  onFilter,
  onReset,
  maximized,
  onToggleMaximize,
  t,
}: {
  shown: number;
  total: number;
  methods: string[];
  mimes: string[];
  fidelities: string[];
  kinds: ResourceKind[];
  breakdown: { kind: ResourceKind; count: number; bytes: number }[];
  filter: TransactionFilter;
  filterActive: boolean;
  onFilter: (patch: Partial<TransactionFilter>) => void;
  onReset: () => void;
  maximized: boolean;
  onToggleMaximize: () => void;
  t: Translate;
}): React.JSX.Element {
  const selectClass =
    "h-8 rounded-md border border-input bg-transparent px-2 text-xs focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring";
  const numberClass =
    "h-8 w-[4.5rem] rounded-md border border-input bg-transparent px-2 text-xs focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring";
  return (
    <div className="flex shrink-0 flex-col gap-2 border-b border-border p-2">
      <div className="flex flex-wrap items-center gap-1.5">
        <Input
          value={filter.query}
          onChange={(event) => onFilter({ query: event.target.value })}
          placeholder={t("httpCaptureFilterQuery")}
          className="h-8 min-w-48 flex-1 text-xs"
          aria-label={t("httpCaptureFilterQuery")}
        />
        <select
          className={selectClass}
          value={filter.method}
          onChange={(event) => onFilter({ method: event.target.value })}
          aria-label={t("httpCaptureFilterMethodAll")}
        >
          <option value="">{t("httpCaptureFilterMethodAll")}</option>
          {methods.map((method) => (
            <option key={method} value={method}>
              {method}
            </option>
          ))}
        </select>
        <select
          className={selectClass}
          value={filter.statusClass}
          onChange={(event) =>
            onFilter({ statusClass: event.target.value as TransactionFilter["statusClass"] })
          }
          aria-label={t("httpCaptureFilterStatusAll")}
        >
          <option value="">{t("httpCaptureFilterStatusAll")}</option>
          {STATUS_CLASSES.map((statusClass) => (
            <option key={statusClass} value={statusClass}>
              {statusClass}
            </option>
          ))}
        </select>
        <select
          className={selectClass}
          value={filter.kind}
          onChange={(event) =>
            onFilter({ kind: event.target.value as TransactionFilter["kind"] })
          }
          aria-label={t("httpAnalyzerFilterKindAll")}
        >
          <option value="">{t("httpAnalyzerFilterKindAll")}</option>
          {kinds.map((kind) => (
            <option key={kind} value={kind}>
              {t(RESOURCE_KIND_LABEL_KEYS[kind])}
            </option>
          ))}
        </select>
        <select
          className={selectClass}
          value={filter.mime}
          onChange={(event) => onFilter({ mime: event.target.value })}
          aria-label={t("httpCaptureFilterMimeAll")}
        >
          <option value="">{t("httpCaptureFilterMimeAll")}</option>
          {mimes.map((mime) => (
            <option key={mime} value={mime}>
              {mime}
            </option>
          ))}
        </select>
        <select
          className={selectClass}
          value={filter.fidelity}
          onChange={(event) => onFilter({ fidelity: event.target.value })}
          aria-label={t("httpCaptureFilterFidelityAll")}
        >
          <option value="">{t("httpCaptureFilterFidelityAll")}</option>
          {fidelities.map((fidelity) => (
            <option key={fidelity} value={fidelity}>
              {t(CAPTURE_FIDELITY_LABEL_KEYS[resolveCaptureFidelity(fidelity)])}
            </option>
          ))}
        </select>
        <div
          className="flex items-center gap-1"
          role="group"
          aria-label={t("httpCaptureFilterDurationRange")}
        >
          <input
            type="number"
            min={0}
            inputMode="numeric"
            className={numberClass}
            value={filter.minDurationMs ?? ""}
            onChange={(event) =>
              onFilter({
                minDurationMs: event.target.value === "" ? null : Number(event.target.value),
              })
            }
            placeholder={t("httpCaptureFilterMinDuration")}
            aria-label={t("httpCaptureFilterMinDuration")}
          />
          <span className="text-muted-foreground">–</span>
          <input
            type="number"
            min={0}
            inputMode="numeric"
            className={numberClass}
            value={filter.maxDurationMs ?? ""}
            onChange={(event) =>
              onFilter({
                maxDurationMs: event.target.value === "" ? null : Number(event.target.value),
              })
            }
            placeholder={t("httpCaptureFilterMaxDuration")}
            aria-label={t("httpCaptureFilterMaxDuration")}
          />
        </div>
        <label className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
          <input
            type="checkbox"
            checked={filter.errorsOnly}
            onChange={(event) => onFilter({ errorsOnly: event.target.checked })}
          />
          {t("httpCaptureFilterErrorsOnly")}
        </label>
        {filterActive && (
          <Button type="button" size="sm" variant="ghost" onClick={onReset}>
            {t("httpCaptureFilterReset")}
          </Button>
        )}
        <span className="ml-auto tabular-nums text-xs text-muted-foreground">
          {formatNumber(shown)} / {formatNumber(total)} {t("httpAnalyzerShown")}
        </span>
        <Button
          type="button"
          size="sm"
          variant="ghost"
          onClick={onToggleMaximize}
          aria-pressed={maximized}
          title={t("httpAnalyzerTitle")}
        >
          {maximized ? <Minimize2 className="h-3.5 w-3.5" /> : <Maximize2 className="h-3.5 w-3.5" />}
        </Button>
      </div>

      {breakdown.length > 0 && (
        <div
          className="flex flex-wrap items-center gap-1"
          role="group"
          aria-label={t("httpAnalyzerResourceBreakdown")}
        >
          <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
            {t("httpAnalyzerResourceBreakdown")}
          </span>
          {breakdown.map((entry) => {
            const active = filter.kind === entry.kind;
            return (
              <button
                key={entry.kind}
                type="button"
                aria-pressed={active}
                onClick={() => onFilter({ kind: active ? "" : entry.kind })}
                className={`inline-flex items-center gap-1 rounded-full border px-1.5 py-0.5 text-[11px] ${
                  active
                    ? "border-primary bg-primary/15 text-primary"
                    : "border-border bg-background text-muted-foreground hover:bg-accent"
                }`}
                title={formatBytes(entry.bytes)}
              >
                <span className={`h-1.5 w-1.5 rounded-full ${KIND_DOT[entry.kind]}`} aria-hidden="true" />
                {t(RESOURCE_KIND_LABEL_KEYS[entry.kind])}
                <span className="font-mono tabular-nums">{formatNumber(entry.count)}</span>
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

// ── Virtualized packet grid ─────────────────────────────────────────

type GridColumn = {
  key: SortColumn | "timeline";
  labelKey: MessageKey;
  width: string;
  align?: "right";
  sortable: boolean;
};

const COLUMNS: GridColumn[] = [
  { key: "sequence", labelKey: "httpAnalyzerColSeq", width: "3.5rem", align: "right", sortable: true },
  { key: "started", labelKey: "httpAnalyzerColStarted", width: "7rem", sortable: true },
  { key: "method", labelKey: "httpAnalyzerColMethod", width: "4.5rem", sortable: true },
  { key: "status", labelKey: "httpAnalyzerColStatus", width: "4rem", align: "right", sortable: true },
  { key: "host", labelKey: "httpAnalyzerColHost", width: "12rem", sortable: true },
  { key: "path", labelKey: "httpAnalyzerColPath", width: "auto", sortable: true },
  { key: "kind", labelKey: "httpAnalyzerColKind", width: "6.5rem", sortable: true },
  { key: "size", labelKey: "httpAnalyzerColSize", width: "5.5rem", align: "right", sortable: true },
  { key: "duration", labelKey: "httpAnalyzerColDuration", width: "5rem", align: "right", sortable: true },
  { key: "timeline", labelKey: "httpAnalyzerColTimeline", width: "14rem", sortable: false },
  { key: "process", labelKey: "httpAnalyzerColProcess", width: "9rem", sortable: true },
];

function TransactionGrid({
  rows,
  span,
  sort,
  onSort,
  selectedId,
  onSelect,
  t,
}: {
  rows: HttpCaptureTransactionRow[];
  span: ReturnType<typeof sessionSpan>;
  sort: GridSort;
  onSort: (column: SortColumn) => void;
  selectedId: string | null;
  onSelect: (id: string) => void;
  t: Translate;
}): React.JSX.Element {
  const scrollRef = useRef<HTMLDivElement | null>(null);
  const [scrollTop, setScrollTop] = useState(0);
  const [viewport, setViewport] = useState(600);

  // The grid holds a whole session — tens of thousands of rows once resource
  // requests are no longer dropped — so only the visible window is mounted.
  useEffect(() => {
    const element = scrollRef.current;
    if (!element) return;
    const measure = () => setViewport(element.clientHeight);
    measure();
    if (typeof ResizeObserver === "undefined") return;
    const observer = new ResizeObserver(measure);
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  const start = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN);
  const end = Math.min(rows.length, Math.ceil((scrollTop + viewport) / ROW_HEIGHT) + OVERSCAN);
  const visible = rows.slice(start, end);
  const padTop = start * ROW_HEIGHT;
  const padBottom = Math.max(0, (rows.length - end) * ROW_HEIGHT);

  // Keep the selected row reachable when the selection moves by keyboard.
  useEffect(() => {
    const element = scrollRef.current;
    if (!element || !selectedId) return;
    const index = rows.findIndex((row) => row.id === selectedId);
    if (index < 0) return;
    const top = index * ROW_HEIGHT;
    const bottom = top + ROW_HEIGHT;
    if (top < element.scrollTop) element.scrollTop = top;
    else if (bottom > element.scrollTop + element.clientHeight) {
      element.scrollTop = bottom - element.clientHeight;
    }
  }, [rows, selectedId]);

  const onKeyDown = useCallback(
    (event: React.KeyboardEvent<HTMLDivElement>) => {
      const delta =
        event.key === "ArrowDown" ? 1 : event.key === "ArrowUp" ? -1 : event.key === "PageDown" ? 10 : event.key === "PageUp" ? -10 : 0;
      if (delta === 0) return;
      const next = nextSelection(rows, selectedId, delta);
      if (next === null) return;
      event.preventDefault();
      onSelect(next);
    },
    [onSelect, rows, selectedId],
  );

  if (rows.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center">
        <p className="text-sm text-muted-foreground" role="status">
          {t("httpAnalyzerEmpty")}
        </p>
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div
        ref={scrollRef}
        className="min-h-0 flex-1 overflow-auto focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
        onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}
        onKeyDown={onKeyDown}
        tabIndex={0}
        role="group"
        aria-label={t("httpAnalyzerTitle")}
      >
        <table className="w-full table-fixed border-collapse text-left text-[11px]">
          <caption className="sr-only">{t("httpAnalyzerGridCaption")}</caption>
          <colgroup>
            {COLUMNS.map((column) => (
              <col key={column.key} style={{ width: column.width }} />
            ))}
          </colgroup>
          <thead className="sticky top-0 z-10 bg-card">
            <tr className="border-b border-border text-muted-foreground">
              {COLUMNS.map((column) => {
                const active = column.sortable && sort.column === column.key;
                return (
                  <th
                    key={column.key}
                    scope="col"
                    aria-sort={
                      active ? (sort.direction === "asc" ? "ascending" : "descending") : undefined
                    }
                    className={`px-2 py-1 font-medium ${column.align === "right" ? "text-right" : ""}`}
                  >
                    {column.sortable ? (
                      <button
                        type="button"
                        onClick={() => onSort(column.key as SortColumn)}
                        className="inline-flex items-center gap-0.5 hover:text-foreground"
                      >
                        {t(column.labelKey)}
                        {active && (
                          <span aria-hidden="true">{sort.direction === "asc" ? "▲" : "▼"}</span>
                        )}
                      </button>
                    ) : (
                      t(column.labelKey)
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {padTop > 0 && (
              <tr aria-hidden="true" style={{ height: padTop }}>
                <td colSpan={COLUMNS.length} />
              </tr>
            )}
            {visible.map((tx, offset) => (
              <GridRow
                key={tx.id}
                tx={tx}
                index={start + offset}
                span={span}
                selected={tx.id === selectedId}
                onSelect={onSelect}
                t={t}
              />
            ))}
            {padBottom > 0 && (
              <tr aria-hidden="true" style={{ height: padBottom }}>
                <td colSpan={COLUMNS.length} />
              </tr>
            )}
          </tbody>
        </table>
      </div>
      <p className="shrink-0 border-t border-border/60 px-2 py-0.5 text-[10px] text-muted-foreground">
        {t("httpAnalyzerKeyboardHint")}
      </p>
    </div>
  );
}

function GridRow({
  tx,
  index,
  span,
  selected,
  onSelect,
  t,
}: {
  tx: HttpCaptureTransactionRow;
  index: number;
  span: ReturnType<typeof sessionSpan>;
  selected: boolean;
  onSelect: (id: string) => void;
  t: Translate;
}): React.JSX.Element {
  const kind = resourceKindOf(tx);
  const placement = placeOnSession(tx, span);
  const duration = Number(tx.duration_ms);
  return (
    <tr
      style={{ height: ROW_HEIGHT }}
      aria-selected={selected}
      className={`cursor-pointer border-b border-border/40 ${
        selected ? "bg-primary/15" : `${rowTint(tx)} hover:bg-accent`
      }`}
      onClick={() => onSelect(tx.id)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect(tx.id);
        }
      }}
      tabIndex={-1}
    >
      <td className="px-2 text-right font-mono tabular-nums text-muted-foreground">{index + 1}</td>
      <td className="truncate px-2 font-mono tabular-nums text-muted-foreground" title={tx.started_at}>
        {clockOf(tx.started_at)}
      </td>
      <td className="px-2 font-mono">{tx.method}</td>
      <td
        className={`px-2 text-right font-mono tabular-nums ${statusColor(tx.status)}`}
        title={tx.status_text || tx.error || tx.state}
      >
        {tx.status || "—"}
      </td>
      <td className="truncate px-2 text-muted-foreground" title={tx.host}>
        {tx.host}
      </td>
      <td className="truncate px-2" title={tx.url || tx.path}>
        {tx.path}
        {tx.query ? <span className="text-muted-foreground">?{tx.query}</span> : null}
      </td>
      <td className="truncate px-2">
        <span className="inline-flex items-center gap-1">
          <span className={`h-1.5 w-1.5 shrink-0 rounded-full ${KIND_DOT[kind]}`} aria-hidden="true" />
          {t(RESOURCE_KIND_LABEL_KEYS[kind])}
        </span>
      </td>
      <td className="px-2 text-right tabular-nums text-muted-foreground">
        {formatBytes(tx.response_bytes)}
      </td>
      <td className="px-2 text-right tabular-nums">
        {Number.isFinite(duration) ? `${Math.round(duration)} ms` : "—"}
      </td>
      <td className="px-2">
        {placement ? (
          <div className="relative h-2 w-full overflow-hidden rounded-sm bg-muted/60" aria-hidden="true">
            <div
              className={`absolute inset-y-0 rounded-sm ${
                statusClassOf(tx.status) === "5xx" || tx.state === "failed"
                  ? "bg-red-500"
                  : statusClassOf(tx.status) === "4xx"
                    ? "bg-amber-500"
                    : "bg-primary/70"
              }`}
              style={{ left: `${placement.offsetPct}%`, width: `${placement.widthPct}%` }}
            />
          </div>
        ) : (
          <span className="text-[10px] text-muted-foreground">—</span>
        )}
      </td>
      <td className="truncate px-2 text-muted-foreground" title={tx.process?.name ?? ""}>
        {tx.process?.name ?? "—"}
      </td>
    </tr>
  );
}

/** HH:MM:SS.mmm of an ISO instant, or the raw value when it will not parse. */
function clockOf(value: string | undefined): string {
  if (!value) return "—";
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) return value;
  return new Date(parsed).toISOString().slice(11, 23);
}

// ── Split handle ────────────────────────────────────────────────────

function SplitHandle({
  height,
  onResize,
  t,
}: {
  height: number;
  onResize: (next: number) => void;
  t: Translate;
}): React.JSX.Element {
  const dragging = useRef<{ startY: number; startHeight: number } | null>(null);

  const clamp = useCallback((next: number) => {
    const maximum = Math.max(MIN_DETAIL_HEIGHT, window.innerHeight - MIN_GRID_HEIGHT);
    return Math.max(MIN_DETAIL_HEIGHT, Math.min(maximum, next));
  }, []);

  const onPointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      event.currentTarget.setPointerCapture(event.pointerId);
      dragging.current = { startY: event.clientY, startHeight: height };
    },
    [height],
  );

  const onPointerMove = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      const state = dragging.current;
      if (!state) return;
      // Dragging up grows the detail pane, which is what the handle sits on.
      onResize(clamp(state.startHeight + (state.startY - event.clientY)));
    },
    [clamp, onResize],
  );

  const onPointerUp = useCallback(() => {
    dragging.current = null;
  }, []);

  return (
    <div
      role="separator"
      aria-orientation="horizontal"
      aria-label={t("httpAnalyzerSplitHandle")}
      aria-valuenow={height}
      tabIndex={0}
      className="group flex h-2 shrink-0 cursor-row-resize items-center justify-center border-t border-border bg-muted/40 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onKeyDown={(event) => {
        const delta = event.key === "ArrowUp" ? 24 : event.key === "ArrowDown" ? -24 : 0;
        if (delta === 0) return;
        event.preventDefault();
        onResize(clamp(height + delta));
      }}
      title={t("httpAnalyzerSplitHandleHint")}
    >
      <span className="h-0.5 w-10 rounded-full bg-border group-hover:bg-primary/60" aria-hidden="true" />
    </div>
  );
}

// ── Detail pane ─────────────────────────────────────────────────────

type DetailTab = "raw" | "headers" | "body" | "timing" | "cookies" | "process";

const DETAIL_TABS: Array<{ key: DetailTab; labelKey: MessageKey }> = [
  { key: "raw", labelKey: "httpAnalyzerTabRaw" },
  { key: "headers", labelKey: "httpAnalyzerTabHeaders" },
  { key: "body", labelKey: "httpAnalyzerTabBody" },
  { key: "timing", labelKey: "httpAnalyzerTabTiming" },
  { key: "cookies", labelKey: "httpAnalyzerTabCookies" },
  { key: "process", labelKey: "httpAnalyzerTabProcess" },
];

function TransactionDetailPane({
  transaction,
  t,
}: {
  transaction: HttpCaptureTransactionRow;
  t: Translate;
}): React.JSX.Element {
  const [tab, setTab] = useState<DetailTab>("raw");
  const perspectives = useMemo(() => availablePerspectives(transaction), [transaction]);
  const [perspective, setPerspective] = useState<TimingPerspective | null>(null);

  // A new selection resets the pane so the tab and perspective always describe
  // the row on screen.
  const openId = transaction.id;
  const shownFor = useRef<string>("");
  if (shownFor.current !== openId) {
    shownFor.current = openId;
    if (tab !== "raw") setTab("raw");
    if (perspective !== null) setPerspective(null);
  }

  return (
    <div className="flex flex-col">
      <div className="sticky top-0 z-10 flex flex-wrap items-center gap-2 border-b border-border bg-card px-3 py-1.5">
        <span className={`font-mono text-xs font-semibold ${statusColor(transaction.status)}`}>
          {transaction.status || transaction.state}
        </span>
        <span className="font-mono text-xs">{transaction.method}</span>
        <span className="min-w-0 flex-1 truncate font-mono text-xs" title={transaction.url}>
          {transaction.url || `${transaction.host}${transaction.path}`}
        </span>
        <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
          {transaction.used_existing_connection
            ? t("httpAnalyzerReusedConnection")
            : t("httpAnalyzerNewConnection")}
        </span>
        <span
          className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground"
          title={transaction.fidelity}
        >
          {t(CAPTURE_FIDELITY_LABEL_KEYS[resolveCaptureFidelity(transaction.fidelity)])}
        </span>
      </div>

      <div role="tablist" aria-label={t("httpAnalyzerTitle")} className="flex gap-1 border-b border-border px-2">
        {DETAIL_TABS.map((entry) => (
          <button
            key={entry.key}
            type="button"
            role="tab"
            id={`http-analyzer-tab-${entry.key}`}
            aria-selected={tab === entry.key}
            aria-controls={`http-analyzer-panel-${entry.key}`}
            onClick={() => setTab(entry.key)}
            className={`-mb-px border-b-2 px-2.5 py-1 text-xs font-medium ${
              tab === entry.key
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            {t(entry.labelKey)}
          </button>
        ))}
      </div>

      <div
        role="tabpanel"
        id={`http-analyzer-panel-${tab}`}
        aria-labelledby={`http-analyzer-tab-${tab}`}
        className="p-3"
      >
        {tab === "raw" && <RawTab transaction={transaction} t={t} />}
        {tab === "headers" && <HeadersTab transaction={transaction} t={t} />}
        {tab === "body" && <BodyTab transaction={transaction} t={t} />}
        {tab === "timing" && (
          <TimingTab
            transaction={transaction}
            perspectives={perspectives}
            perspective={perspective}
            onPerspective={setPerspective}
            t={t}
          />
        )}
        {tab === "cookies" && <CookiesTab transaction={transaction} t={t} />}
        {tab === "process" && <ProcessTab transaction={transaction} t={t} />}
      </div>
    </div>
  );
}

// ── Raw protocol text ───────────────────────────────────────────────

function RawTab({
  transaction,
  t,
}: {
  transaction: HttpCaptureTransactionRow;
  t: Translate;
}): React.JSX.Element {
  const request = useMemo(() => buildRawRequest(transaction), [transaction]);
  const response = useMemo(() => buildRawResponse(transaction), [transaction]);
  return (
    <div className="flex flex-col gap-2">
      <p className="rounded-md border border-border bg-muted/30 p-2 text-[11px] text-muted-foreground">
        {t("httpAnalyzerRawReconstructed")}
      </p>
      <div className="grid gap-2 lg:grid-cols-2">
        <RawPane title={t("httpAnalyzerRequestSide")} message={request} t={t} />
        <RawPane title={t("httpAnalyzerResponseSide")} message={response} t={t} />
      </div>
    </div>
  );
}

function RawPane({
  title,
  message,
  t,
}: {
  title: string;
  message: ReturnType<typeof buildRawRequest>;
  t: Translate;
}): React.JSX.Element {
  return (
    <section className="min-w-0">
      <div className="mb-1 flex items-center gap-2">
        <h4 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
          {title}
        </h4>
        <CopyButton value={message.text} t={t} />
      </div>
      <pre className="max-h-80 overflow-auto whitespace-pre-wrap break-all rounded border border-border bg-muted/20 p-2 font-mono text-[11px] leading-relaxed">
        {message.text}
      </pre>
      {!message.hasBody && message.bodyNoticeKey && (
        <p className="mt-1 text-[11px] text-muted-foreground">{t(message.bodyNoticeKey)}</p>
      )}
    </section>
  );
}

function CopyButton({ value, t }: { value: string; t: Translate }): React.JSX.Element {
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 1500);
    return () => clearTimeout(timer);
  }, [copied]);
  return (
    <button
      type="button"
      className="inline-flex items-center gap-1 rounded border border-border px-1.5 py-0.5 text-[10px] text-muted-foreground hover:bg-accent"
      onClick={() => {
        void navigator.clipboard?.writeText(value).then(() => setCopied(true));
      }}
    >
      {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
      {copied ? t("httpAnalyzerCopied") : t("httpAnalyzerCopy")}
    </button>
  );
}

// ── Headers / cookies ───────────────────────────────────────────────

function HeadersTab({
  transaction,
  t,
}: {
  transaction: HttpCaptureTransactionRow;
  t: Translate;
}): React.JSX.Element {
  return (
    <div className="grid gap-3 lg:grid-cols-2">
      <FieldTable
        title={t("httpAnalyzerRequestSide")}
        fields={transaction.request?.headers ?? []}
        emptyLabel={t("httpCaptureDetailNoHeaders")}
        redactedLabel={t("httpCaptureDetailRedactedBadge")}
      />
      <FieldTable
        title={t("httpAnalyzerResponseSide")}
        fields={transaction.response?.headers ?? []}
        emptyLabel={t("httpCaptureDetailNoHeaders")}
        redactedLabel={t("httpCaptureDetailRedactedBadge")}
      />
    </div>
  );
}

function CookiesTab({
  transaction,
  t,
}: {
  transaction: HttpCaptureTransactionRow;
  t: Translate;
}): React.JSX.Element {
  return (
    <div className="grid gap-3 lg:grid-cols-2">
      <FieldTable
        title={t("httpAnalyzerRequestSide")}
        fields={transaction.request?.cookies ?? []}
        emptyLabel={t("httpCaptureDetailNoCookies")}
        redactedLabel={t("httpCaptureDetailRedactedBadge")}
      />
      <FieldTable
        title={t("httpAnalyzerResponseSide")}
        fields={transaction.response?.cookies ?? []}
        emptyLabel={t("httpCaptureDetailNoCookies")}
        redactedLabel={t("httpCaptureDetailRedactedBadge")}
      />
    </div>
  );
}

function FieldTable({
  title,
  fields,
  emptyLabel,
  redactedLabel,
}: {
  title: string;
  fields: CaptureHTTPMessage["headers"];
  emptyLabel: string;
  redactedLabel: string;
}): React.JSX.Element {
  return (
    <section className="min-w-0">
      <h4 className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
        {title}
      </h4>
      {fields.length === 0 ? (
        <p className="text-[11px] text-muted-foreground">{emptyLabel}</p>
      ) : (
        <table className="w-full text-left text-[11px]">
          <tbody>
            {fields.map((field, index) => (
              <tr key={`${field.name}-${index}`} className="border-t border-border/50 align-top">
                <td className="w-1/3 py-0.5 pr-2 font-mono font-medium">{field.name}</td>
                <td className="break-all py-0.5 font-mono text-muted-foreground">
                  {field.value}
                  {field.redacted && (
                    <span className="ml-1 rounded bg-amber-500/20 px-1 text-[10px] text-amber-700 dark:text-amber-400">
                      {redactedLabel}
                    </span>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </section>
  );
}

// ── Body ────────────────────────────────────────────────────────────

function BodyTab({
  transaction,
  t,
}: {
  transaction: HttpCaptureTransactionRow;
  t: Translate;
}): React.JSX.Element {
  return (
    <div className="grid gap-3 lg:grid-cols-2">
      <BodyPane title={t("httpAnalyzerRequestSide")} message={transaction.request} t={t} />
      <BodyPane title={t("httpAnalyzerResponseSide")} message={transaction.response} t={t} />
    </div>
  );
}

function BodyPane({
  title,
  message,
  t,
}: {
  title: string;
  message: CaptureHTTPMessage | undefined;
  t: Translate;
}): React.JSX.Element {
  const storage = resolveBodyStorage(message?.bodyStorage);
  const noticeKey = bodyUnavailableKey(message);
  return (
    <section className="min-w-0">
      <div className="mb-1 flex flex-wrap items-center gap-2">
        <h4 className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
          {title}
        </h4>
        <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
          {message?.contentType || "—"}
        </span>
        <span className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
          {formatBytes(message?.bodySize)}
        </span>
        {message?.bodyEncoding && (
          <span className="rounded bg-muted px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground">
            {message.bodyEncoding}
          </span>
        )}
        {message?.bodyPreview && <CopyButton value={message.bodyPreview} t={t} />}
      </div>
      {message?.redacted && (
        <p
          className="mb-1 rounded border border-amber-500/50 bg-amber-500/10 p-1.5 text-[11px] text-amber-700 dark:text-amber-400"
          role="status"
        >
          {t("httpCaptureDetailMessageRedacted")}
        </p>
      )}
      {message?.bodyPreview ? (
        <>
          {storage === "truncated" && (
            <p className="mb-1 text-[11px] text-amber-700 dark:text-amber-400">
              {t(BODY_STORAGE_LABEL_KEYS.truncated)}
            </p>
          )}
          <pre className="max-h-80 overflow-auto whitespace-pre-wrap break-all rounded border border-border bg-muted/20 p-2 font-mono text-[11px] leading-relaxed">
            {message.bodyPreview}
          </pre>
        </>
      ) : (
        <p className="rounded border border-border bg-muted/20 p-2 text-[11px] text-muted-foreground" role="status">
          {noticeKey ? t(noticeKey) : t("httpAnalyzerBodyUnknown")}
        </p>
      )}
    </section>
  );
}

// ── Timing ──────────────────────────────────────────────────────────

function TimingTab({
  transaction,
  perspectives,
  perspective,
  onPerspective,
  t,
}: {
  transaction: HttpCaptureTransactionRow;
  perspectives: TimingPerspective[];
  perspective: TimingPerspective | null;
  onPerspective: (value: TimingPerspective) => void;
  t: Translate;
}): React.JSX.Element {
  const waterfall = useMemo(
    () => buildWaterfall(transaction, perspective),
    [perspective, transaction],
  );

  if (perspectives.length === 0 || waterfall.perspective === null) {
    return (
      <p className="text-xs text-muted-foreground" role="status">
        {t("httpAnalyzerNoTimings")}
      </p>
    );
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
          {t("httpAnalyzerTimingPerspective")}
        </span>
        {/* Perspectives are selected, never merged: a single TimingSet cannot
            hold two of them and their phases must not be summed together. */}
        {perspectives.map((value) => (
          <button
            key={value}
            type="button"
            aria-pressed={waterfall.perspective === value}
            onClick={() => onPerspective(value)}
            className={`rounded-full border px-2 py-0.5 text-[11px] ${
              waterfall.perspective === value
                ? "border-primary bg-primary/15 text-primary"
                : "border-border text-muted-foreground hover:bg-accent"
            }`}
          >
            {t(TIMING_PERSPECTIVE_LABEL_KEYS[value])}
          </button>
        ))}
        <span className="ml-auto font-mono text-[11px] tabular-nums">
          {t("httpAnalyzerKnownSum")}: {waterfall.knownMs.toFixed(1)} ms
        </span>
      </div>

      <p className="text-[11px] text-muted-foreground">{t("httpAnalyzerPerspectiveNote")}</p>
      {waterfall.perspective === "proxyUpstream" && (
        <p className="rounded-md border border-sky-500/40 bg-sky-500/10 p-2 text-[11px] text-sky-800 dark:text-sky-300">
          {t("httpAnalyzerPerspectiveUpstreamNote")}
        </p>
      )}
      {waterfall.hasUnknown && (
        <p className="text-[11px] text-amber-700 dark:text-amber-400">
          {t("httpAnalyzerKnownSumFloor")}
        </p>
      )}

      <ul className="flex flex-col gap-0.5">
        {waterfall.segments.map((segment) => (
          <li key={segment.phase} className="flex items-center gap-2">
            <span className="w-24 shrink-0 text-[11px] text-muted-foreground">
              {t(TIMING_PHASE_LABEL_KEYS[segment.phase])}
            </span>
            <div className="relative h-3 flex-1 overflow-hidden rounded-sm bg-muted/60">
              {segment.state === "known" && (
                <div
                  className="absolute inset-y-0 rounded-sm bg-primary/70"
                  style={{
                    left: `${segment.offsetPct}%`,
                    // A measured-but-zero phase still gets a hairline so it is
                    // visibly distinct from one that was never measured.
                    width: `${Math.max(segment.widthPct, 0.6)}%`,
                  }}
                />
              )}
            </div>
            <span
              className={`w-24 shrink-0 text-right font-mono text-[11px] tabular-nums ${
                segment.state === "known" ? "" : "text-muted-foreground"
              }`}
            >
              {segment.state === "known"
                ? `${segment.ms.toFixed(1)} ms`
                : segment.state === "not_applicable"
                  ? t("httpAnalyzerPhaseNotApplicable")
                  : t("httpAnalyzerPhaseUnknown")}
            </span>
          </li>
        ))}
      </ul>

      <dl className="grid grid-cols-2 gap-2 pt-1 text-[11px] sm:grid-cols-4">
        <DetailField label={t("httpCaptureColDuration")} value={`${Math.round(Number(transaction.duration_ms) || 0)} ms`} />
        <DetailField label={t("httpCaptureDetailConnection")} value={transaction.connection_id || "—"} />
        <DetailField label={t("httpCaptureDialectLabel")} value={transaction.http_version || "—"} />
        <DetailField label={t("httpAnalyzerColStarted")} value={transaction.started_at || "—"} />
      </dl>
    </div>
  );
}

// ── Process ─────────────────────────────────────────────────────────

function ProcessTab({
  transaction,
  t,
}: {
  transaction: HttpCaptureTransactionRow;
  t: Translate;
}): React.JSX.Element {
  const process = transaction.process;
  if (!process) {
    return (
      <p className="text-xs text-muted-foreground" role="status">
        {t("httpCaptureDetailNoProcess")}
      </p>
    );
  }
  return (
    <dl className="grid grid-cols-2 gap-2 text-[11px] sm:grid-cols-4">
      <DetailField label={t("httpAnalyzerColProcess")} value={process.name || "—"} />
      <DetailField label="PID" value={String(process.pid)} />
      <DetailField label={t("httpCaptureTreePseudo")} value={process.attribution || "—"} />
      <DetailField label="exec" value={process.exec_path || "—"} />
    </dl>
  );
}

function DetailField({ label, value }: { label: string; value: string }): React.JSX.Element {
  return (
    <div className="min-w-0">
      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
        {label}
      </dt>
      <dd className="truncate font-mono" title={value}>
        {value}
      </dd>
    </div>
  );
}
