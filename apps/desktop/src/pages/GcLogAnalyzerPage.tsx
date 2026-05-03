import { useMemo, useRef, useState } from "react";

import {
  createAnalyzerRequestId,
  getAnalyzerClient,
  type BridgeError,
  type GcEventRow,
  type GcLogAnalysisResult,
} from "../api/analyzerClient";
import { saveDashboardResult } from "./DashboardPage";
import { createChartOption } from "../charts/chartFactory";
import type { GcChartLabels } from "../charts/chartOptions";
import { getChartTemplate } from "../charts/chartTemplates";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "../components/AnalyzerFeedback";
import { ChartPanel } from "../components/ChartPanel";
import { FileDropZone } from "../components/FileDropZone";
import { MetricCard } from "../components/MetricCard";
import type { MessageKey } from "../i18n/messages";
import { useI18n } from "../i18n/I18nProvider";
import { formatMilliseconds, formatNumber, formatPercent } from "../utils/formatters";

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

const MAX_EVENT_ROWS = 200;

export function GcLogAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [filePath, setFilePath] = useState("");
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<GcLogAnalysisResult | null>(null);
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const currentRequestIdRef = useRef<string | null>(null);

  const canAnalyze = Boolean(filePath) && state !== "running";
  const summary = result?.summary;

  const gcChartLabels = useMemo(() => createGcChartLabels(t), [t]);

  const pauseTimelineTemplate = getChartTemplate("GcLog.PauseTimeline");
  const pauseHistogramTemplate = getChartTemplate("GcLog.PauseHistogram");
  const heapBeforeAfterTemplate = getChartTemplate("GcLog.HeapBeforeAfter");
  const allocationRateTemplate = getChartTemplate("GcLog.AllocationRate");
  const typeDistTemplate = getChartTemplate("GcLog.TypeDistribution");
  const causeDistTemplate = getChartTemplate("GcLog.CauseDistribution");

  const pauseTimelineOption = useMemo(
    () => createChartOption(pauseTimelineTemplate.id, result ?? emptyGcResult, gcChartLabels),
    [gcChartLabels, pauseTimelineTemplate.id, result],
  );
  const pauseHistogramOption = useMemo(
    () => createChartOption(pauseHistogramTemplate.id, result ?? emptyGcResult, gcChartLabels),
    [gcChartLabels, pauseHistogramTemplate.id, result],
  );
  const heapBeforeAfterOption = useMemo(
    () => createChartOption(heapBeforeAfterTemplate.id, result ?? emptyGcResult, gcChartLabels),
    [gcChartLabels, heapBeforeAfterTemplate.id, result],
  );
  const allocationRateOption = useMemo(
    () => createChartOption(allocationRateTemplate.id, result ?? emptyGcResult, gcChartLabels),
    [gcChartLabels, allocationRateTemplate.id, result],
  );
  const typeDistOption = useMemo(
    () => createChartOption(typeDistTemplate.id, result ?? emptyGcResult, gcChartLabels),
    [gcChartLabels, typeDistTemplate.id, result],
  );
  const causeDistOption = useMemo(
    () => createChartOption(causeDistTemplate.id, result ?? emptyGcResult, gcChartLabels),
    [gcChartLabels, causeDistTemplate.id, result],
  );

  const eventRows = getTopEventRows(result?.tables?.events);
  const findings = result?.metadata?.findings ?? [];

  async function browseFile(): Promise<void> {
    const response = await window.archscope?.selectFile?.({
      title: t("selectGcLogFile"),
      filters: [
        { name: t("logFilesFilter"), extensions: ["log", "txt"] },
        { name: t("allFilesFilter"), extensions: ["*"] },
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
    const requestId = createAnalyzerRequestId();
    currentRequestIdRef.current = requestId;

    try {
      const response = await getAnalyzerClient().execute({
        requestId,
        type: "gc_log",
        params: { filePath },
      });

      if (currentRequestIdRef.current !== requestId) {
        return;
      }
      currentRequestIdRef.current = null;

      if (response.ok) {
        const gcResult = response.result as GcLogAnalysisResult;
        setResult(gcResult);
        setEngineMessages(response.engine_messages ?? []);
        setState("success");
        saveDashboardResult(gcResult);
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
    if (!requestId) {
      return;
    }
    const response = await getAnalyzerClient().cancel(requestId);
    if (!response.ok) {
      setError(response.error);
      setState("error");
    }
  }

  return (
    <div className="page">
      <section className="workspace-grid">
        <div className="tool-panel">
          <h2>{t("gcLogAnalyzer")}</h2>
          <FileDropZone
            label={t("selectGcLogFile")}
            description={t("gcLogFileDescription")}
            selectedPath={filePath}
            browseLabel={t("browseFile")}
            onBrowse={browseFile}
            onFileSelected={handleFileInput}
          />
          <div className="button-row">
            <button
              className="primary-button"
              type="button"
              disabled={!canAnalyze}
              onClick={() => void analyze()}
            >
              {state === "running" ? t("analyzing") : t("analyze")}
            </button>
            <button
              className="secondary-button"
              type="button"
              disabled={state !== "running"}
              onClick={() => void cancelAnalysis()}
            >
              {t("cancelAnalysis")}
            </button>
          </div>
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
              label={t("throughputPercent")}
              value={formatPercent(summary?.throughput_percent)}
            />
            <MetricCard
              label={t("totalEvents")}
              value={formatNumber(summary?.total_events)}
            />
            <MetricCard
              label={t("p50PauseMs")}
              value={formatMilliseconds(summary?.p50_pause_ms)}
            />
            <MetricCard
              label={t("p95PauseMs")}
              value={formatMilliseconds(summary?.p95_pause_ms)}
            />
            <MetricCard
              label={t("p99PauseMs")}
              value={formatMilliseconds(summary?.p99_pause_ms)}
            />
            <MetricCard
              label={t("maxPauseMs")}
              value={formatMilliseconds(summary?.max_pause_ms)}
            />
            <MetricCard
              label={t("avgPauseMs")}
              value={formatMilliseconds(summary?.avg_pause_ms)}
            />
            <MetricCard
              label={t("totalPauseMs")}
              value={formatMilliseconds(summary?.total_pause_ms)}
            />
            <MetricCard
              label={t("youngGcCount")}
              value={formatNumber(summary?.young_gc_count)}
            />
            <MetricCard
              label={t("fullGcCount")}
              value={formatNumber(summary?.full_gc_count)}
            />
            <MetricCard
              label={t("avgAllocationRate")}
              value={formatMbPerSec(summary?.avg_allocation_rate_mb_per_sec)}
            />
            <MetricCard
              label={t("avgPromotionRate")}
              value={formatMbPerSec(summary?.avg_promotion_rate_mb_per_sec)}
            />
            <MetricCard
              label={t("humongousAllocationCount")}
              value={formatNumber(summary?.humongous_allocation_count)}
            />
            <MetricCard
              label={t("concurrentModeFailureCount")}
              value={formatNumber(summary?.concurrent_mode_failure_count)}
            />
            <MetricCard
              label={t("promotionFailureCount")}
              value={formatNumber(summary?.promotion_failure_count)}
            />
          </section>
          <ChartPanel
            title={t(pauseTimelineTemplate.titleKey)}
            option={pauseTimelineOption}
            busy={state === "running"}
          />
          <ChartPanel
            title={t(pauseHistogramTemplate.titleKey)}
            option={pauseHistogramOption}
            busy={state === "running"}
          />
          <ChartPanel
            title={t(heapBeforeAfterTemplate.titleKey)}
            option={heapBeforeAfterOption}
            busy={state === "running"}
          />
          <ChartPanel
            title={t(allocationRateTemplate.titleKey)}
            option={allocationRateOption}
            busy={state === "running"}
          />
          <ChartPanel
            title={t(typeDistTemplate.titleKey)}
            option={typeDistOption}
            busy={state === "running"}
          />
          <ChartPanel
            title={t(causeDistTemplate.titleKey)}
            option={causeDistOption}
            busy={state === "running"}
          />
          <section className="table-panel diagnostics-panel">
            <div className="panel-header">
              <h2>{t("gcEventTable")}</h2>
            </div>
            <table>
              <thead>
                <tr>
                  <th>{t("timestampLabel")}</th>
                  <th>{t("uptimeSec")}</th>
                  <th>{t("gcTypeLabel")}</th>
                  <th>{t("gcCauseLabel")}</th>
                  <th>{t("pauseMsAxis")}</th>
                  <th>{t("heapBeforeMb")}</th>
                  <th>{t("heapAfterMb")}</th>
                  <th>{t("heapCommittedMb")}</th>
                </tr>
              </thead>
              <tbody>
                {eventRows.length > 0 ? (
                  eventRows.map((row, idx) => (
                    <tr key={idx}>
                      <td>{row.timestamp ?? "-"}</td>
                      <td>{row.uptime_sec != null ? row.uptime_sec.toFixed(3) : "-"}</td>
                      <td>{row.gc_type ?? "-"}</td>
                      <td>{row.cause ?? "-"}</td>
                      <td>{row.pause_ms != null ? row.pause_ms.toFixed(2) : "-"}</td>
                      <td>{row.heap_before_mb != null ? row.heap_before_mb.toFixed(1) : "-"}</td>
                      <td>{row.heap_after_mb != null ? row.heap_after_mb.toFixed(1) : "-"}</td>
                      <td>{row.heap_committed_mb != null ? row.heap_committed_mb.toFixed(1) : "-"}</td>
                    </tr>
                  ))
                ) : (
                  <tr>
                    <td colSpan={8}>-</td>
                  </tr>
                )}
              </tbody>
            </table>
          </section>
          {findings.length > 0 && (
            <section className="table-panel diagnostics-panel">
              <div className="panel-header">
                <h2>{t("findings")}</h2>
              </div>
              <ul className="findings-list">
                {findings.map((f, idx) => (
                  <li key={idx}>[{f.severity}] {f.message}</li>
                ))}
              </ul>
            </section>
          )}
          <DiagnosticsPanel
            diagnostics={result?.metadata?.diagnostics}
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

function getTopEventRows(rows: GcEventRow[] | undefined): GcEventRow[] {
  return rows?.slice(0, MAX_EVENT_ROWS) ?? [];
}

function createGcChartLabels(t: (key: MessageKey) => string): GcChartLabels {
  return {
    pauseMs: t("pauseMsAxis"),
    heapMb: t("heapMbAxis"),
    gcType: t("gcTypeLabel"),
    heapBefore: t("heapBeforeMb"),
    heapAfter: t("heapAfterMb"),
    youngAfter: t("youngAfterMb"),
    cause: t("gcCauseLabel"),
    events: t("eventsLabel"),
    mbPerSec: t("mbPerSecAxis"),
    allocationRate: t("allocationRateLabel"),
    promotionRate: t("promotionRateLabel"),
  };
}

function formatMbPerSec(value: number | null | undefined): string {
  return typeof value === "number" ? `${value.toLocaleString()} MB/s` : "-";
}

const emptyGcResult: GcLogAnalysisResult = {
  type: "gc_log",
  source_files: [],
  created_at: "",
  summary: {
    total_events: 0,
    total_pause_ms: 0,
    avg_pause_ms: 0,
    max_pause_ms: 0,
    p50_pause_ms: 0,
    p95_pause_ms: 0,
    p99_pause_ms: 0,
    throughput_percent: 0,
    wall_time_sec: 0,
    young_gc_count: 0,
    full_gc_count: 0,
    avg_allocation_rate_mb_per_sec: 0,
    avg_promotion_rate_mb_per_sec: 0,
    humongous_allocation_count: 0,
    concurrent_mode_failure_count: 0,
    promotion_failure_count: 0,
  },
  series: {
    pause_timeline: [],
    heap_after_mb: [],
    heap_before_mb: [],
    young_after_mb: [],
    pause_histogram: [],
    allocation_rate_mb_per_sec: [],
    promotion_rate_mb_per_sec: [],
    gc_type_breakdown: [],
    cause_breakdown: [],
  },
  tables: { events: [] },
  charts: {},
  metadata: {
    parser: "",
    schema_version: "0.1.0",
    diagnostics: {
      total_lines: 0,
      parsed_records: 0,
      skipped_lines: 0,
      skipped_by_reason: {},
      samples: [],
    },
    findings: [],
  },
};
