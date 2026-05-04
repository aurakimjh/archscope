import { Camera, Loader2, Play, Square } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import {
  createAnalyzerRequestId,
  getAnalyzerClient,
  type AnalysisResult,
  type AnalysisValue,
  type BridgeError,
  type ParserDiagnostics,
  type SelectFileRequest,
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useI18n } from "@/i18n/I18nProvider";
import { exportChartsInContainer } from "@/lib/batchExport";

type AnalyzerState = "idle" | "ready" | "running" | "error";

type PlaceholderAnalyzerProps = {
  fileLabel: string;
  fileDescription?: string;
  fileFilters?: SelectFileRequest["filters"];
  executionType?: "gc_log" | "thread_dump" | "exception_stack" | "jfr_recording";
};

type PlaceholderPageProps = {
  title: string;
  items: string[];
  analyzer?: PlaceholderAnalyzerProps;
};

const ACCEPT_BY_EXECUTION: Record<NonNullable<PlaceholderAnalyzerProps["executionType"]>, string> = {
  gc_log: ".log,.txt,.gz",
  thread_dump: ".txt,.log,.dump",
  exception_stack: ".log,.txt",
  jfr_recording: ".json",
};

const DROP_HINT_BY_EXECUTION: Record<
  NonNullable<PlaceholderAnalyzerProps["executionType"]>,
  | "dropOrBrowseGcLog"
  | "dropOrBrowseThreadDump"
  | "dropOrBrowseException"
  | "dropOrBrowseJfr"
> = {
  gc_log: "dropOrBrowseGcLog",
  thread_dump: "dropOrBrowseThreadDump",
  exception_stack: "dropOrBrowseException",
  jfr_recording: "dropOrBrowseJfr",
};

const EXPORT_PREFIX_BY_EXECUTION: Record<
  NonNullable<PlaceholderAnalyzerProps["executionType"]>,
  string
> = {
  gc_log: "gc-log",
  thread_dump: "thread-dump",
  exception_stack: "exception",
  jfr_recording: "jfr",
};

