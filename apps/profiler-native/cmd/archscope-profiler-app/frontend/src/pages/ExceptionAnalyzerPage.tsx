import { Loader2, Play, Search, Square, X } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import { engine } from "@/bridge/engine";
import type {
  BridgeError,
  ExceptionEventRow,
  ExceptionFinding,
  ExceptionSignatureRow,
  ExceptionStackAnalysisResult,
  ExceptionTypeDistributionRow,
} from "@/bridge/types";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { MetricCard } from "@/components/MetricCard";
import { RecentFilesPanel } from "@/components/RecentFilesPanel";
import { SlideOverPanel } from "@/components/SlideOverPanel";
import {
  WailsFileDock,
  type FileDockSelection,
} from "@/components/WailsFileDock";
import { useRecentFiles } from "@/hooks/useRecentFiles";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import { formatNumber } from "@/utils/formatters";

// Wails port of apps/frontend/src/pages/ExceptionAnalyzerPage.tsx
// (T-351'-Phase-2). Mirrors the AccessLog port in shape:
//   - file picker via WailsFileDock (single file path → engine reads)
//   - synchronous analyze through engine.analyzeException
//   - tabs: Events / Top types / Top signatures / Findings / Diagnostics
//
// Behavioural deltas vs. the web original:
//   - Web uses an `analyzerClient.execute({ type, params })` envelope
//     with `{ ok, error, engine_messages }`. Wails uses a typed
//     `Promise<ExceptionStackAnalysisResult>` directly, so the error
//     plumbing collapses to a try/catch.
//   - The web page renders detail in a Radix `Sheet` overlay. The Wails
//     shell hasn't ported `Sheet` yet (no extra Radix dep beyond what
//     Phase 1 needed), so detail rows expand inline as a card under the
//     table. The information density is identical.
//   - The web `D3BarChart` for "exceptions by type" is replaced with a
//     compact bar table (count + relative bar fill). Phase 2 keeps the
//     surface layer-thin until a charting helper is shared.

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

const DEFAULT_TOP_N = 50;
const PAGE_SIZE_OPTIONS = [10, 25, 50, 100] as const;

