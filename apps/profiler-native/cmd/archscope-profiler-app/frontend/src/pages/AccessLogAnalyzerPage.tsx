import { Loader2, Play, Square } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import { engine } from "@/bridge/engine";
import type {
  AccessLogAnalysisResult,
  AccessLogFormat,
  AccessLogStatusCodeRow,
  BridgeError,
} from "@/bridge/types";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { ChartPanel } from "@/components/ChartPanel";
import { MetricCard } from "@/components/MetricCard";
import {
  WailsFileDock,
  type FileDockSelection,
} from "@/components/WailsFileDock";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import {
  p95TrendOption,
  requestCountTrendOption,
  type ChartLabels,
} from "@/charts/accessLogCharts";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import {
  formatMilliseconds,
  formatNumber,
  formatPercent,
} from "@/utils/formatters";

// Wails port of apps/frontend/src/pages/AccessLogAnalyzerPage.tsx.
//
// Behavioural deltas vs. the web original:
//   - File path comes from a Wails native dialog (or drop) rather than an
//     HTTP multipart upload. We hand the engine the absolute path; it
//     reads the file off disk in-process.
//   - The web page persists every successful run to a "Dashboard" panel
//     and offers a Save-All-Charts batch exporter. Both lean on the
//     web-only chartFactory + html-to-image utilities and are deferred to
//     Phase 2 for parity. Their absence is invisible to the analyzer
//     surface itself.
//   - The web ChartPanel ships fullscreen + customizer + theme-toggle.
//     The Wails ChartPanel is a thin echarts host; charts still render,
//     they just don't carry per-panel chrome yet.
//
// Everything else — the metric grid, URL stats, status-class breakdown,
// error timeline table, parser diagnostics — lands as a faithful port.

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

