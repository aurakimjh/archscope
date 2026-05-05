import { Loader2, Play, Search, Square } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import {
  createAnalyzerRequestId,
  getAnalyzerClient,
  type AnalysisValue,
  type BridgeError,
  type ExceptionStackAnalysisResult,
  type ExceptionStackSummary,
  type ParserDiagnostics,
} from "@/api/analyzerClient";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { FileDock, type FileDockSelection } from "@/components/FileDock";
import { D3BarChart, type BarDatum } from "@/components/charts/D3BarChart";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useI18n } from "@/i18n/I18nProvider";

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

type ExceptionEvent = {
  timestamp: string | null;
  language: string | null;
  exception_type: string | null;
  message: string | null;
  root_cause: string | null;
  signature: string | null;
  top_frame: string | null;
  stack: string[] | null;
};

type SignatureRow = {
  signature: string;
  count: number;
};

type TypeRow = {
  exception_type: string;
  count: number;
};

const DEFAULT_TOP_N = 50;
const PAGE_SIZE_OPTIONS = [10, 25, 50, 100] as const;

export function ExceptionAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(null);
  const [topN, setTopN] = useState(DEFAULT_TOP_N);
  const [state, setState] = useState<AnalyzerState>("idle");
  const [error, setError] = useState<BridgeError | null>(null);
  const [result, setResult] = useState<ExceptionStackAnalysisResult | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const [filter, setFilter] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState<number>(10);
  const [activeEvent, setActiveEvent] = useState<ExceptionEvent | null>(null);
  const [activeSignature, setActiveSignature] = useState<SignatureRow | null>(null);
  const currentRequestIdRef = useRef<string | null>(null);

  const filePath = selectedFile?.filePath ?? "";
  const canAnalyze = Boolean(filePath) && state !== "running";

  const handleFileSelected = useCallback((file: FileDockSelection) => {
    setSelectedFile(file);
    setResult(null);
    setEngineMessages([]);
    setState("ready");
    setError(null);
    setPage(1);
    setFilter("");
  }, []);

  const handleClearFile = useCallback(() => {
    setSelectedFile(null);
    setResult(null);
    setEngineMessages([]);
    if (state !== "running") setState("idle");
  }, [state]);

  async function analyze(): Promise<void> {
    if (!canAnalyze) return;
    setState("running");
    setError(null);
    setEngineMessages([]);
    setPage(1);
    const requestId = createAnalyzerRequestId();
    currentRequestIdRef.current = requestId;
    try {
      const response = await getAnalyzerClient().execute({
        requestId,
        type: "exception_stack",
        params: { requestId, filePath, topN },
      });
      if (currentRequestIdRef.current !== requestId) return;
      currentRequestIdRef.current = null;
      if (response.ok) {
        setResult(response.result as ExceptionStackAnalysisResult);
        setEngineMessages(response.engine_messages ?? []);
        setState("success");
        return;
      }
      if (response.error.code === "ENGINE_CANCELED") {
        setEngineMessages([t("analysisCanceled")]);
        setState("ready");
        return;
      }
      setError(response.error);
      setState("error");
    } catch (caught) {
      setError({
        code: "IPC_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
      setState("error");
    } finally {
      if (currentRequestIdRef.current === requestId) {
        currentRequestIdRef.current = null;
      }
    }
  }

  async function cancelAnalysis(): Promise<void> {
    const requestId = currentRequestIdRef.current;
    if (!requestId) return;
    const response = await getAnalyzerClient().cancel(requestId);
    if (!response.ok) {
      setError(response.error);
      setState("error");
    }
  }

  const summary = result?.summary as ExceptionStackSummary | undefined;
  const series = result?.series as Record<string, AnalysisValue[]> | undefined;
  const tableEvents = (result?.tables.exceptions ?? []) as unknown as ExceptionEvent[];

  const typeBars = useMemo<BarDatum[]>(() => {
    const rows = (series?.exception_type_distribution ?? []) as unknown as TypeRow[];
    return rows.map((row) => ({
      id: row.exception_type,
      label: simpleClassName(row.exception_type),
      fullLabel: row.exception_type,
      value: row.count,
    }));
  }, [series]);

  const signatureRows = useMemo<SignatureRow[]>(() => {
    const rows = (series?.top_stack_signatures ?? []) as unknown as SignatureRow[];
    return rows;
  }, [series]);

  const filteredEvents = useMemo<ExceptionEvent[]>(() => {
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

  return (
    <div className="flex flex-col gap-5">
      <FileDock
        label={t("selectExceptionFile")}
        description={t("dropOrBrowseException")}
        accept=".log,.txt"
        selected={selectedFile}
        onSelect={handleFileSelected}
        onClear={handleClearFile}
        browseLabel={t("browseFile")}
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
                onClick={() => void cancelAnalysis()}
              >
                <Square className="h-3.5 w-3.5" />
                {t("cancelAnalysis")}
              </Button>
            )}
          </div>
        }
      />

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">{t("analyzerOptions")}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-3 md:grid-cols-3">
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("topN")}</span>
            <Input
              type="number"
              min={1}
              value={topN}
              onChange={(event) => setTopN(Number(event.target.value) || DEFAULT_TOP_N)}
            />
          </label>
        </CardContent>
      </Card>

      <ErrorPanel
        error={error}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />
      <EngineMessagesPanel messages={engineMessages} title={t("engineMessages")} />

      {summary && (
        <section className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <SummaryCell label={t("exceptionTotalCount")} value={summary.total_exceptions} />
          <SummaryCell label={t("exceptionUniqueTypes")} value={summary.unique_exception_types} />
          <SummaryCell label={t("exceptionUniqueSignatures")} value={summary.unique_signatures} />
          <SummaryCell
            label={t("exceptionTopTypeLabel")}
            value={summary.top_exception_type ?? "—"}
          />
        </section>
      )}

      <Tabs defaultValue="events" className="w-full">
        <TabsList>
          <TabsTrigger value="events">{t("exceptionTabEvents")}</TabsTrigger>
          <TabsTrigger value="types">{t("exceptionTabTopTypes")}</TabsTrigger>
          <TabsTrigger value="signatures">{t("exceptionTabSignatures")}</TabsTrigger>
          <TabsTrigger value="diagnostics">{t("tabDiagnostics")}</TabsTrigger>
        </TabsList>

        <TabsContent value="events" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <CardTitle className="text-sm">{t("exceptionTabEvents")}</CardTitle>
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
                  labels={{
                    time: t("exceptionEventTime"),
                    type: t("exceptionEventType"),
                    topFrame: t("exceptionEventTopFrame"),
                    language: t("exceptionEventLanguage"),
                    showDetails: t("exceptionShowDetails"),
                    frameCount: (n) => t("exceptionFrameCount").replace("{n}", String(n)),
                  }}
                  onRowClick={setActiveEvent}
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
        </TabsContent>

        <TabsContent value="types" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("exceptionsByType")}</CardTitle>
            </CardHeader>
            <CardContent>
              {typeBars.length === 0 ? (
                <p className="text-sm text-muted-foreground">—</p>
              ) : (
                <D3BarChart
                  title={t("exceptionsByType")}
                  exportName="exception-type-distribution"
                  data={typeBars}
                  orientation="horizontal"
                  sort
                  height={Math.max(220, typeBars.length * 26 + 48)}
                />
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="signatures" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("exceptionsBySignature")}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              {signatureRows.length === 0 ? (
                <p className="px-6 py-6 text-sm text-muted-foreground">—</p>
              ) : (
                <SignatureTable
                  rows={signatureRows}
                  labels={{
                    signature: t("exceptionEventSignature"),
                    count: t("count"),
                    showDetails: t("exceptionShowDetails"),
                  }}
                  onRowClick={setActiveSignature}
                />
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="diagnostics" className="mt-4">
          <DiagnosticsPanel
            diagnostics={result?.metadata.diagnostics as ParserDiagnostics | undefined}
            labels={{
              title: t("parserDiagnostics"),
              parsedRecords: t("parsedRecords"),
              skippedLines: t("skippedLines"),
              samples: t("diagnosticSamples"),
            }}
          />
        </TabsContent>
      </Tabs>

      <Sheet open={activeEvent !== null} onOpenChange={(open) => !open && setActiveEvent(null)}>
        <SheetContent side="right" className="w-full sm:max-w-2xl">
          {activeEvent && (
            <EventDetailContent
              event={activeEvent}
              labels={{
                title: t("exceptionEventDetails"),
                time: t("exceptionEventTime"),
                type: t("exceptionEventType"),
                language: t("exceptionEventLanguage"),
                message: t("exceptionEventMessage"),
                rootCause: t("exceptionEventRootCause"),
                signature: t("exceptionEventSignature"),
                stack: t("exceptionEventStack"),
              }}
            />
          )}
        </SheetContent>
      </Sheet>

      <Sheet
        open={activeSignature !== null}
        onOpenChange={(open) => !open && setActiveSignature(null)}
      >
        <SheetContent side="right" className="w-full sm:max-w-2xl">
          {activeSignature && (
            <SignatureDetailContent
              row={activeSignature}
              labels={{
                title: t("exceptionEventSignature"),
                count: t("count"),
                signature: t("exceptionEventSignature"),
              }}
            />
          )}
        </SheetContent>
      </Sheet>
    </div>
  );
}

function SummaryCell({ label, value }: { label: string; value: string | number }): JSX.Element {
  const isNumber = typeof value === "number";
  return (
    <div className="min-w-0 overflow-hidden rounded-md border border-border bg-muted/30 px-3 py-2 text-xs">
      <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      {isNumber ? (
        <div className="mt-1 truncate text-sm tabular-nums" title={String(value)}>
          {value.toLocaleString()}
        </div>
      ) : (
        <div
          className="mt-1 break-all text-sm leading-snug line-clamp-2"
          title={String(value)}
        >
          {value}
        </div>
      )}
    </div>
  );
}

type EventsTableProps = {
  events: ExceptionEvent[];
  labels: {
    time: string;
    type: string;
    topFrame: string;
    language: string;
    showDetails: string;
    frameCount: (n: number) => string;
  };
  onRowClick: (event: ExceptionEvent) => void;
};

function EventsTable({ events, labels, onRowClick }: EventsTableProps): JSX.Element {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/40 text-[11px] uppercase tracking-wider text-muted-foreground">
            <th className="w-[150px] px-3 py-2 text-left font-medium">{labels.time}</th>
            <th className="w-[60px] px-3 py-2 text-left font-medium">{labels.language}</th>
            <th className="w-[220px] px-3 py-2 text-left font-medium">{labels.type}</th>
            <th className="px-3 py-2 text-left font-medium">{labels.topFrame}</th>
            <th className="w-[80px] px-3 py-2 text-right font-medium">{""}</th>
          </tr>
        </thead>
        <tbody>
          {events.map((evt, index) => {
            const frameCount = evt.stack?.length ?? 0;
            return (
              <tr
                key={`${evt.signature ?? "sig"}-${index}`}
                className="cursor-pointer border-b border-border last:border-0 hover:bg-muted/40"
                onClick={() => onRowClick(evt)}
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
                      onRowClick(evt);
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
  rows: SignatureRow[];
  labels: {
    signature: string;
    count: string;
    showDetails: string;
  };
  onRowClick: (row: SignatureRow) => void;
};

function SignatureTable({ rows, labels, onRowClick }: SignatureTableProps): JSX.Element {
  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-border bg-muted/40 text-[11px] uppercase tracking-wider text-muted-foreground">
            <th className="px-3 py-2 text-left font-medium">{labels.signature}</th>
            <th className="w-[80px] px-3 py-2 text-right font-medium">{labels.count}</th>
            <th className="w-[80px] px-3 py-2 text-right font-medium">{""}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr
              key={`${row.signature}-${index}`}
              className="cursor-pointer border-b border-border last:border-0 hover:bg-muted/40"
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
              <td className="px-3 py-2 text-right tabular-nums">{row.count.toLocaleString()}</td>
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
          ))}
        </tbody>
      </table>
    </div>
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

type EventDetailLabels = {
  title: string;
  time: string;
  type: string;
  language: string;
  message: string;
  rootCause: string;
  signature: string;
  stack: string;
};

function EventDetailContent({
  event,
  labels,
}: {
  event: ExceptionEvent;
  labels: EventDetailLabels;
}): JSX.Element {
  return (
    <div className="flex h-full flex-col">
      <SheetHeader>
        <SheetTitle>{event.exception_type ?? labels.title}</SheetTitle>
        <SheetDescription className="font-mono text-xs">
          {formatTime(event.timestamp) || "—"}
          {event.language && <span className="ml-2 uppercase">{event.language}</span>}
        </SheetDescription>
      </SheetHeader>

      <div className="mt-4 flex-1 space-y-4 overflow-y-auto pr-1 text-sm">
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
      </div>
    </div>
  );
}

function SignatureDetailContent({
  row,
  labels,
}: {
  row: SignatureRow;
  labels: { title: string; count: string; signature: string };
}): JSX.Element {
  return (
    <div className="flex h-full flex-col">
      <SheetHeader>
        <SheetTitle>{labels.title}</SheetTitle>
        <SheetDescription>
          {labels.count}: <span className="tabular-nums">{row.count.toLocaleString()}</span>
        </SheetDescription>
      </SheetHeader>
      <div className="mt-4 flex-1 overflow-y-auto pr-1">
        <pre className="rounded-md border border-border bg-muted/40 p-3 font-mono text-[11px] leading-relaxed">
          {row.signature}
        </pre>
      </div>
    </div>
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

/** Strip the package prefix from a fully-qualified Java/.NET class name.
 *  `java.lang.NullPointerException` → `NullPointerException`.
 *  `org.foo.Bar$Inner` → `Bar$Inner`. */
function simpleClassName(fqn: string): string {
  const lastDot = fqn.lastIndexOf(".");
  if (lastDot < 0) return fqn;
  return fqn.slice(lastDot + 1);
}

function formatTime(value: string | null | undefined): string {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(
    date.getHours(),
  )}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}