export function ExceptionAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(
    null,
  );
  const [topN, setTopN] = useState<number>(DEFAULT_TOP_N);
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<ExceptionStackAnalysisResult | null>(
    null,
  );
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);

  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<number>(10);
  const [activeEventKey, setActiveEventKey] = useState<string | null>(null);
  // Exception-scoped recent-files history.
  const recent = useRecentFiles({ category: "exception" });
  const [activeSignature, setActiveSignature] =
    useState<ExceptionSignatureRow | null>(null);

  const inflightRef = useRef<symbol | null>(null);

  const filePath = selectedFile?.filePath ?? "";
  const canAnalyze = Boolean(filePath) && state !== "running";
  const summary = result?.summary;
  const series = result?.series;
  const tableEvents = (result?.tables.exceptions ?? []) as ExceptionEventRow[];
  const findings = (result?.metadata.findings ?? []) as ExceptionFinding[];

  const rootCausePresent = useMemo(
    () =>
      findings.find((f) => f.code === "ROOT_CAUSE_PRESENT")?.evidence
        ?.root_cause_count ?? 0,
    [findings],
  );

  const handleFileSelected = useCallback((file: FileDockSelection) => {
    setSelectedFile(file);
    setState("ready");
    setError(null);
    setEngineMessages([]);
    setPage(1);
    setFilter("");
    setActiveEventKey(null);
  }, []);

  const handleClearFile = useCallback(() => {
    setSelectedFile(null);
    if (state !== "running") setState("idle");
  }, [state]);

  const analyze = useCallback(async () => {
    if (!canAnalyze) return;
    if (!Number.isInteger(topN) || topN <= 0) {
      setError({
        code: "INVALID_OPTION",
        message: t("invalidAnalyzerOptions"),
      });
      setState("error");
      return;
    }

    setState("running");
    setError(null);
    setEngineMessages([]);
    setPage(1);
    setActiveEventKey(null);
    const token = Symbol("exception-analysis");
    inflightRef.current = token;

    try {
      const response = await engine.analyzeException({
        path: filePath,
        topN,
      });
      if (inflightRef.current !== token) return;
      inflightRef.current = null;
      setResult(response);
      setState("success");
      if (filePath) {
        recent.push({
          path: filePath,
          analyzer: "exception",
          meta: { topN },
        });
      }
    } catch (caught) {
      if (inflightRef.current !== token) return;
      inflightRef.current = null;
      const message = caught instanceof Error ? caught.message : String(caught);
      setError({ code: "ENGINE_FAILED", message });
      setState("error");
    }
  }, [canAnalyze, filePath, recent, t, topN]);

  const handleRecentSelect = useCallback(
    (entry: { path: string; name?: string; meta?: Record<string, unknown> }) => {
      setSelectedFile({
        filePath: entry.path,
        originalName: entry.name ?? entry.path,
      } as FileDockSelection);
      const meta = entry.meta ?? {};
      if (typeof meta.topN === "number") setTopN(meta.topN);
      setState("ready");
      setError(null);
      setEngineMessages([]);
    },
    [],
  );

  const cancelAnalysis = useCallback(() => {
    // The synchronous Exception analyzer doesn't expose a cancel hook;
    // discarding the token detaches the renderer from the in-flight call.
    inflightRef.current = null;
    setState("ready");
    setEngineMessages([t("analysisCanceled")]);
  }, [t]);

  // ── derived event listing (filter / paginate) ──
  const filteredEvents = useMemo<ExceptionEventRow[]>(() => {
    const term = filter.trim().toLowerCase();
    if (!term) return tableEvents;
    return tableEvents.filter((evt) => {
      const haystack = [
        evt.exception_type ?? "",
        evt.message ?? "",
        evt.top_frame ?? "",
        evt.root_cause ?? "",
      ]
        .join(" \n ")
        .toLowerCase();
      return haystack.includes(term);
    });
  }, [tableEvents, filter]);

  const totalRows = filteredEvents.length;
  const totalPages = Math.max(1, Math.ceil(totalRows / pageSize));
  const safePage = Math.min(page, totalPages);
  const pageStart = (safePage - 1) * pageSize;
  const pagedEvents = filteredEvents.slice(pageStart, pageStart + pageSize);
  const showingFrom = totalRows === 0 ? 0 : pageStart + 1;
  const showingTo = Math.min(pageStart + pageSize, totalRows);

  const typeRows = useMemo<ExceptionTypeDistributionRow[]>(
    () => series?.exception_type_distribution ?? [],
    [series],
  );
  const signatureRows = useMemo<ExceptionSignatureRow[]>(
    () => series?.top_stack_signatures ?? [],
    [series],
  );

  const activeEvent = useMemo<ExceptionEventRow | null>(() => {
    if (!activeEventKey) return null;
    return (
      tableEvents.find(
        (evt, idx) => keyForEvent(evt, idx) === activeEventKey,
      ) ?? null
    );
  }, [activeEventKey, tableEvents]);

  return (
    <main className="flex flex-col gap-5 p-5">
      <WailsFileDock
        label={t("selectExceptionFile")}
        description={t("dropOrBrowseException")}
        accept=".log,.txt"
        selected={selectedFile}
        onSelect={handleFileSelected}
        onClear={handleClearFile}
        browseLabel={t("browseFile")}
        dropHereLabel={t("dropHere")}
        errorLabel={t("error")}
        fileFilters={[
          { displayName: "Exception logs", pattern: "*.log;*.txt" },
          { displayName: "All files", pattern: "*.*" },
        ]}
        rightSlot={
          <div className="flex items-center gap-2">
            <Button
              type="button"
              size="sm"
              disabled={!canAnalyze}
              onClick={() => void analyze()}
            >
              {state === "running" ? (
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
            {state === "running" && (
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => cancelAnalysis()}
              >
                <Square className="h-3.5 w-3.5" />
                {t("cancelAnalysis")}
              </Button>
            )}
          </div>
        }
      />

      <RecentFilesPanel
        entries={recent.entries}
        onSelect={handleRecentSelect}
        onRemove={recent.remove}
        onClear={recent.clear}
      />

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">{t("analyzerOptions")}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-3 md:grid-cols-3">
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("topN")} <span className="text-muted-foreground">({DEFAULT_TOP_N})</span>
            </span>
            <Input
              type="number"
              min={1}
              placeholder={`${DEFAULT_TOP_N}`}
              value={topN}
              onChange={(event) =>
                setTopN(Number(event.target.value) || DEFAULT_TOP_N)
              }
            />
          </label>
        </CardContent>
      </Card>

      <ErrorPanel
        error={error}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />
      <EngineMessagesPanel
        messages={engineMessages}
        title={t("engineMessages")}
      />

      {summary && (
        <section className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
          <MetricCard
            label={t("exceptionTotalCount")}
            value={formatNumber(summary.total_exceptions)}
          />
          <MetricCard
            label={t("exceptionUniqueTypes")}
            value={formatNumber(summary.unique_exception_types)}
          />
          <MetricCard
            label={t("exceptionUniqueSignatures")}
            value={formatNumber(summary.unique_signatures)}
          />
          <MetricCard
            label={t("exceptionTopTypeLabel")}
            value={summary.top_exception_type ?? "—"}
          />
          <MetricCard
            label={t("exceptionRootCausePresent")}
            value={formatNumber(Number(rootCausePresent) || 0)}
          />
        </section>
      )}

      <Tabs defaultValue="events" className="w-full">
        <TabsList>
          <TabsTrigger value="events">{t("exceptionTabEvents")}</TabsTrigger>
          <TabsTrigger value="types">{t("exceptionTabTopTypes")}</TabsTrigger>
          <TabsTrigger value="signatures">
            {t("exceptionTabSignatures")}
          </TabsTrigger>
          <TabsTrigger value="findings">
            {t("exceptionTabFindings")}
          </TabsTrigger>
          <TabsTrigger value="diagnostics">{t("tabDiagnostics")}</TabsTrigger>
        </TabsList>

        <TabsContent value="events" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <CardTitle className="text-sm">
                  {t("exceptionTabEvents")}
                </CardTitle>
                <div className="relative w-full max-w-xs">
                  <Search className="pointer-events-none absolute left-2 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
                  <Input
                    type="search"
                    placeholder={t("exceptionFilterPlaceholder")}
                    className="h-8 pl-7 text-xs"
                    value={filter}
                    onChange={(event) => {
                      setFilter(event.target.value);
                      setPage(1);
                    }}
                  />
                </div>
              </div>
            </CardHeader>
            <CardContent className="p-0">
              {tableEvents.length === 0 ? (
                <p className="px-6 py-6 text-center text-sm text-muted-foreground">
                  {t("exceptionNoEvents")}
                </p>
              ) : pagedEvents.length === 0 ? (
                <p className="px-6 py-6 text-center text-sm text-muted-foreground">
                  {t("exceptionEmpty")}
                </p>
              ) : (
                <EventsTable
                  events={pagedEvents}
                  pageStart={pageStart}
                  activeKey={activeEventKey}
                  onRowClick={(evt, idx) =>
                    setActiveEventKey((prev) => {
                      const key = keyForEvent(evt, idx);
                      return prev === key ? null : key;
                    })
                  }
                  labels={{
                    time: t("exceptionEventTime"),
                    type: t("exceptionEventType"),
                    topFrame: t("exceptionEventTopFrame"),
                    language: t("exceptionEventLanguage"),
                    showDetails: t("exceptionShowDetails"),
                    frameCount: (n) =>
                      t("exceptionFrameCount").replace("{n}", String(n)),
                  }}
                />
              )}
            </CardContent>
            {totalRows > 0 && (
              <PaginationBar
                page={safePage}
                totalPages={totalPages}
                pageSize={pageSize}
                onPageChange={setPage}
                onPageSizeChange={(size) => {
                  setPageSize(size);
                  setPage(1);
                }}
                showingLabel={t("paginationShowing")
                  .replace("{from}", String(showingFrom))
                  .replace("{to}", String(showingTo))
                  .replace("{total}", String(totalRows))}
                pageOfLabel={t("paginationPageOf")
                  .replace("{page}", String(safePage))
                  .replace("{total}", String(totalPages))}
                prevLabel={t("paginationPrev")}
                nextLabel={t("paginationNext")}
                pageSizeLabel={t("paginationPageSize")}
              />
            )}
          </Card>

          <SlideOverPanel
            open={Boolean(activeEvent)}
            onClose={() => setActiveEventKey(null)}
            title={t("exceptionEventDetails")}
            width={560}
          >
            {activeEvent && (
              <EventDetailCard
                event={activeEvent}
                onClose={() => setActiveEventKey(null)}
                hideHeader
                labels={{
                  title: t("exceptionEventDetails"),
                  close: t("exceptionCloseDetails"),
                  time: t("exceptionEventTime"),
                  language: t("exceptionEventLanguage"),
                  message: t("exceptionEventMessage"),
                  rootCause: t("exceptionEventRootCause"),
                  signature: t("exceptionEventSignature"),
                  stack: t("exceptionEventStack"),
                }}
              />
            )}
          </SlideOverPanel>
        </TabsContent>

        <TabsContent value="types" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("exceptionsByType")}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              {typeRows.length === 0 ? (
                <p className="px-6 py-6 text-sm text-muted-foreground">—</p>
              ) : (
                <BarTable
                  rows={typeRows.map((row) => ({
                    label: row.exception_type,
                    short: simpleClassName(row.exception_type),
                    count: row.count,
                  }))}
                  labels={{
                    name: t("exceptionEventType"),
                    count: t("count"),
                  }}
                />
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="signatures" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">
                {t("exceptionsBySignature")}
              </CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              {signatureRows.length === 0 ? (
                <p className="px-6 py-6 text-sm text-muted-foreground">—</p>
              ) : (
                <SignatureTable
                  rows={signatureRows}
                  activeSignature={activeSignature?.signature ?? null}
                  onRowClick={(row) =>
                    setActiveSignature((prev) =>
                      prev?.signature === row.signature ? null : row,
                    )
                  }
                  labels={{
                    signature: t("exceptionEventSignature"),
                    count: t("count"),
                    showDetails: t("exceptionShowDetails"),
                  }}
                />
              )}
            </CardContent>
          </Card>

          {activeSignature && (
            <Card>
              <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
                <CardTitle className="text-sm">
                  {t("exceptionEventSignature")}
                </CardTitle>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs"
                  onClick={() => setActiveSignature(null)}
                >
                  <X className="h-3.5 w-3.5" />
                  {t("exceptionCloseDetails")}
                </Button>
              </CardHeader>
              <CardContent>
                <p className="mb-2 text-xs text-muted-foreground">
                  {t("count")}:{" "}
                  <span className="tabular-nums">
                    {activeSignature.count.toLocaleString()}
                  </span>
                </p>
                <pre className="overflow-x-auto rounded-md border border-border bg-muted/40 p-3 font-mono text-[11px] leading-relaxed">
                  {activeSignature.signature}
                </pre>
              </CardContent>
            </Card>
          )}
        </TabsContent>

        <TabsContent value="findings" className="mt-4">
          <FindingsPanel
            findings={findings}
            labels={{
              empty: t("exceptionFindingsEmpty"),
              severity: t("exceptionFindingSeverity"),
              code: t("exceptionFindingCode"),
              message: t("exceptionFindingMessage"),
              title: t("exceptionTabFindings"),
            }}
          />
        </TabsContent>

        <TabsContent value="diagnostics" className="mt-4">
          <DiagnosticsPanel
            diagnostics={result?.metadata.diagnostics}
            labels={{
              title: t("parserDiagnostics"),
              parsedRecords: t("parsedRecords"),
              skippedLines: t("skippedLines"),
              samples: t("diagnosticSamples"),
            }}
          />
        </TabsContent>
      </Tabs>
    </main>
  );
}