export function AccessLogAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(
    null,
  );
  const [format, setFormat] = useState<AccessLogFormat>("nginx");
  const [maxLines, setMaxLines] = useState("");
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<AccessLogAnalysisResult | null>(null);
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const inflightRef = useRef<symbol | null>(null);

  const filePath = selectedFile?.filePath ?? "";
  const canAnalyze = Boolean(filePath) && state !== "running";
  const summary = result?.summary;

  const chartLabels = useMemo<ChartLabels>(
    () => ({
      requestsAxis: t("requestsAxis"),
      millisecondsAxis: t("millisecondsAxis"),
      p95Series: t("p95Series"),
    }),
    [t],
  );
  const requestsOption = useMemo(
    () => requestCountTrendOption(result, chartLabels),
    [chartLabels, result],
  );
  const p95Option = useMemo(
    () => p95TrendOption(result, chartLabels),
    [chartLabels, result],
  );

  const handleFileSelected = useCallback((file: FileDockSelection) => {
    setSelectedFile(file);
    setState("ready");
    setError(null);
    setEngineMessages([]);
  }, []);

  const handleClearFile = useCallback(() => {
    setSelectedFile(null);
    if (state !== "running") setState("idle");
  }, [state]);

  const analyze = useCallback(async () => {
    if (!canAnalyze) return;
    const parsedMaxLines = parseOptionalPositiveInteger(maxLines);
    if (parsedMaxLines === null) {
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
    const token = Symbol("access-log-analysis");
    inflightRef.current = token;

    try {
      const response = await engine.analyzeAccessLog({
        path: filePath,
        format,
        maxLines: parsedMaxLines ?? undefined,
        startTime: rfc3339OrUndefined(startTime),
        endTime: rfc3339OrUndefined(endTime),
      });

      if (inflightRef.current !== token) return;
      inflightRef.current = null;
      setResult(response);
      setState("success");
    } catch (caught) {
      if (inflightRef.current !== token) return;
      inflightRef.current = null;
      const message = caught instanceof Error ? caught.message : String(caught);
      setError({ code: "ENGINE_FAILED", message });
      setState("error");
    }
  }, [canAnalyze, endTime, filePath, format, maxLines, startTime, t]);

  const cancelAnalysis = useCallback(() => {
    // The synchronous AccessLog analyzer doesn't expose a cancel hook
    // (it returns a Promise<AccessLogAnalysisResult>, not an async
    // taskId). We just discard the in-flight token so the renderer
    // stops listening for the result.
    inflightRef.current = null;
    setState("ready");
    setEngineMessages([t("analysisCanceled")]);
  }, [t]);

  return (
    <main className="flex flex-col gap-5 p-5">
      <WailsFileDock
        label={t("selectAccessLogFile")}
        description={t("dropOrBrowseAccessLog")}
        accept=".log,.txt,.gz"
        selected={selectedFile}
        onSelect={handleFileSelected}
        onClear={handleClearFile}
        browseLabel={t("browseFile")}
        dropHereLabel={t("dropHere")}
        errorLabel={t("error")}
        fileFilters={[
          { displayName: "Access logs", pattern: "*.log;*.txt;*.gz" },
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

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">{t("analyzerOptions")}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-3 md:grid-cols-4">
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("logFormat")}
            </span>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={format}
              onChange={(event) => {
                setFormat(event.target.value as AccessLogFormat);
                if (filePath && state === "idle") setState("ready");
              }}
            >
              <option value="nginx">NGINX</option>
              <option value="apache">Apache</option>
              <option value="ohs">OHS</option>
              <option value="weblogic">WebLogic</option>
              <option value="tomcat">Tomcat</option>
              <option value="custom-regex">Custom Regex</option>
            </select>
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("maxLines")}
            </span>
            <Input
              type="number"
              min={1}
              placeholder="100000"
              value={maxLines}
              onChange={(event) => setMaxLines(event.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("startTime")}
            </span>
            <Input
              type="datetime-local"
              value={startTime}
              onChange={(event) => setStartTime(event.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("endTime")}
            </span>
            <Input
              type="datetime-local"
              value={endTime}
              onChange={(event) => setEndTime(event.target.value)}
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

      <Tabs defaultValue="summary" className="w-full">
        <TabsList>
          <TabsTrigger value="summary">{t("tabSummary")}</TabsTrigger>
          <TabsTrigger value="charts">{t("tabCharts")}</TabsTrigger>
          <TabsTrigger value="urls">{t("accessLogUrlsTab")}</TabsTrigger>
          <TabsTrigger value="status">{t("accessLogStatusTab")}</TabsTrigger>
          <TabsTrigger value="diagnostics">{t("tabDiagnostics")}</TabsTrigger>
        </TabsList>

        <TabsContent value="summary" className="mt-4">
          <section className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
            <MetricCard
              label={t("totalRequests")}
              value={formatNumber(summary?.total_requests)}
            />
            <MetricCard
              label={t("errorRate")}
              value={formatPercent(summary?.error_rate)}
            />
            <MetricCard
              label={t("averageResponseTime")}
              value={formatMilliseconds(summary?.avg_response_ms)}
            />
            <MetricCard
              label={t("accessLogP50")}
              value={formatMilliseconds(summary?.p50_response_ms)}
            />
            <MetricCard
              label={t("accessLogP90")}
              value={formatMilliseconds(summary?.p90_response_ms)}
            />
            <MetricCard
              label={t("p95ResponseTime")}
              value={formatMilliseconds(summary?.p95_response_ms)}
            />
            <MetricCard
              label={t("accessLogP99")}
              value={formatMilliseconds(summary?.p99_response_ms)}
            />
            <MetricCard
              label={t("accessLogTotalBytes")}
              value={formatBytesShort(summary?.total_bytes)}
            />
            <MetricCard
              label={t("accessLogAvgRps")}
              value={`${formatNumber(summary?.avg_requests_per_sec)} req/s`}
            />
            <MetricCard
              label={t("accessLogAvgThroughput")}
              value={`${formatBytesShort(summary?.avg_bytes_per_sec)}/s`}
            />
            <MetricCard
              label={t("accessLogStaticCount")}
              value={formatNumber(summary?.static_count)}
            />
            <MetricCard
              label={t("accessLogApiCount")}
              value={formatNumber(summary?.api_count)}
            />
          </section>
        </TabsContent>

        <TabsContent value="charts" className="mt-4">
          <div className="grid gap-4 md:grid-cols-2">
            <ChartPanel
              title={t("requestsAxis")}
              option={requestsOption}
              busy={state === "running"}
            />
            <ChartPanel
              title={t("p95Series")}
              option={p95Option}
              busy={state === "running"}
            />
          </div>
        </TabsContent>

        <TabsContent value="urls" className="mt-4">
          <UrlStatsPanel t={t} result={result} />
        </TabsContent>

        <TabsContent value="status" className="mt-4">
          <StatusBreakdownPanel t={t} result={result} />
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
// Helpers — kept inline so the page is a single self-contained file.
// ──────────────────────────────────────────────────────────────────

function parseOptionalPositiveInteger(
  value: string,
): number | undefined | null {
  if (!value.trim()) return undefined;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) return null;
  return parsed;
}

function rfc3339OrUndefined(value: string): string | undefined {
  const trimmed = value.trim();
  if (!trimmed) return undefined;
  // <input type=datetime-local> emits "YYYY-MM-DDTHH:MM" without a
  // timezone. The Go side parses RFC3339 strictly, so we attach the local
  // zone once before forwarding.
  if (/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}$/.test(trimmed)) {
    const date = new Date(trimmed);
    if (!Number.isNaN(date.getTime())) {
      return date.toISOString();
    }
  }
  return trimmed;
}

function formatBytesShort(value: number | undefined | null): string {
  if (value == null || !Number.isFinite(value) || value <= 0) return "—";
  const v = Number(value);
  if (v < 1024) return `${v} B`;
  if (v < 1024 * 1024) return `${(v / 1024).toFixed(1)} KB`;
  if (v < 1024 * 1024 * 1024) return `${(v / 1024 / 1024).toFixed(1)} MB`;
  return `${(v / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

type UrlSortKey =
  | "count"
  | "avg_response_ms"
  | "p95_response_ms"
  | "total_bytes"
  | "error_count";

const URL_SORT_KEYS: Array<{ key: UrlSortKey; labelKey: MessageKey }> = [
  { key: "count", labelKey: "accessLogSortCount" },
  { key: "avg_response_ms", labelKey: "accessLogSortAvg" },
  { key: "p95_response_ms", labelKey: "accessLogSortP95" },
  { key: "total_bytes", labelKey: "accessLogSortBytes" },
  { key: "error_count", labelKey: "accessLogSortErrors" },
];

function UrlStatsPanel({
  t,
  result,
}: {
  t: (key: MessageKey) => string;
  result: AccessLogAnalysisResult | null;
}): JSX.Element {
  const allUrls = result?.tables?.url_stats ?? [];
  const [sortKey, setSortKey] = useState<UrlSortKey>("count");
  const [classFilter, setClassFilter] = useState<"all" | "static" | "api">(
    "all",
  );

  const sorted = useMemo(() => {
    const filtered =
      classFilter === "all"
        ? allUrls
        : allUrls.filter((row) => row.classification === classFilter);
    return [...filtered].sort(
      (a, b) => Number(b[sortKey] ?? 0) - Number(a[sortKey] ?? 0),
    );
  }, [allUrls, sortKey, classFilter]);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
        <CardTitle className="text-sm">{t("accessLogUrlsTab")}</CardTitle>
        <div className="flex items-center gap-2 text-xs">
          <select
            className="h-7 rounded-md border border-border bg-background px-2 text-xs"
            value={classFilter}
            onChange={(e) =>
              setClassFilter(e.target.value as typeof classFilter)
            }
          >
            <option value="all">{t("accessLogClassAll")}</option>
            <option value="api">{t("accessLogClassApi")}</option>
            <option value="static">{t("accessLogClassStatic")}</option>
          </select>
          <span className="text-muted-foreground">{t("accessLogSortBy")}</span>
          <select
            className="h-7 rounded-md border border-border bg-background px-2 text-xs"
            value={sortKey}
            onChange={(e) => setSortKey(e.target.value as UrlSortKey)}
          >
            {URL_SORT_KEYS.map((entry) => (
              <option key={entry.key} value={entry.key}>
                {t(entry.labelKey)}
              </option>
            ))}
          </select>
        </div>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        {sorted.length === 0 ? (
          <p className="px-6 py-6 text-center text-sm text-muted-foreground">
            —
          </p>
        ) : (
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">
                  {t("accessLogColMethod")}
                </th>
                <th className="px-3 py-2 text-left font-medium">{t("uri")}</th>
                <th className="w-[60px] px-3 py-2 text-left font-medium">
                  {t("accessLogColClass")}
                </th>
                <th className="w-[70px] px-3 py-2 text-right font-medium">
                  {t("count")}
                </th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  {t("accessLogColAvg")}
                </th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  {t("accessLogColP95")}
                </th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  {t("accessLogColBytes")}
                </th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  {t("accessLogColErrors")}
                </th>
                <th className="w-[160px] px-3 py-2 text-right font-medium">
                  {t("accessLogColStatusMix")}
                </th>
              </tr>
            </thead>
            <tbody>
              {sorted.slice(0, 100).map((row) => (
                <tr
                  key={`${row.method ?? ""}-${row.uri}`}
                  className="border-b border-border last:border-0"
                >
                  <td className="px-3 py-1.5 font-mono text-[11px] text-muted-foreground">
                    {row.method || "—"}
                  </td>
                  <td
                    className="max-w-[420px] truncate px-3 py-1.5 font-mono"
                    title={row.uri}
                  >
                    {row.uri}
                  </td>
                  <td className="px-3 py-1.5">
                    <span
                      className={
                        row.classification === "static"
                          ? "rounded bg-sky-500/15 px-1.5 py-0.5 text-[10px] font-medium text-sky-700 dark:text-sky-300"
                          : "rounded bg-emerald-500/15 px-1.5 py-0.5 text-[10px] font-medium text-emerald-700 dark:text-emerald-300"
                      }
                    >
                      {row.classification ?? "api"}
                    </span>
                  </td>
                  <td className="px-3 py-1.5 text-right tabular-nums">
                    {row.count.toLocaleString()}
                  </td>
                  <td className="px-3 py-1.5 text-right tabular-nums">
                    {formatMilliseconds(row.avg_response_ms)}
                  </td>
                  <td className="px-3 py-1.5 text-right tabular-nums">
                    {formatMilliseconds(row.p95_response_ms)}
                  </td>
                  <td className="px-3 py-1.5 text-right tabular-nums">
                    {formatBytesShort(row.total_bytes)}
                  </td>
                  <td
                    className={`px-3 py-1.5 text-right tabular-nums ${
                      (row.error_count ?? 0) > 0
                        ? "font-semibold text-rose-600 dark:text-rose-400"
                        : ""
                    }`}
                  >
                    {row.error_count ?? 0}
                  </td>
                  <td className="px-3 py-1.5 text-right text-[10px] tabular-nums text-muted-foreground">
                    {row.status_2xx ?? 0}·{row.status_3xx ?? 0}·
                    <span className="text-amber-600 dark:text-amber-400">
                      {row.status_4xx ?? 0}
                    </span>
                    ·
                    <span className="text-rose-600 dark:text-rose-400">
                      {row.status_5xx ?? 0}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </CardContent>
    </Card>
  );
}

function StatusBreakdownPanel({
  t,
  result,
}: {
  t: (key: MessageKey) => string;
  result: AccessLogAnalysisResult | null;
}): JSX.Element {
  const codes = (result?.tables?.top_status_codes ?? []) as
    AccessLogStatusCodeRow[];
  const families = result?.series?.status_code_distribution ?? [];
  const errorPerMin = result?.series?.error_rate_per_minute ?? [];
  const statusOverTime = result?.series?.status_class_per_minute ?? [];
  const peakError = errorPerMin.length
    ? errorPerMin.reduce(
        (acc, row) => (row.value > acc.value ? row : acc),
        errorPerMin[0],
      )
    : null;
  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">
            {t("accessLogStatusFamiliesTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto p-0">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">
                  {t("accessLogColStatusFamily")}
                </th>
                <th className="w-[120px] px-3 py-2 text-right font-medium">
                  {t("count")}
                </th>
              </tr>
            </thead>
            <tbody>
              {families.map((row) => (
                <tr
                  key={row.status}
                  className="border-b border-border last:border-0"
                >
                  <td
                    className={`px-3 py-1.5 font-mono ${
                      row.status === "5xx"
                        ? "text-rose-600 dark:text-rose-400"
                        : row.status === "4xx"
                        ? "text-amber-600 dark:text-amber-400"
                        : ""
                    }`}
                  >
                    {row.status}
                  </td>
                  <td className="px-3 py-1.5 text-right tabular-nums">
                    {row.count.toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">
            {t("accessLogStatusCodesTitle")}
          </CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto p-0">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">
                  {t("accessLogColStatusCode")}
                </th>
                <th className="w-[120px] px-3 py-2 text-right font-medium">
                  {t("count")}
                </th>
              </tr>
            </thead>
            <tbody>
              {codes.map((row) => (
                <tr
                  key={row.status}
                  className="border-b border-border last:border-0"
                >
                  <td
                    className={`px-3 py-1.5 font-mono ${
                      row.status >= 500
                        ? "text-rose-600 dark:text-rose-400"
                        : row.status >= 400
                        ? "text-amber-600 dark:text-amber-400"
                        : ""
                    }`}
                  >
                    {row.status}
                  </td>
                  <td className="px-3 py-1.5 text-right tabular-nums">
                    {row.count.toLocaleString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">
            {t("accessLogErrorTimelineTitle")}
          </CardTitle>
          {peakError && peakError.value > 0 ? (
            <p className="text-xs text-muted-foreground">
              {t("accessLogPeakErrorAt")
                .replace("{minute}", peakError.time)
                .replace("{rate}", String(peakError.value))
                .replace("{errors}", String(peakError.errors))
                .replace("{total}", String(peakError.total))}
            </p>
          ) : null}
        </CardHeader>
        <CardContent className="overflow-x-auto p-0">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">
                  {t("accessLogColMinute")}
                </th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  2xx
                </th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  3xx
                </th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  4xx
                </th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  5xx
                </th>
                <th className="w-[100px] px-3 py-2 text-right font-medium">
                  {t("accessLogColErrorRate")}
                </th>
              </tr>
            </thead>
            <tbody>
              {statusOverTime.slice(-60).map((row, idx) => {
                const errorRate = errorPerMin.find((e) => e.time === row.time);
                const total =
                  row["2xx"] + row["3xx"] + row["4xx"] + row["5xx"] + row.other;
                const rate = errorRate?.value ?? 0;
                return (
                  <tr
                    key={`${row.time}-${idx}`}
                    className={`border-b border-border last:border-0 ${
                      rate >= 50 ? "bg-rose-100/40 dark:bg-rose-900/20" : ""
                    }`}
                  >
                    <td className="px-3 py-1 font-mono text-[11px]">
                      {row.time}
                    </td>
                    <td className="px-3 py-1 text-right tabular-nums">
                      {row["2xx"]}
                    </td>
                    <td className="px-3 py-1 text-right tabular-nums">
                      {row["3xx"]}
                    </td>
                    <td className="px-3 py-1 text-right tabular-nums text-amber-600 dark:text-amber-400">
                      {row["4xx"]}
                    </td>
                    <td className="px-3 py-1 text-right tabular-nums text-rose-600 dark:text-rose-400">
                      {row["5xx"]}
                    </td>
                    <td className="px-3 py-1 text-right tabular-nums">
                      {rate.toFixed(1)}% ({total})
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </CardContent>
      </Card>
    </div>
  );
}