export function PlaceholderPage({
  title,
  items,
  analyzer,
}: PlaceholderPageProps): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(null);
  const [topN, setTopN] = useState(20);
  const [state, setState] = useState<AnalyzerState>("idle");
  const [error, setError] = useState<BridgeError | null>(null);
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const [savingAll, setSavingAll] = useState(false);
  const currentRequestIdRef = useRef<string | null>(null);
  const chartsContainerRef = useRef<HTMLDivElement | null>(null);

  const filePath = selectedFile?.filePath ?? "";
  const canAnalyze = Boolean(filePath) && state !== "running";

  const handleFileSelected = useCallback((file: FileDockSelection) => {
    setSelectedFile(file);
    setResult(null);
    setEngineMessages([]);
    setState("ready");
    setError(null);
  }, []);

  const handleClearFile = useCallback(() => {
    setSelectedFile(null);
    setResult(null);
    setEngineMessages([]);
    if (state !== "running") setState("idle");
  }, [state]);

  async function analyze(): Promise<void> {
    if (!analyzer || !canAnalyze) return;
    if (!analyzer.executionType) {
      setError({
        code: "ANALYZER_NOT_IMPLEMENTED",
        message: t("analyzerNotImplemented"),
      });
      setState("error");
      return;
    }

    setState("running");
    setError(null);
    setEngineMessages([]);
    const requestId = createAnalyzerRequestId();
    currentRequestIdRef.current = requestId;

    try {
      const response = await getAnalyzerClient().execute({
        requestId,
        type: analyzer.executionType,
        params: { requestId, filePath, topN },
      });

      if (currentRequestIdRef.current !== requestId) return;
      currentRequestIdRef.current = null;

      if (response.ok) {
        setResult(response.result);
        setEngineMessages(response.engine_messages ?? []);
        setState("ready");
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

  async function handleSaveAllCharts(): Promise<void> {
    const container = chartsContainerRef.current;
    if (!container || savingAll || !analyzer?.executionType) return;
    setSavingAll(true);
    try {
      await exportChartsInContainer(container, {
        format: "png",
        multiplier: 2,
        prefix: EXPORT_PREFIX_BY_EXECUTION[analyzer.executionType],
      });
    } catch (caught) {
      setError({
        code: "EXPORT_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
    } finally {
      setSavingAll(false);
    }
  }

  const threadStateBars = useMemo<BarDatum[] | null>(() => {
    if (!result || analyzer?.executionType !== "thread_dump") return null;
    const summary = result.summary as Record<string, AnalysisValue>;
    const fields: Array<{ key: string; label: string; color: string }> = [
      { key: "runnable_threads", label: "Runnable", color: "#22c55e" },
      { key: "blocked_threads", label: "Blocked", color: "#ef4444" },
      { key: "waiting_threads", label: "Waiting", color: "#f59e0b" },
      { key: "timed_waiting_threads", label: "Timed waiting", color: "#a855f7" },
      { key: "threads_with_locks", label: "Holding locks", color: "#0ea5e9" },
    ];
    const bars: BarDatum[] = [];
    for (const f of fields) {
      const value = summary[f.key];
      if (typeof value === "number") {
        bars.push({ id: f.key, label: f.label, value, color: f.color });
      }
    }
    return bars;
  }, [result, analyzer]);

  if (!analyzer) {
    return (
      <div className="flex flex-col gap-5">
        <Card>
          <CardHeader>
            <CardTitle>{title}</CardTitle>
          </CardHeader>
          <CardContent className="flex flex-wrap gap-2 text-sm text-muted-foreground">
            {items.map((item) => (
              <span
                key={item}
                className="rounded-md border border-border bg-muted/40 px-2.5 py-1"
              >
                {item}
              </span>
            ))}
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-5">
      <FileDock
        label={analyzer.fileLabel}
        description={
          analyzer.fileDescription ??
          (analyzer.executionType ? t(DROP_HINT_BY_EXECUTION[analyzer.executionType]) : undefined)
        }
        accept={analyzer.executionType ? ACCEPT_BY_EXECUTION[analyzer.executionType] : undefined}
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
              onChange={(event) => setTopN(Number(event.target.value))}
            />
          </label>
        </CardContent>
      </Card>

      <ErrorPanel
        error={error}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />
      <EngineMessagesPanel messages={engineMessages} title={t("engineMessages")} />

      <Tabs defaultValue="summary" className="w-full">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <TabsList>
            <TabsTrigger value="summary">{t("tabSummary")}</TabsTrigger>
            {threadStateBars && (
              <TabsTrigger value="charts">{t("tabCharts")}</TabsTrigger>
            )}
            <TabsTrigger value="raw">{t("tabEvents")}</TabsTrigger>
            <TabsTrigger value="diagnostics">{t("tabDiagnostics")}</TabsTrigger>
          </TabsList>
          {threadStateBars && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={!result || savingAll}
              onClick={() => void handleSaveAllCharts()}
            >
              {savingAll ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  {t("saveAllChartsBusy")}
                </>
              ) : (
                <>
                  <Camera className="h-3.5 w-3.5" />
                  {t("saveAllCharts")}
                </>
              )}
            </Button>
          )}
        </div>

        <TabsContent value="summary" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{title}</CardTitle>
            </CardHeader>
            <CardContent>
              {result ? (
                <SummaryGrid result={result} />
              ) : (
                <div className="flex flex-wrap gap-2 text-sm text-muted-foreground">
                  {items.map((item) => (
                    <span
                      key={item}
                      className="rounded-md border border-border bg-muted/40 px-2.5 py-1"
                    >
                      {item}
                    </span>
                  ))}
                </div>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        {threadStateBars && (
          <TabsContent value="charts" className="mt-4">
            <div ref={chartsContainerRef} className="grid gap-4">
              <D3BarChart
                title={t("threadStateDistribution")}
                exportName="thread-state-distribution"
                data={threadStateBars}
                orientation="horizontal"
                sort
                height={Math.max(180, threadStateBars.length * 28 + 40)}
              />
            </div>
          </TabsContent>
        )}

        <TabsContent value="raw" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("tabEvents")}</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto p-0">
              {result ? (
                <RawTablePreview result={result} />
              ) : (
                <p className="px-6 py-3 text-sm text-muted-foreground">—</p>
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
    </div>
  );
}

function SummaryGrid({ result }: { result: AnalysisResult }): JSX.Element {
  const entries = Object.entries(result.summary).slice(0, 16);
  return (
    <dl className="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4">
      {entries.map(([key, value]) => (
        <div
          key={key}
          className="rounded-md border border-border bg-muted/30 px-3 py-2 text-xs"
        >
          <dt className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
            {labelize(key)}
          </dt>
          <dd className="mt-1 text-sm tabular-nums">{formatValue(value)}</dd>
        </div>
      ))}
    </dl>
  );
}

function RawTablePreview({ result }: { result: AnalysisResult }): JSX.Element {
  const firstTable = Object.values(result.tables).find(Array.isArray);
  const rows = (Array.isArray(firstTable) ? firstTable.slice(0, 25) : []) as Array<
    Record<string, AnalysisValue>
  >;
  if (rows.length === 0) {
    return <p className="px-6 py-3 text-sm text-muted-foreground">—</p>;
  }
  const columns = Object.keys(rows[0]).slice(0, 8);
  return (
    <table className="w-full text-sm">
      <thead>
        <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
          {columns.map((key) => (
            <th key={key} className="px-3 py-2 text-left font-medium">
              {labelize(key)}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {rows.map((row, index) => (
          <tr key={index} className="border-b border-border last:border-0">
            {columns.map((key) => (
              <td key={key} className="px-3 py-2 font-mono text-xs">
                {formatValue(row[key])}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}

function labelize(value: string): string {
  return value.replace(/_/g, " ");
}

function formatValue(value: AnalysisValue | undefined): string {
  if (value === undefined || value === null) return "-";
  if (typeof value === "number") {
    return Number.isInteger(value) ? value.toLocaleString() : value.toFixed(2);
  }
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}