// ──────────────────────────────────────────────────────────────────
// Sub-components — kept inline so the page is self-contained, like
// the AccessLog port.
// ──────────────────────────────────────────────────────────────────

type EventsTableProps = {
  events: ExceptionEventRow[];
  pageStart: number;
  activeKey: string | null;
  onRowClick: (event: ExceptionEventRow, indexInPage: number) => void;
  labels: {
    time: string;
    type: string;
    topFrame: string;
    language: string;
    showDetails: string;
    frameCount: (n: number) => string;
  };
};

function EventsTable({
  events,
  pageStart,
  activeKey,
  onRowClick,
  labels,
}: EventsTableProps): JSX.Element {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/40 text-[11px] uppercase tracking-wider text-muted-foreground">
            <th className="w-[150px] px-3 py-2 text-left font-medium">
              {labels.time}
            </th>
            <th className="w-[60px] px-3 py-2 text-left font-medium">
              {labels.language}
            </th>
            <th className="w-[220px] px-3 py-2 text-left font-medium">
              {labels.type}
            </th>
            <th className="px-3 py-2 text-left font-medium">
              {labels.topFrame}
            </th>
            <th className="w-[80px] px-3 py-2 text-right font-medium">
              {""}
            </th>
          </tr>
        </thead>
        <tbody>
          {events.map((evt, index) => {
            const frameCount = evt.stack?.length ?? 0;
            const key = keyForEvent(evt, pageStart + index);
            const isActive = key === activeKey;
            return (
              <tr
                key={key}
                className={`cursor-pointer border-b border-border last:border-0 hover:bg-muted/40 ${
                  isActive ? "bg-muted/60" : ""
                }`}
                onClick={() => onRowClick(evt, pageStart + index)}
                title={labels.showDetails}
              >
                <td className="px-3 py-2 font-mono text-[11px] text-muted-foreground">
                  {formatTime(evt.timestamp) || "—"}
                </td>
                <td className="px-3 py-2 text-[11px] uppercase text-muted-foreground">
                  {evt.language ?? "—"}
                </td>
                <td className="px-3 py-2">
                  <span className="inline-block max-w-full truncate align-middle font-medium">
                    {evt.exception_type ?? "—"}
                  </span>
                  {frameCount > 0 && (
                    <span className="ml-2 align-middle text-[10px] text-muted-foreground">
                      {labels.frameCount(frameCount)}
                    </span>
                  )}
                </td>
                <td className="px-3 py-2">
                  <span
                    className="block truncate font-mono text-xs"
                    title={evt.top_frame ?? evt.message ?? ""}
                  >
                    {evt.top_frame ?? evt.message ?? "—"}
                  </span>
                </td>
                <td className="px-3 py-2 text-right">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2 text-xs"
                    onClick={(e) => {
                      e.stopPropagation();
                      onRowClick(evt, pageStart + index);
                    }}
                  >
                    {labels.showDetails}
                  </Button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

type SignatureTableProps = {
  rows: ExceptionSignatureRow[];
  activeSignature: string | null;
  onRowClick: (row: ExceptionSignatureRow) => void;
  labels: {
    signature: string;
    count: string;
    showDetails: string;
  };
};

function SignatureTable({
  rows,
  activeSignature,
  onRowClick,
  labels,
}: SignatureTableProps): JSX.Element {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/40 text-[11px] uppercase tracking-wider text-muted-foreground">
            <th className="px-3 py-2 text-left font-medium">
              {labels.signature}
            </th>
            <th className="w-[80px] px-3 py-2 text-right font-medium">
              {labels.count}
            </th>
            <th className="w-[80px] px-3 py-2 text-right font-medium">
              {""}
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => {
            const isActive = row.signature === activeSignature;
            return (
              <tr
                key={`${row.signature}-${index}`}
                className={`cursor-pointer border-b border-border last:border-0 hover:bg-muted/40 ${
                  isActive ? "bg-muted/60" : ""
                }`}
                onClick={() => onRowClick(row)}
                title={labels.showDetails}
              >
                <td className="px-3 py-2">
                  <span
                    className="block truncate font-mono text-xs"
                    title={row.signature}
                  >
                    {row.signature}
                  </span>
                </td>
                <td className="px-3 py-2 text-right tabular-nums">
                  {row.count.toLocaleString()}
                </td>
                <td className="px-3 py-2 text-right">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-7 px-2 text-xs"
                    onClick={(e) => {
                      e.stopPropagation();
                      onRowClick(row);
                    }}
                  >
                    {labels.showDetails}
                  </Button>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

type BarTableRow = { label: string; short: string; count: number };

function BarTable({
  rows,
  labels,
}: {
  rows: BarTableRow[];
  labels: { name: string; count: string };
}): JSX.Element {
  const max = rows.reduce((acc, row) => Math.max(acc, row.count), 0) || 1;
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/40 text-[11px] uppercase tracking-wider text-muted-foreground">
            <th className="px-3 py-2 text-left font-medium">{labels.name}</th>
            <th className="w-[40%] px-3 py-2 text-left font-medium">{""}</th>
            <th className="w-[80px] px-3 py-2 text-right font-medium">
              {labels.count}
            </th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => {
            const ratio = row.count / max;
            return (
              <tr
                key={`${row.label}-${index}`}
                className="border-b border-border last:border-0"
              >
                <td className="px-3 py-1.5">
                  <span
                    className="block truncate font-medium"
                    title={row.label}
                  >
                    {row.short}
                  </span>
                </td>
                <td className="px-3 py-1.5">
                  <div className="h-2 w-full rounded-full bg-muted">
                    <div
                      className="h-2 rounded-full bg-primary"
                      style={{ width: `${Math.max(2, ratio * 100)}%` }}
                    />
                  </div>
                </td>
                <td className="px-3 py-1.5 text-right tabular-nums">
                  {row.count.toLocaleString()}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

type EventDetailCardProps = {
  event: ExceptionEventRow;
  onClose: () => void;
  /** Skip the inline header — used when the panel is hosted inside a
   *  SlideOverPanel that already provides its own title bar. */
  hideHeader?: boolean;
  labels: {
    title: string;
    close: string;
    time: string;
    language: string;
    message: string;
    rootCause: string;
    signature: string;
    stack: string;
  };
};

function EventDetailCard({
  event,
  onClose,
  hideHeader,
  labels,
}: EventDetailCardProps): JSX.Element {
  if (hideHeader) {
    return (
      <div className="flex flex-col gap-3 text-sm">
        <div className="text-xs text-muted-foreground">
          <div className="font-mono">{formatTime(event.timestamp) || "—"}</div>
          {event.exception_type && (
            <div className="mt-0.5 font-semibold text-foreground">
              {event.exception_type}
            </div>
          )}
          {event.language && (
            <div className="mt-0.5 uppercase">{event.language}</div>
          )}
        </div>
        <DetailField label={labels.message} value={event.message} mono />
        <DetailField label={labels.rootCause} value={event.root_cause} />
        <DetailField label={labels.signature} value={event.signature} mono />
        <section>
          <div className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
            {labels.stack}
          </div>
          {event.stack && event.stack.length > 0 ? (
            <pre className="overflow-auto rounded-md border border-border bg-muted/40 p-3 font-mono text-[11px] leading-relaxed">
              {event.stack.join("\n")}
            </pre>
          ) : (
            <p className="text-xs text-muted-foreground">—</p>
          )}
        </section>
      </div>
    );
  }
  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between space-y-0 pb-3">
        <div>
          <CardTitle className="text-sm">
            {event.exception_type ?? labels.title}
          </CardTitle>
          <p className="mt-1 font-mono text-xs text-muted-foreground">
            {formatTime(event.timestamp) || "—"}
            {event.language && (
              <span className="ml-2 uppercase">{event.language}</span>
            )}
          </p>
        </div>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="h-7 px-2 text-xs"
          onClick={onClose}
        >
          <X className="h-3.5 w-3.5" />
          {labels.close}
        </Button>
      </CardHeader>
      <CardContent className="space-y-3 text-sm">
        <DetailField label={labels.message} value={event.message} mono />
        <DetailField label={labels.rootCause} value={event.root_cause} />
        <DetailField label={labels.signature} value={event.signature} mono />
        <section>
          <div className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
            {labels.stack}
          </div>
          {event.stack && event.stack.length > 0 ? (
            <pre className="max-h-[60vh] overflow-auto rounded-md border border-border bg-muted/40 p-3 font-mono text-[11px] leading-relaxed">
              {event.stack.join("\n")}
            </pre>
          ) : (
            <p className="text-xs text-muted-foreground">—</p>
          )}
        </section>
      </CardContent>
    </Card>
  );
}

function DetailField({
  label,
  value,
  mono = false,
}: {
  label: string;
  value: string | null | undefined;
  mono?: boolean;
}): JSX.Element | null {
  if (!value) return null;
  return (
    <section>
      <div className="mb-1 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div
        className={`whitespace-pre-wrap break-words rounded-md border border-border bg-muted/40 p-2.5 ${
          mono ? "font-mono text-[11px]" : "text-sm"
        }`}
      >
        {value}
      </div>
    </section>
  );
}

type FindingsPanelProps = {
  findings: ExceptionFinding[];
  labels: {
    empty: string;
    severity: string;
    code: string;
    message: string;
    title: string;
  };
};

function FindingsPanel({
  findings,
  labels,
}: FindingsPanelProps): JSX.Element {
  if (!findings || findings.length === 0) {
    return (
      <Card>
        <CardContent className="py-6 text-center text-sm text-muted-foreground">
          {labels.empty}
        </CardContent>
      </Card>
    );
  }
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{labels.title}</CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-muted/40 text-[11px] uppercase tracking-wider text-muted-foreground">
              <th className="w-[100px] px-3 py-2 text-left font-medium">
                {labels.severity}
              </th>
              <th className="w-[260px] px-3 py-2 text-left font-medium">
                {labels.code}
              </th>
              <th className="px-3 py-2 text-left font-medium">
                {labels.message}
              </th>
            </tr>
          </thead>
          <tbody>
            {findings.map((finding, index) => (
              <tr
                key={`${finding.code}-${index}`}
                className="border-b border-border last:border-0"
              >
                <td className="px-3 py-2">
                  <span
                    className={severityBadgeClass(finding.severity)}
                  >
                    {finding.severity}
                  </span>
                </td>
                <td className="px-3 py-2 font-mono text-xs">
                  {finding.code}
                </td>
                <td className="px-3 py-2 text-xs">
                  <p>{finding.message}</p>
                  {finding.evidence &&
                    Object.keys(finding.evidence).length > 0 && (
                      <ul className="mt-1 space-y-0.5 text-[10px] text-muted-foreground">
                        {Object.entries(finding.evidence).map(
                          ([key, value]) => (
                            <li key={key}>
                              <span className="font-mono">{key}</span>:{" "}
                              <span className="font-mono">{String(value)}</span>
                            </li>
                          ),
                        )}
                      </ul>
                    )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

type PaginationBarProps = {
  page: number;
  totalPages: number;
  pageSize: number;
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
  showingLabel: string;
  pageOfLabel: string;
  prevLabel: string;
  nextLabel: string;
  pageSizeLabel: string;
};

function PaginationBar({
  page,
  totalPages,
  pageSize,
  onPageChange,
  onPageSizeChange,
  showingLabel,
  pageOfLabel,
  prevLabel,
  nextLabel,
  pageSizeLabel,
}: PaginationBarProps): JSX.Element {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-2.5 text-xs text-muted-foreground">
      <span className="tabular-nums">{showingLabel}</span>
      <div className="flex items-center gap-3">
        <label className="flex items-center gap-2">
          <span>{pageSizeLabel}</span>
          <select
            className="h-7 rounded-md border border-border bg-background px-2 text-xs"
            value={pageSize}
            onChange={(e) => onPageSizeChange(Number(e.target.value))}
          >
            {PAGE_SIZE_OPTIONS.map((size) => (
              <option key={size} value={size}>
                {size}
              </option>
            ))}
          </select>
        </label>
        <div className="flex items-center gap-1">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-7 px-2 text-xs"
            disabled={page <= 1}
            onClick={() => onPageChange(page - 1)}
          >
            {prevLabel}
          </Button>
          <span className="px-2 tabular-nums">{pageOfLabel}</span>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-7 px-2 text-xs"
            disabled={page >= totalPages}
            onClick={() => onPageChange(page + 1)}
          >
            {nextLabel}
          </Button>
        </div>
      </div>
    </div>
  );
}

// ──────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────

function keyForEvent(event: ExceptionEventRow, index: number): string {
  return `${event.signature ?? "sig"}::${event.timestamp ?? ""}::${index}`;
}

/** Strip the package prefix from a fully-qualified Java/.NET class name.
 *  `java.lang.NullPointerException` → `NullPointerException`.
 *  `org.foo.Bar$Inner` → `Bar$Inner`. */
function simpleClassName(fqn: string): string {
  if (!fqn) return fqn;
  const lastDot = fqn.lastIndexOf(".");
  if (lastDot < 0) return fqn;
  return fqn.slice(lastDot + 1);
}

function formatTime(value: string | null | undefined): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(
    date.getDate(),
  )} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(
    date.getSeconds(),
  )}`;
}

function severityBadgeClass(severity: string): string {
  const base =
    "rounded px-1.5 py-0.5 text-[10px] font-medium uppercase tracking-wider";
  switch (severity.toLowerCase()) {
    case "error":
    case "critical":
      return `${base} bg-rose-500/15 text-rose-700 dark:text-rose-300`;
    case "warning":
    case "warn":
      return `${base} bg-amber-500/15 text-amber-700 dark:text-amber-300`;
    case "info":
      return `${base} bg-sky-500/15 text-sky-700 dark:text-sky-300`;
    default:
      return `${base} bg-muted text-muted-foreground`;
  }
}

// Unused-but-typed helper kept to mirror the AccessLog page surface;
// MessageKey usage keeps the import grounded so renames at the i18n
// layer surface here at compile time.
export type _ExceptionPageMessageKey = MessageKey;
