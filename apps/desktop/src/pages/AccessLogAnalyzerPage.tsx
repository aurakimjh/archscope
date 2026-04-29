import type { EChartsOption } from "echarts";
import { useMemo, useState } from "react";

import {
  getAnalyzerClient,
  type AccessLogAnalysisResult,
  type AccessLogFormat,
  type BridgeError,
  type TopUrlAvgResponseRow,
} from "../api/analyzerClient";
import { ChartPanel } from "../components/ChartPanel";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "../components/AnalyzerFeedback";
import { FileDropZone } from "../components/FileDropZone";
import { MetricCard } from "../components/MetricCard";
import { useI18n } from "../i18n/I18nProvider";

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

type SlowUrlRow = {
  uri: string;
  count: number;
  avg_response_ms: number;
};

export function AccessLogAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [filePath, setFilePath] = useState("");
  const [format, setFormat] = useState<AccessLogFormat>("nginx");
  const [maxLines, setMaxLines] = useState("");
  const [startTime, setStartTime] = useState("");
  const [endTime, setEndTime] = useState("");
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<AccessLogAnalysisResult | null>(null);
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);

  const canAnalyze = Boolean(filePath) && state !== "running";
  const summary = result?.summary;
  const chartOption = useMemo(() => buildRequestChartOption(result, t("requestsAxis")), [
    result,
    t,
  ]);
  const slowUrls = getSlowUrlRows(result?.series.top_urls_by_avg_response_time);

  async function browseFile(): Promise<void> {
    const response = await window.archscope?.selectFile?.({
      title: t("selectAccessLogFile"),
      filters: [
        { name: "Log files", extensions: ["log", "txt"] },
        { name: "All files", extensions: ["*"] },
      ],
    });

    if (response?.filePath) {
      setFilePath(response.filePath);
      setState("ready");
      setError(null);
      setEngineMessages([]);
    }
  }

  function handleFileInput(nextPath: string | undefined): void {
    if (!nextPath) {
      setError({
        code: "FILE_PATH_UNAVAILABLE",
        message: t("filePathUnavailable"),
      });
      setState("error");
      return;
    }

    setFilePath(nextPath);
    setState("ready");
    setError(null);
    setEngineMessages([]);
  }

  async function analyze(): Promise<void> {
    if (!canAnalyze) {
      return;
    }

    setState("running");
    setError(null);
    setEngineMessages([]);

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
        filePath,
        format,
        maxLines: parsedMaxLines,
        startTime: normalizeOptionalDateTime(startTime),
        endTime: normalizeOptionalDateTime(endTime),
      });

      if (response.ok) {
        setResult(response.result);
        setEngineMessages(response.engine_messages ?? []);
        setState("success");
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
    }
  }

  return (
    <div className="page">
      <section className="workspace-grid">
        <div className="tool-panel">
          <h2>{t("accessLogAnalyzer")}</h2>
          <FileDropZone
            label={t("selectAccessLogFile")}
            description={t("accessLogFileDescription")}
            selectedPath={filePath}
            browseLabel={t("browseFile")}
            onBrowse={browseFile}
            onFileSelected={handleFileInput}
          />
          <label className="field">
            <span>{t("logFormat")}</span>
            <select
              value={format}
              onChange={(event) => {
                setFormat(event.target.value as AccessLogFormat);
                if (filePath && state === "idle") {
                  setState("ready");
                }
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
          <div className="input-grid">
            <label className="field">
              <span>{t("maxLines")}</span>
              <input
                type="number"
                value={maxLines}
                min={1}
                placeholder="100000"
                onChange={(event) => setMaxLines(event.target.value)}
              />
            </label>
            <label className="field">
              <span>{t("startTime")}</span>
              <input
                type="datetime-local"
                value={startTime}
                onChange={(event) => setStartTime(event.target.value)}
              />
            </label>
            <label className="field">
              <span>{t("endTime")}</span>
              <input
                type="datetime-local"
                value={endTime}
                onChange={(event) => setEndTime(event.target.value)}
              />
            </label>
          </div>
          <button
            className="primary-button"
            type="button"
            disabled={!canAnalyze}
            onClick={() => void analyze()}
          >
            {state === "running" ? t("analyzing") : t("analyze")}
          </button>
          <ErrorPanel
            error={error}
            labels={{ title: t("analysisError"), code: t("errorCode") }}
          />
          <EngineMessagesPanel
            messages={engineMessages}
            title={t("engineMessages")}
          />
        </div>
        <div>
          <section className="summary-grid compact">
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
          <ChartPanel title={t("requestCountTrend")} option={chartOption} />
          <section className="table-panel diagnostics-panel">
            <div className="panel-header">
              <h2>{t("topUrlsByResponseTime")}</h2>
            </div>
            <table>
              <thead>
                <tr>
                  <th>{t("uri")}</th>
                  <th>{t("count")}</th>
                  <th>{t("responseTime")}</th>
                </tr>
              </thead>
              <tbody>
                {slowUrls.length > 0 ? (
                  slowUrls.map((row) => (
                    <tr key={row.uri}>
                      <td>{row.uri}</td>
                      <td>{row.count}</td>
                      <td>{formatMilliseconds(row.avg_response_ms)}</td>
                    </tr>
                  ))
                ) : (
                  <tr>
                    <td>-</td>
                    <td>-</td>
                    <td>-</td>
                  </tr>
                )}
              </tbody>
            </table>
          </section>
          <DiagnosticsPanel
            diagnostics={result?.metadata.diagnostics}
            labels={{
              title: t("parserDiagnostics"),
              parsedRecords: t("parsedRecords"),
              skippedLines: t("skippedLines"),
              samples: t("diagnosticSamples"),
            }}
          />
        </div>
      </section>
    </div>
  );
}

function buildRequestChartOption(
  result: AccessLogAnalysisResult | null,
  requestsAxis: string,
): EChartsOption {
  const rows = result?.series.requests_per_minute ?? [];

  return {
    tooltip: { trigger: "axis" },
    xAxis: { type: "category", data: rows.map((row) => row.time) },
    yAxis: { type: "value", name: requestsAxis },
    series: [
      {
        type: "bar",
        name: requestsAxis,
        data: rows.map((row) => row.value),
      },
    ],
  };
}

function getSlowUrlRows(value: TopUrlAvgResponseRow[] | undefined): SlowUrlRow[] {
  return value ?? [];
}

function formatNumber(value: number | undefined): string {
  return typeof value === "number" ? value.toLocaleString() : "-";
}

function formatMilliseconds(value: number | undefined): string {
  return typeof value === "number" ? `${value.toLocaleString()} ms` : "-";
}

function formatPercent(value: number | undefined): string {
  return typeof value === "number" ? `${value.toLocaleString()}%` : "-";
}


function parseOptionalPositiveInteger(value: string): number | undefined | null {
  if (!value.trim()) {
    return undefined;
  }

  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) {
    return null;
  }

  return parsed;
}


function normalizeOptionalDateTime(value: string): string | undefined {
  return value.trim() || undefined;
}
