import { Camera, Loader2, Play, Square } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import {
  createAnalyzerRequestId,
  getAnalyzerClient,
  type AccessLogAnalysisResult,
  type AccessLogFormat,
  type AccessLogStatusCodeRow,
  type AccessLogUrlStatRow,
  type BridgeError,
  type TopUrlAvgResponseRow,
} from "@/api/analyzerClient";
import { ChartPanel } from "@/components/ChartPanel";
import { FileDock, type FileDockSelection } from "@/components/FileDock";
import { MetricCard } from "@/components/MetricCard";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import { exportChartsInContainer } from "@/lib/batchExport";
import { createChartOption } from "@/charts/chartFactory";
import type { ChartLabels } from "@/charts/chartOptions";
import { getChartTemplate } from "@/charts/chartTemplates";
import { saveDashboardResult } from "@/pages/DashboardPage";
import {
  formatMilliseconds,
  formatNumber,
  formatPercent,
} from "@/utils/formatters";

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

type SlowUrlRow = {
  uri: string;
  count: number;
  avg_response_ms: number;
};

export function AccessLogAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(null);
  const [format, setFormat] = useState<AccessLogFormat>("nginx");
  const [maxLines, setMaxLines] = useState("");
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<AccessLogAnalysisResult | null>(null);
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const [savingAll, setSavingAll] = useState(false);
  const currentRequestIdRef = useRef<string | null>(null);
  const chartsContainerRef = useRef<HTMLDivElement | null>(null);

  const filePath = selectedFile?.filePath ?? "";
  const canAnalyze = Boolean(filePath) && state !== "running";
  const summary = result?.summary;

  const requestChartTemplate = getChartTemplate("AccessLog.RequestCountTrend");
  const chartLabels = useMemo(() => createChartLabels(t), [t]);
  const chartOption = useMemo(
    () =>
      createChartOption(requestChartTemplate.id, result ?? emptyAccessLogResult, chartLabels),
    [chartLabels, requestChartTemplate.id, result],
  );
  const slowUrls = getSlowUrlRows(result?.series.top_urls_by_avg_response_time);

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

  async function analyze(): Promise<void> {
    if (!canAnalyze) return;

    setState("running");
    setError(null);
    setEngineMessages([]);
    const requestId = createAnalyzerRequestId();
    currentRequestIdRef.current = requestId;

    try {
      const parsedMaxLines = parseOptionalPositiveInteger(maxLines);
      if (parsedMaxLines === null) {
        setError({
          code: "INVALID_OPTION",
          message: t("invalidAnalyzerOptions"),
        });
        setState("error");
        return;
      }

      const response = await getAnalyzerClient().analyzeAccessLog({
        requestId,
        filePath,
        format,
        maxLines: parsedMaxLines,
        startTime: normalizeOptionalDateTime(startTime),
        endTime: normalizeOptionalDateTime(endTime),
      });

      if (currentRequestIdRef.current !== requestId) return;
      currentRequestIdRef.current = null;

      if (response.ok) {
        setResult(response.result);
        setEngineMessages(response.engine_messages ?? []);
        setState("success");
        saveDashboardResult(response.result);
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
    if (!container || savingAll) return;
    setSavingAll(true);
    try {
      await exportChartsInContainer(container, {
        format: "png",
        multiplier: 2,
        prefix: "access-log",
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

  return (
    <div className="flex flex-col gap-5">
      <FileDock
        label={t("selectAccessLogFile")}
        description={t("dropOrBrowseAccessLog")}
        accept=".log,.txt,.gz"
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
        <CardContent className="grid grid-cols-1 gap-3 md:grid-cols-4">
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("logFormat")}</span>
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
            <span className="font-medium text-foreground/80">{t("maxLines")}</span>
            <Input
              type="number"
              min={1}
              placeholder="100000"
              value={maxLines}
              onChange={(event) => setMaxLines(event.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("startTime")}</span>
            <Input
              type="datetime-local"
              value={startTime}
              onChange={(event) => setStartTime(event.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("endTime")}</span>
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
      <EngineMessagesPanel messages={engineMessages} title={t("engineMessages")} />

      <Tabs defaultValue="summary" className="w-full">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <TabsList>
            <TabsTrigger value="summary">{t("tabSummary")}</TabsTrigger>
            <TabsTrigger value="charts">{t("tabCharts")}</TabsTrigger>
            <TabsTrigger value="urls">{t("accessLogUrlsTab")}</TabsTrigger>
            <TabsTrigger value="status">{t("accessLogStatusTab")}</TabsTrigger>
            <TabsTrigger value="diagnostics">{t("tabDiagnostics")}</TabsTrigger>
          </TabsList>
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
        </div>

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
          <div ref={chartsContainerRef} className="grid gap-4">
            <ChartPanel
              title={t(requestChartTemplate.titleKey)}
              option={chartOption}
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
    </div>
  );
}

function getSlowUrlRows(value: TopUrlAvgResponseRow[] | undefined): SlowUrlRow[] {
  return value ?? [];
}

function parseOptionalPositiveInteger(value: string): number | undefined | null {
  if (!value.trim()) return undefined;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) return null;
  return parsed;
}

function normalizeOptionalDateTime(value: string): string | undefined {
  return value.trim() || undefined;
}

function createChartLabels(t: (key: MessageKey) => string): ChartLabels {
  return {
    requestsAxis: t("requestsAxis"),
    millisecondsAxis: t("millisecondsAxis"),
    statusSeries: t("statusSeries"),
    samplesAxis: t("samplesAxis"),
    p95Series: t("p95Series"),
  };
}

const emptyAccessLogResult: AccessLogAnalysisResult = {
  type: "access_log",
  source_files: [],
  created_at: "",
  summary: {
    total_requests: 0,
    avg_response_ms: 0,
    p95_response_ms: 0,
    p99_response_ms: 0,
    error_rate: 0,
  },
  series: {
    requests_per_minute: [],
    avg_response_time_per_minute: [],
    p95_response_time_per_minute: [],
    status_code_distribution: [],
    top_urls_by_count: [],
    top_urls_by_avg_response_time: [],
  },
  tables: { sample_records: [] },
  charts: {},
  metadata: {
    format: "nginx",
    parser: "nginx_combined_with_response_time",
    schema_version: "0.1.0",
    diagnostics: {
      total_lines: 0,
      parsed_records: 0,
      skipped_lines: 0,
      skipped_by_reason: {},
      samples: [],
    },
    analysis_options: {
      max_lines: null,
      start_time: null,
      end_time: null,
    },
    findings: [],
  },
};

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
  const [classFilter, setClassFilter] = useState<"all" | "static" | "api">("all");

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
            onChange={(e) => setClassFilter(e.target.value as typeof classFilter)}
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
          <p className="px-6 py-6 text-center text-sm text-muted-foreground">—</p>
        ) : (
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">{t("accessLogColMethod")}</th>
                <th className="px-3 py-2 text-left font-medium">{t("uri")}</th>
                <th className="w-[60px] px-3 py-2 text-left font-medium">
                  {t("accessLogColClass")}
                </th>
                <th className="w-[70px] px-3 py-2 text-right font-medium">{t("count")}</th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">{t("accessLogColAvg")}</th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">{t("accessLogColP95")}</th>
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
                  <td className="max-w-[420px] truncate px-3 py-1.5 font-mono" title={row.uri}>
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
                    {(row.status_2xx ?? 0)}·{(row.status_3xx ?? 0)}·
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
  const codes = (result?.tables?.top_status_codes ?? []) as AccessLogStatusCodeRow[];
  const families = result?.series?.status_code_distribution ?? [];
  const errorPerMin = result?.series?.error_rate_per_minute ?? [];
  const statusOverTime = result?.series?.status_class_per_minute ?? [];
  const peakError = errorPerMin.length
    ? errorPerMin.reduce((acc, row) => (row.value > acc.value ? row : acc), errorPerMin[0])
    : null;
  return (
    <div className="grid gap-4">
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">{t("accessLogStatusFamiliesTitle")}</CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto p-0">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">
                  {t("accessLogColStatusFamily")}
                </th>
                <th className="w-[120px] px-3 py-2 text-right font-medium">{t("count")}</th>
              </tr>
            </thead>
            <tbody>
              {families.map((row) => (
                <tr key={row.status} className="border-b border-border last:border-0">
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
          <CardTitle className="text-sm">{t("accessLogStatusCodesTitle")}</CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto p-0">
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">{t("accessLogColStatusCode")}</th>
                <th className="w-[120px] px-3 py-2 text-right font-medium">{t("count")}</th>
              </tr>
            </thead>
            <tbody>
              {codes.map((row) => (
                <tr key={row.status} className="border-b border-border last:border-0">
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
          <CardTitle className="text-sm">{t("accessLogErrorTimelineTitle")}</CardTitle>
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
                <th className="px-3 py-2 text-left font-medium">{t("accessLogColMinute")}</th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">2xx</th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">3xx</th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">4xx</th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">5xx</th>
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
                    <td className="px-3 py-1 font-mono text-[11px]">{row.time}</td>
                    <td className="px-3 py-1 text-right tabular-nums">{row["2xx"]}</td>
                    <td className="px-3 py-1 text-right tabular-nums">{row["3xx"]}</td>
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
