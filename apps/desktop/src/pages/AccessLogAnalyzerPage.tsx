import { Camera, Loader2, Play, Square } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import {
  createAnalyzerRequestId,
  getAnalyzerClient,
  type AccessLogAnalysisResult,
  type AccessLogFormat,
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
            <TabsTrigger value="top-urls">{t("tabTopUrls")}</TabsTrigger>
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
          <section className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <MetricCard
              label={t("totalRequests")}
              value={formatNumber(summary?.total_requests)}
            />
            <MetricCard
              label={t("averageResponseTime")}
              value={formatMilliseconds(summary?.avg_response_ms)}
            />
            <MetricCard
              label={t("p95ResponseTime")}
              value={formatMilliseconds(summary?.p95_response_ms)}
            />
            <MetricCard
              label={t("errorRate")}
              value={formatPercent(summary?.error_rate)}
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

        <TabsContent value="top-urls" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("topUrlsByResponseTime")}</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                    <th className="px-4 py-2 text-left font-medium">{t("uri")}</th>
                    <th className="px-4 py-2 text-right font-medium">{t("count")}</th>
                    <th className="px-4 py-2 text-right font-medium">{t("responseTime")}</th>
                  </tr>
                </thead>
                <tbody>
                  {slowUrls.length > 0 ? (
                    slowUrls.map((row) => (
                      <tr key={row.uri} className="border-b border-border last:border-0">
                        <td className="px-4 py-2 font-mono text-xs">{row.uri}</td>
                        <td className="px-4 py-2 text-right tabular-nums">{row.count}</td>
                        <td className="px-4 py-2 text-right tabular-nums">
                          {formatMilliseconds(row.avg_response_ms)}
                        </td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td className="px-4 py-3 text-muted-foreground" colSpan={3}>
                        —
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </CardContent>
          </Card>
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
