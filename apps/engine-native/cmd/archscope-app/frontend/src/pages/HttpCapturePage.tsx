// ─────────────────────────────────────────────────────────────────────
// [한글] HttpCapturePage.tsx — 오프라인 HAR(HTTP capture) 분석 페이지.
//
// 책임/목적: engine `http_capture` 결과를 요약 메트릭, 캡처 충실도/리댁션
// 배너, 요청 타임라인+브러시, 유사(pseudo) 프로세스 트리, 필터+트랜잭션
// 목록/상세 로 렌더. 순수 파생 로직은 state/httpCapture.ts 에 있고 이
// 파일은 표현/상호작용만 담당 (browserCpuProfile 패턴과 동일).
//
// 이 페이지는 의도적으로 import 전용입니다 — 라이브 프록시 능력을
// 암시하지 않고 HAR 충실도/원본 형식을 그대로 노출합니다.
//
// 분모 계약(H-RG1 U1): brush 선택이나 필터가 활성화되면 요약 카드는 engine
// 전체 summary 대신 목록·트리와 동일한 필터링 행에서 재집계한 값을 보여주며,
// 상세 행 상한 때문에 이 값이 하한임을 배너로 명시합니다. Provenance(U3)는
// state/httpCapture.ts 의 순수 reducer 가 관리합니다.
// ─────────────────────────────────────────────────────────────────────
import { Loader2, Play } from "lucide-react";
import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";

import { engine } from "@/bridge/engine";
import type {
  CaptureHTTPMessage,
  HttpCaptureTimelineBucket,
  HttpCaptureTransactionRow,
} from "@/bridge/types";
import { ErrorPanel } from "@/components/AnalyzerFeedback";
import { DiagnosticsPanel } from "@/components/DiagnosticsPanel";
import { MetricCard } from "@/components/MetricCard";
import { SlideOverPanel } from "@/components/SlideOverPanel";
import { WailsFileDock, type FileDockSelection } from "@/components/WailsFileDock";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import { addWorkspaceResult } from "@/state/analysisWorkspace";
import {
  availableFidelities,
  availableMethods,
  availableMimeTypes,
  buildProcessTree,
  emptyFilter,
  extractCaptureMeta,
  extractHttpDiagnostics,
  extractRedaction,
  filterTransactions,
  formatBytes,
  httpCaptureReducer,
  httpDiagnosticIssueCount,
  initialHttpCaptureState,
  isFilterActive,
  projectSummary,
  selectFindings,
  selectHttpSummary,
  selectTimeline,
  selectTransactions,
  statusClassOf,
  timelineWindow,
  timingBreakdown,
  type ProcessTreeNode,
  type StatusClass,
  type SummaryProjection,
  type TransactionFilter,
} from "@/state/httpCapture";
import { formatMilliseconds, formatNumber } from "@/utils/formatters";

const STATUS_CLASSES: StatusClass[] = ["2xx", "3xx", "4xx", "5xx", "other"];

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

export function HttpCapturePage(): React.JSX.Element {
  const { t } = useI18n();
  const [file, setFile] = useState<FileDockSelection | null>(null);
  const [state, dispatch] = useReducer(httpCaptureReducer, initialHttpCaptureState);
  const { running, result, error, filter, selectedId } = state;

  const analyze = useCallback(async () => {
    if (!file || running) return;
    dispatch({ type: "analyzeStart" });
    try {
      const next = await engine.analyzeHttpCapture({ path: file.filePath, format: "auto" });
      dispatch({ type: "analyzeSuccess", result: next, source: file.originalName });
      addWorkspaceResult({
        result: next,
        title: `http_capture: ${file.originalName}`,
        sourceLabel: file.originalName,
      });
    } catch (caught) {
      dispatch({
        type: "analyzeError",
        error: {
          code: "HTTP_CAPTURE_IMPORT_FAILED",
          message: caught instanceof Error ? caught.message : String(caught),
        },
      });
    }
  }, [file, running]);

  // Selecting or clearing a file drops any prior result so a stale success
  // can never render under the new source (U3 provenance).
  const onSelectFile = useCallback((selection: FileDockSelection) => {
    setFile(selection);
    dispatch({ type: "reset" });
  }, []);
  const onClearFile = useCallback(() => {
    setFile(null);
    dispatch({ type: "reset" });
  }, []);

  const summary = useMemo(() => selectHttpSummary(result), [result]);
  const captureMeta = useMemo(() => extractCaptureMeta(result), [result]);
  const redaction = useMemo(() => extractRedaction(result), [result]);
  const timeline = useMemo(() => selectTimeline(result), [result]);
  const allTransactions = useMemo(() => selectTransactions(result), [result]);
  const methods = useMemo(() => availableMethods(allTransactions), [allTransactions]);
  const mimes = useMemo(() => availableMimeTypes(allTransactions), [allTransactions]);
  const fidelities = useMemo(() => availableFidelities(allTransactions), [allTransactions]);
  const filtered = useMemo(
    () => filterTransactions(allTransactions, filter),
    [allTransactions, filter],
  );
  const filterActive = isFilterActive(filter);
  const projection = useMemo(() => projectSummary(filtered), [filtered]);
  const tree = useMemo(() => buildProcessTree(filtered), [filtered]);
  const diagnostics = useMemo(() => extractHttpDiagnostics(result), [result]);
  const diagnosticIssues = httpDiagnosticIssueCount(diagnostics);
  const findings = useMemo(() => selectFindings(result), [result]);
  const selected = useMemo(
    () => allTransactions.find((row) => row.id === selectedId) ?? null,
    [allTransactions, selectedId],
  );
  // The inline detail table is bounded, so recomputed selection metrics are a
  // floor over the visible rows rather than the full session.
  const boundedDetail = captureMeta
    ? captureMeta.transaction_rows < captureMeta.transactions
    : false;

  const patchFilter = useCallback(
    (patch: Partial<TransactionFilter>) => dispatch({ type: "patchFilter", patch }),
    [],
  );
  const resetFilter = useCallback(() => dispatch({ type: "setFilter", filter: emptyFilter }), []);
  const openById = useCallback((id: string) => dispatch({ type: "select", id }), []);
  const closeDetail = useCallback(() => dispatch({ type: "closeDetail" }), []);

  return (
    <main className="flex flex-col gap-5 p-5">
      <WailsFileDock
        label={t("httpCaptureLabel")}
        description={t("httpCaptureDescription")}
        accept=".har,.json"
        selected={file}
        onSelect={onSelectFile}
        onClear={onClearFile}
        browseLabel={t("browseFile")}
        dropHereLabel={t("dropHere")}
        errorLabel={t("error")}
        fileFilters={[
          { displayName: t("httpCaptureFilterHar"), pattern: "*.har;*.json" },
          { displayName: "All files", pattern: "*.*" },
        ]}
        rightSlot={
          <Button type="button" size="sm" disabled={!file || running} onClick={() => void analyze()}>
            {running ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                {t("analyzing")}
              </>
            ) : (
              <>
                <Play className="h-3.5 w-3.5" />
                {t("analyze")}
              </>
            )}
          </Button>
        }
      />
      <ErrorPanel error={error} labels={{ title: t("analysisError"), code: t("errorCode") }} />

      {!result && !error && (
        <Card>
          <CardContent className="py-8 text-center text-sm text-muted-foreground">
            {t("httpCaptureEmpty")}
          </CardContent>
        </Card>
      )}

      {result && summary && (
        <section className="flex flex-col gap-2">
          <ScopeBanner
            active={filterActive}
            bounded={boundedDetail}
            shown={filtered.length}
            detailTotal={allTransactions.length}
            t={t}
          />
          <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-6">
            {filterActive ? (
              <ProjectionCards projection={projection} t={t} />
            ) : (
              <>
                <MetricCard label={t("httpCaptureMetricTransactions")} value={formatNumber(summary.total_transactions)} />
                <MetricCard
                  label={t("httpCaptureMetricErrorRate")}
                  value={`${(summary.error_rate * 100).toFixed(1)}%`}
                />
                <MetricCard label={t("httpCaptureMetricHosts")} value={formatNumber(summary.unique_hosts)} />
                <MetricCard label={t("httpCaptureMetricEndpoints")} value={formatNumber(summary.unique_endpoints)} />
                <MetricCard
                  label={t("httpCaptureMetricP95")}
                  value={formatMilliseconds(Math.round(summary.duration_p95_ms))}
                />
                <MetricCard label={t("httpCaptureMetricResponseBytes")} value={formatBytes(summary.response_bytes)} />
              </>
            )}
          </div>
        </section>
      )}

      {captureMeta && <FidelityBanner meta={captureMeta} redaction={redaction} t={t} />}

      {result && (
        <TimelineBrush
          buckets={timeline}
          available={summary?.timeline_available ?? false}
          window={filter.window}
          onSelect={(window) => patchFilter({ window })}
          t={t}
        />
      )}

      {result && (
        <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)]">
          <ProcessTreePanel tree={tree} onOpen={openById} t={t} />
          <TransactionListPanel
            transactions={filtered}
            total={allTransactions.length}
            methods={methods}
            mimes={mimes}
            fidelities={fidelities}
            filter={filter}
            filterActive={filterActive}
            onFilter={patchFilter}
            onReset={resetFilter}
            onOpen={openById}
            t={t}
          />
        </div>
      )}

      {result && findings.length > 0 && (
        <Card>
          <CardHeader>
            <CardTitle>{t("httpCaptureFindingsTitle")}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-col gap-2">
            {findings.map((finding, index) => (
              <div key={`${finding.code}-${index}`} className="flex items-start gap-2 text-sm">
                <SeverityDot severity={finding.severity} />
                <div className="min-w-0">
                  <code className="text-xs font-semibold">{finding.code}</code>
                  <p className="text-muted-foreground">{finding.message}</p>
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {result && (
        <Card>
          <CardHeader>
            <CardTitle className="inline-flex items-center gap-2">
              {t("httpCaptureDiagnosticsTitle")}
              {diagnosticIssues > 0 && (
                <span className="rounded-full bg-amber-500/20 px-1.5 py-0.5 text-[10px] font-bold text-amber-700 dark:text-amber-400">
                  {diagnosticIssues}
                </span>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent>
            <DiagnosticsPanel diagnostics={diagnostics as never} />
          </CardContent>
        </Card>
      )}

      <TransactionDetail transaction={selected} onClose={closeDetail} t={t} />
    </main>
  );
}

type Translate = (key: MessageKey) => string;

function SeverityDot({ severity }: { severity: string }): React.JSX.Element {
  const color =
    severity === "critical" || severity === "error"
      ? "bg-red-500"
      : severity === "warning"
        ? "bg-amber-500"
        : "bg-sky-500";
  return <span className={`mt-1.5 h-2 w-2 shrink-0 rounded-full ${color}`} aria-hidden="true" />;
}

// ── Summary scope (shared denominator) ──────────────────────────────

function ScopeBanner({
  active,
  bounded,
  shown,
  detailTotal,
  t,
}: {
  active: boolean;
  bounded: boolean;
  shown: number;
  detailTotal: number;
  t: Translate;
}): React.JSX.Element {
  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs" role="status">
      <span
        className={`rounded-full px-2 py-0.5 font-medium ${
          active
            ? "bg-primary/15 text-primary"
            : "bg-muted text-muted-foreground"
        }`}
      >
        {active ? t("httpCaptureScopeSelection") : t("httpCaptureScopeSession")}
      </span>
      {active && (
        <span className="tabular-nums text-muted-foreground">
          {formatNumber(shown)} / {formatNumber(detailTotal)}
        </span>
      )}
      <span className="text-muted-foreground">
        {active ? t("httpCaptureScopeSelectionNote") : t("httpCaptureScopeSessionNote")}
      </span>
      {active && bounded && (
        <span className="text-amber-700 dark:text-amber-400">{t("httpCaptureScopeBoundedNote")}</span>
      )}
    </div>
  );
}

function ProjectionCards({
  projection,
  t,
}: {
  projection: SummaryProjection;
  t: Translate;
}): React.JSX.Element {
  return (
    <>
      <MetricCard label={t("httpCaptureMetricTransactions")} value={formatNumber(projection.transactions)} />
      <MetricCard
        label={t("httpCaptureMetricErrorRate")}
        value={`${(projection.errorRate * 100).toFixed(1)}%`}
      />
      <MetricCard label={t("httpCaptureMetricHosts")} value={formatNumber(projection.uniqueHosts)} />
      <MetricCard label={t("httpCaptureMetricEndpoints")} value={formatNumber(projection.uniqueEndpoints)} />
      <MetricCard
        label={t("httpCaptureMetricP95")}
        value={formatMilliseconds(Math.round(projection.durationP95Ms))}
      />
      <MetricCard label={t("httpCaptureMetricResponseBytes")} value={formatBytes(projection.responseBytes)} />
    </>
  );
}

// ── Fidelity / redaction banner ─────────────────────────────────────

function FidelityBanner({
  meta,
  redaction,
  t,
}: {
  meta: NonNullable<ReturnType<typeof extractCaptureMeta>>;
  redaction: ReturnType<typeof extractRedaction>;
  t: Translate;
}): React.JSX.Element {
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("httpCaptureFidelityTitle")}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <dl className="grid gap-3 text-sm sm:grid-cols-2 lg:grid-cols-5">
          <Field label={t("httpCaptureFidelityLabel")} value={meta.fidelity} />
          <Field label={t("httpCaptureModeLabel")} value={meta.capture_mode} />
          <Field label={t("httpCaptureObservationLabel")} value={meta.observation_point} />
          <Field label={t("httpCaptureDialectLabel")} value={meta.dialect} />
          <Field label={t("httpCaptureDetailStorageLabel")} value={meta.detail_storage} />
        </dl>
        <p className="text-xs text-muted-foreground">{t("httpCaptureFidelityHint")}</p>
        {meta.truncated && (
          <p
            className="rounded-md border border-amber-500/50 bg-amber-500/10 p-2 text-xs text-amber-700 dark:text-amber-400"
            role="status"
          >
            {t("httpCaptureTruncatedNote")}
          </p>
        )}
        <div className="rounded-md border border-border bg-muted/20 p-3">
          <div className="mb-1 flex flex-wrap items-center gap-2">
            <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
              {t("httpCaptureRedactionTitle")}
            </span>
            {redaction?.applied ? (
              <span className="rounded-full bg-emerald-500/15 px-2 py-0.5 text-[11px] font-medium text-emerald-700 dark:text-emerald-400">
                {formatNumber(redaction.total)} · {redaction.version}
              </span>
            ) : (
              <span className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                {t("httpCaptureRedactionNone")}
              </span>
            )}
          </div>
          <p className="text-xs text-muted-foreground">
            {redaction?.applied ? t("httpCaptureRedactionApplied") : t("httpCaptureRedactionNone")}
          </p>
          {redaction?.applied && redaction.rules.length > 0 && (
            <ul className="mt-2 flex flex-wrap gap-1.5">
              {redaction.rules.map((rule) => (
                <li
                  key={rule}
                  className="rounded border border-border bg-background px-1.5 py-0.5 font-mono text-[11px]"
                >
                  {rule}
                  {typeof redaction.counts[rule] === "number" && (
                    <span className="ml-1 text-muted-foreground">×{redaction.counts[rule]}</span>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

function Field({ label, value }: { label: string; value: string }): React.JSX.Element {
  return (
    <div className="min-w-0">
      <dt className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">{label}</dt>
      <dd className="truncate font-mono text-xs" title={value}>
        {value || "-"}
      </dd>
    </div>
  );
}

// ── Timeline + brush ────────────────────────────────────────────────

function TimelineBrush({
  buckets,
  available,
  window,
  onSelect,
  t,
}: {
  buckets: HttpCaptureTimelineBucket[];
  available: boolean;
  window: TransactionFilter["window"];
  onSelect: (window: TransactionFilter["window"]) => void;
  t: Translate;
}): React.JSX.Element {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const dragStart = useRef<number | null>(null);
  const [drag, setDrag] = useState<{ lo: number; hi: number } | null>(null);
  // Keyboard-accessible start/end selection mirrored to the pointer brush.
  const lastBucket = Math.max(0, buckets.length - 1);
  const [range, setRange] = useState<{ lo: number; hi: number }>({ lo: 0, hi: lastBucket });

  useEffect(() => {
    // Reset the sliders to the full extent whenever the dataset changes or the
    // selection is cleared elsewhere.
    if (!window) setRange({ lo: 0, hi: Math.max(0, buckets.length - 1) });
  }, [window, buckets.length]);

  const maxCount = useMemo(
    () => buckets.reduce((max, bucket) => Math.max(max, bucket.count), 0),
    [buckets],
  );

  const indexAt = useCallback(
    (clientX: number): number => {
      const el = containerRef.current;
      if (!el || buckets.length === 0) return 0;
      const rect = el.getBoundingClientRect();
      const ratio = (clientX - rect.left) / rect.width;
      return Math.max(0, Math.min(buckets.length - 1, Math.floor(ratio * buckets.length)));
    },
    [buckets.length],
  );

  const onPointerDown = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (buckets.length === 0) return;
      event.currentTarget.setPointerCapture(event.pointerId);
      const idx = indexAt(event.clientX);
      dragStart.current = idx;
      setDrag({ lo: idx, hi: idx });
    },
    [buckets.length, indexAt],
  );

  const onPointerMove = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (dragStart.current === null) return;
      const idx = indexAt(event.clientX);
      const start = dragStart.current;
      setDrag({ lo: Math.min(start, idx), hi: Math.max(start, idx) });
    },
    [indexAt],
  );

  const onPointerUp = useCallback(
    (event: React.PointerEvent<HTMLDivElement>) => {
      if (dragStart.current === null) return;
      const idx = indexAt(event.clientX);
      const start = dragStart.current;
      dragStart.current = null;
      const lo = Math.min(start, idx);
      const hi = Math.max(start, idx);
      setDrag(null);
      setRange({ lo, hi });
      onSelect(timelineWindow(buckets, lo, hi));
    },
    [buckets, indexAt, onSelect],
  );

  const commitRange = useCallback(
    (lo: number, hi: number) => {
      const nlo = Math.min(lo, hi);
      const nhi = Math.max(lo, hi);
      setRange({ lo: nlo, hi: nhi });
      onSelect(timelineWindow(buckets, nlo, nhi));
    },
    [buckets, onSelect],
  );

  if (!available || buckets.length === 0) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t("httpCaptureTimelineTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <p className="text-sm text-muted-foreground" role="status">
            {t("httpCaptureTimelineUnavailable")}
          </p>
        </CardContent>
      </Card>
    );
  }

  const selection = drag
    ? { loPct: (drag.lo / buckets.length) * 100, hiPct: ((drag.hi + 1) / buckets.length) * 100 }
    : null;

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-2">
        <CardTitle>{t("httpCaptureTimelineTitle")}</CardTitle>
        {window && (
          <Button type="button" size="sm" variant="outline" onClick={() => onSelect(null)}>
            {t("httpCaptureClearSelection")}
          </Button>
        )}
      </CardHeader>
      <CardContent>
        <p className="mb-2 text-xs text-muted-foreground">{t("httpCaptureTimelineHint")}</p>
        <div
          ref={containerRef}
          className="relative flex h-32 cursor-col-resize touch-none items-end gap-px rounded-md border border-border bg-muted/20 p-1"
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={onPointerUp}
          role="group"
          aria-label={t("httpCaptureTimelineTitle")}
        >
          {selection && (
            <div
              className="pointer-events-none absolute inset-y-1 rounded-sm bg-primary/15 ring-1 ring-primary/40"
              style={{ left: `${selection.loPct}%`, width: `${selection.hiPct - selection.loPct}%` }}
              aria-hidden="true"
            />
          )}
          {buckets.map((bucket, index) => {
            const totalPct = maxCount > 0 ? (bucket.count / maxCount) * 100 : 0;
            const errorPct = bucket.count > 0 ? (bucket.errors / bucket.count) * totalPct : 0;
            return (
              <div
                key={`${bucket.start}-${index}`}
                className="relative flex min-w-0 flex-1 items-end"
                style={{ height: "100%" }}
                title={`${bucket.start} · ${bucket.count} ${t("httpCaptureTimelineRequests")} · ${bucket.errors} ${t("httpCaptureTimelineErrors")}`}
              >
                <div
                  className="w-full rounded-sm bg-primary/70"
                  style={{ height: `${Math.max(totalPct, bucket.count > 0 ? 4 : 0)}%` }}
                >
                  {errorPct > 0 && (
                    <div className="w-full rounded-sm bg-red-500" style={{ height: `${(errorPct / Math.max(totalPct, 1)) * 100}%` }} />
                  )}
                </div>
              </div>
            );
          })}
        </div>
        <div className="mt-1 flex justify-between font-mono text-[10px] text-muted-foreground">
          <span>{buckets[0]?.start}</span>
          <span>{buckets[buckets.length - 1]?.end}</span>
        </div>
        {/* Keyboard-accessible equivalent of the pointer brush. */}
        <div className="mt-2 flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-3">
          <label className="flex flex-1 items-center gap-2 text-[11px] text-muted-foreground">
            <span className="w-10 shrink-0">{t("httpCaptureTimelineStart")}</span>
            <input
              type="range"
              min={0}
              max={lastBucket}
              value={Math.min(range.lo, lastBucket)}
              onChange={(event) => commitRange(Number(event.target.value), range.hi)}
              aria-label={t("httpCaptureTimelineStart")}
              className="w-full"
            />
          </label>
          <label className="flex flex-1 items-center gap-2 text-[11px] text-muted-foreground">
            <span className="w-10 shrink-0">{t("httpCaptureTimelineEnd")}</span>
            <input
              type="range"
              min={0}
              max={lastBucket}
              value={Math.min(range.hi, lastBucket)}
              onChange={(event) => commitRange(range.lo, Number(event.target.value))}
              aria-label={t("httpCaptureTimelineEnd")}
              className="w-full"
            />
          </label>
        </div>
        <p className="mt-1 text-[10px] text-muted-foreground">{t("httpCaptureTimelineKeyboardHint")}</p>
        {window && (
          <p className="mt-2 text-xs text-foreground">
            <span className="font-medium">{t("httpCaptureTimelineSelection")}:</span>{" "}
            <span className="font-mono">
              {window.start} → {window.end}
            </span>
          </p>
        )}
      </CardContent>
    </Card>
  );
}

// ── Pseudo-process tree ─────────────────────────────────────────────

function ProcessTreePanel({
  tree,
  onOpen,
  t,
}: {
  tree: ProcessTreeNode[];
  onOpen: (id: string) => void;
  t: Translate;
}): React.JSX.Element {
  // Roots start expanded; deeper levels stay collapsed until opened.
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const rootsKey = tree.map((node) => node.id).join("|");
  const initializedFor = useRef<string>("");
  if (initializedFor.current !== rootsKey) {
    initializedFor.current = rootsKey;
    // default-expand the process roots so the tree is legible at a glance
    setExpanded(new Set(tree.map((node) => node.id)));
  }

  const allNodeIds = useMemo(() => collectExpandableIds(tree), [tree]);
  const toggle = useCallback((id: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-2">
        <CardTitle>{t("httpCaptureTreeTitle")}</CardTitle>
        {tree.length > 0 && (
          <div className="flex gap-1">
            <Button type="button" size="sm" variant="ghost" onClick={() => setExpanded(new Set(allNodeIds))}>
              {t("httpCaptureTreeExpandAll")}
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => setExpanded(new Set())}>
              {t("httpCaptureTreeCollapseAll")}
            </Button>
          </div>
        )}
      </CardHeader>
      <CardContent>
        <p className="mb-2 text-xs text-muted-foreground">{t("httpCaptureTreeHint")}</p>
        {tree.length === 0 ? (
          <p className="text-sm text-muted-foreground" role="status">
            {t("httpCaptureTreeEmpty")}
          </p>
        ) : (
          <ul className="flex flex-col gap-0.5">
            {tree.map((node) => (
              <TreeRow
                key={node.id}
                node={node}
                depth={0}
                expanded={expanded}
                onToggle={toggle}
                onOpen={onOpen}
                t={t}
              />
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

function collectExpandableIds(nodes: ProcessTreeNode[]): string[] {
  const ids: string[] = [];
  const walk = (list: ProcessTreeNode[]) => {
    for (const node of list) {
      if (node.children.length > 0) {
        ids.push(node.id);
        walk(node.children);
      }
    }
  };
  walk(nodes);
  return ids;
}

function TreeRow({
  node,
  depth,
  expanded,
  onToggle,
  onOpen,
  t,
}: {
  node: ProcessTreeNode;
  depth: number;
  expanded: Set<string>;
  onToggle: (id: string) => void;
  onOpen: (id: string) => void;
  t: Translate;
}): React.JSX.Element {
  const isLeaf = node.kind === "transaction";
  const isOpen = expanded.has(node.id);
  const hasChildren = node.children.length > 0;

  const handleActivate = () => {
    if (isLeaf && node.transactionId) {
      onOpen(node.transactionId);
    } else if (hasChildren) {
      onToggle(node.id);
    }
  };

  return (
    <li>
      <button
        type="button"
        onClick={handleActivate}
        className="flex w-full items-center gap-2 rounded px-1.5 py-1 text-left text-xs hover:bg-accent"
        style={{ paddingLeft: `${depth * 14 + 6}px` }}
      >
        <span className="w-3 shrink-0 text-muted-foreground">
          {hasChildren ? (isOpen ? "▾" : "▸") : isLeaf ? "•" : ""}
        </span>
        <span className="min-w-0 flex-1 truncate font-medium" title={node.label}>
          {node.label}
        </span>
        {node.pseudo && node.kind === "process" && (
          <span className="rounded bg-muted px-1 text-[10px] text-muted-foreground">
            {t("httpCaptureTreePseudo")}
          </span>
        )}
        <span className="shrink-0 truncate font-mono text-[10px] text-muted-foreground">{node.sublabel}</span>
        {!isLeaf && (
          <span className="shrink-0 tabular-nums text-[10px] text-muted-foreground">
            {node.count}
            {node.errorCount > 0 && (
              <span className="ml-1 text-red-600 dark:text-red-400">({node.errorCount})</span>
            )}
          </span>
        )}
      </button>
      {hasChildren && isOpen && (
        <ul className="flex flex-col gap-0.5">
          {node.children.map((child) => (
            <TreeRow
              key={child.id}
              node={child}
              depth={depth + 1}
              expanded={expanded}
              onToggle={onToggle}
              onOpen={onOpen}
              t={t}
            />
          ))}
        </ul>
      )}
    </li>
  );
}

// ── Transaction list + filter ───────────────────────────────────────

function TransactionListPanel({
  transactions,
  total,
  methods,
  mimes,
  fidelities,
  filter,
  filterActive,
  onFilter,
  onReset,
  onOpen,
  t,
}: {
  transactions: HttpCaptureTransactionRow[];
  total: number;
  methods: string[];
  mimes: string[];
  fidelities: string[];
  filter: TransactionFilter;
  filterActive: boolean;
  onFilter: (patch: Partial<TransactionFilter>) => void;
  onReset: () => void;
  onOpen: (id: string) => void;
  t: Translate;
}): React.JSX.Element {
  const selectClass =
    "h-9 rounded-md border border-input bg-transparent px-2 text-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring";
  const numberClass =
    "h-9 w-20 rounded-md border border-input bg-transparent px-2 text-sm focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring";
  return (
    <Card>
      <CardHeader className="flex-row items-center justify-between gap-2">
        <CardTitle>{t("httpCaptureListTitle")}</CardTitle>
        <span className="tabular-nums text-xs text-muted-foreground">
          {formatNumber(transactions.length)} / {formatNumber(total)}
        </span>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <Input
            value={filter.query}
            onChange={(event) => onFilter({ query: event.target.value })}
            placeholder={t("httpCaptureFilterQuery")}
            className="min-w-40 flex-1"
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
            onChange={(event) => onFilter({ statusClass: event.target.value as TransactionFilter["statusClass"] })}
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
                {fidelity}
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
                onFilter({ minDurationMs: event.target.value === "" ? null : Number(event.target.value) })
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
                onFilter({ maxDurationMs: event.target.value === "" ? null : Number(event.target.value) })
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
        </div>

        <div className="overflow-x-auto">
          {transactions.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground" role="status">
              {t("httpCaptureListEmpty")}
            </p>
          ) : (
            <table className="w-full text-left text-xs">
              <caption className="sr-only">{t("httpCaptureListCaption")}</caption>
              <thead>
                <tr className="border-b border-border text-muted-foreground">
                  <th scope="col" className="p-2 font-medium">
                    {t("httpCaptureColMethod")}
                  </th>
                  <th scope="col" className="p-2 font-medium">
                    {t("httpCaptureColUrl")}
                  </th>
                  <th scope="col" className="p-2 text-right font-medium">
                    {t("httpCaptureColStatus")}
                  </th>
                  <th scope="col" className="p-2 text-right font-medium">
                    {t("httpCaptureColDuration")}
                  </th>
                  <th scope="col" className="p-2 text-right font-medium">
                    {t("httpCaptureColSize")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {transactions.slice(0, 200).map((tx) => (
                  <tr
                    key={tx.id}
                    className="cursor-pointer border-t hover:bg-accent focus-visible:bg-accent focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-inset focus-visible:ring-ring"
                    role="button"
                    tabIndex={0}
                    aria-label={`${t("httpCaptureRowOpen")}: ${tx.method} ${tx.host}${tx.path} ${tx.status || tx.state}`}
                    onClick={() => onOpen(tx.id)}
                    onKeyDown={(event) => {
                      if (event.key === "Enter" || event.key === " ") {
                        event.preventDefault();
                        onOpen(tx.id);
                      }
                    }}
                  >
                    <td className="p-2 font-mono">{tx.method}</td>
                    <td className="max-w-sm truncate p-2" title={tx.url}>
                      <span className="text-muted-foreground">{tx.host}</span>
                      {tx.path}
                    </td>
                    <td className={`p-2 text-right font-mono tabular-nums ${statusColor(tx.status)}`}>
                      {tx.status || tx.state}
                    </td>
                    <td className="p-2 text-right tabular-nums">{Math.round(tx.duration_ms)}</td>
                    <td className="p-2 text-right tabular-nums text-muted-foreground">
                      {formatBytes(tx.response_bytes)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

// ── Transaction detail slide-over (tabbed) ──────────────────────────

type DetailTab = "request" | "response" | "timing" | "process";

function TransactionDetail({
  transaction,
  onClose,
  t,
}: {
  transaction: HttpCaptureTransactionRow | null;
  onClose: () => void;
  t: Translate;
}): React.JSX.Element {
  const [tab, setTab] = useState<DetailTab>("request");
  // Reset to the request tab each time a different transaction opens.
  const openId = transaction?.id ?? "";
  const tabForId = useRef<string>("");
  if (tabForId.current !== openId) {
    tabForId.current = openId;
    if (tab !== "request") setTab("request");
  }

  const phases = useMemo(() => timingBreakdown(transaction), [transaction]);
  const maxPhase = phases.reduce((max, phase) => Math.max(max, phase.ms), 0);

  const tabs: Array<{ key: DetailTab; label: string }> = [
    { key: "request", label: t("httpCaptureDetailRequest") },
    { key: "response", label: t("httpCaptureDetailResponse") },
    { key: "timing", label: t("httpCaptureDetailTimings") },
    { key: "process", label: t("httpCaptureDetailProcess") },
  ];

  return (
    <SlideOverPanel
      open={transaction !== null}
      onClose={onClose}
      title={
        transaction
          ? `${transaction.method ?? ""} ${transaction.path ?? ""}`.trim() || t("httpCaptureDetailTitle")
          : t("httpCaptureDetailTitle")
      }
      width={560}
    >
      {transaction && (
        <div className="flex flex-col gap-4 text-sm">
          <div className="break-all rounded-md border border-border bg-muted/20 p-2 font-mono text-xs">
            <span className={statusColor(transaction.status)}>{transaction.status || transaction.state}</span>{" "}
            {transaction.url}
          </div>

          <dl className="grid grid-cols-2 gap-2 text-xs">
            <Field label={t("httpCaptureColDuration")} value={`${Math.round(transaction.duration_ms)} ms`} />
            <Field label={t("httpCaptureDialectLabel")} value={transaction.http_version} />
            <Field
              label={t("httpCaptureDetailConnection")}
              value={`${transaction.connection_id}${transaction.used_existing_connection ? ` · ${t("httpCaptureDetailReused")}` : ""}`}
            />
            <Field label={t("httpCaptureFidelityLabel")} value={transaction.fidelity} />
          </dl>

          <div role="tablist" aria-label={t("httpCaptureDetailTitle")} className="flex gap-1 border-b border-border">
            {tabs.map((entry) => (
              <button
                key={entry.key}
                type="button"
                role="tab"
                id={`http-detail-tab-${entry.key}`}
                aria-selected={tab === entry.key}
                aria-controls={`http-detail-panel-${entry.key}`}
                onClick={() => setTab(entry.key)}
                className={`-mb-px border-b-2 px-3 py-1.5 text-xs font-medium ${
                  tab === entry.key
                    ? "border-primary text-foreground"
                    : "border-transparent text-muted-foreground hover:text-foreground"
                }`}
              >
                {entry.label}
              </button>
            ))}
          </div>

          <div
            role="tabpanel"
            id={`http-detail-panel-${tab}`}
            aria-labelledby={`http-detail-tab-${tab}`}
          >
            {tab === "request" && (
              <MessagePanel message={transaction.request} t={t} />
            )}
            {tab === "response" && (
              <MessagePanel message={transaction.response} t={t} />
            )}
            {tab === "timing" && (
              <section>
                {phases.length === 0 || maxPhase === 0 ? (
                  <p className="text-xs text-muted-foreground">{t("httpCaptureDetailNoTimings")}</p>
                ) : (
                  <ul className="flex flex-col gap-1">
                    {phases.map((phase) => (
                      <li key={phase.phase} className="flex items-center gap-2">
                        <span className="w-16 shrink-0 font-mono text-[11px] text-muted-foreground">{phase.phase}</span>
                        <div className="h-3 flex-1 overflow-hidden rounded-sm bg-muted">
                          <div
                            className={`h-full ${phase.state === "known" ? "bg-primary/70" : "bg-muted-foreground/30"}`}
                            style={{ width: `${maxPhase > 0 ? (phase.ms / maxPhase) * 100 : 0}%` }}
                          />
                        </div>
                        <span className="w-14 shrink-0 text-right font-mono text-[11px] tabular-nums">
                          {phase.state === "known" ? `${phase.ms.toFixed(1)}ms` : phase.state}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </section>
            )}
            {tab === "process" && (
              <section>
                {transaction.process ? (
                  <p className="font-mono text-xs">
                    {transaction.process.name} · pid {transaction.process.pid} · {transaction.process.attribution}
                  </p>
                ) : (
                  <p className="text-xs text-muted-foreground">{t("httpCaptureDetailNoProcess")}</p>
                )}
              </section>
            )}
          </div>
        </div>
      )}
    </SlideOverPanel>
  );
}

// ── Request / response message panel (headers, cookies, body) ───────

function MessagePanel({
  message,
  t,
}: {
  message: CaptureHTTPMessage | undefined;
  t: Translate;
}): React.JSX.Element {
  return (
    <div className="flex flex-col gap-3">
      {message?.redacted && (
        <p
          className="rounded-md border border-amber-500/50 bg-amber-500/10 p-2 text-[11px] text-amber-700 dark:text-amber-400"
          role="status"
        >
          {t("httpCaptureDetailMessageRedacted")}
        </p>
      )}
      <dl className="grid grid-cols-2 gap-2 text-xs">
        <Field label={t("httpCaptureDetailContentType")} value={message?.contentType ?? ""} />
        <Field label={t("httpCaptureDetailBodyStorage")} value={message?.bodyStorage ?? ""} />
        <Field label={t("httpCaptureDetailBodySize")} value={formatBytes(message?.bodySize)} />
      </dl>

      <KeyValueTable
        title={t("httpCaptureDetailHeaders")}
        fields={message?.headers ?? []}
        emptyLabel={t("httpCaptureDetailNoHeaders")}
        redactedLabel={t("httpCaptureDetailRedactedBadge")}
      />
      <KeyValueTable
        title={t("httpCaptureDetailCookies")}
        fields={message?.cookies ?? []}
        emptyLabel={t("httpCaptureDetailNoCookies")}
        redactedLabel={t("httpCaptureDetailRedactedBadge")}
      />

      {message?.bodyPreview && (
        <div>
          <span className="text-[10px] uppercase tracking-wider text-muted-foreground">
            {t("httpCaptureDetailBodyPreview")}
          </span>
          <pre className="mt-0.5 max-h-40 overflow-auto rounded border border-border bg-muted/30 p-2 text-[11px]">
            {message.bodyPreview}
          </pre>
        </div>
      )}
    </div>
  );
}

function KeyValueTable({
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
    <section>
      <h4 className="mb-1 text-xs font-semibold uppercase tracking-wider text-muted-foreground">{title}</h4>
      {fields.length === 0 ? (
        <p className="text-xs text-muted-foreground">{emptyLabel}</p>
      ) : (
        <table className="w-full text-left text-[11px]">
          <tbody>
            {fields.map((field, index) => (
              <tr key={`${field.name}-${index}`} className="border-t border-border/50 align-top">
                <td className="w-1/3 py-1 pr-2 font-mono font-medium">{field.name}</td>
                <td className="break-all py-1 font-mono text-muted-foreground">
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
