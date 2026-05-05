import { Camera, Loader2, Play, Square } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import {
  createAnalyzerRequestId,
  getAnalyzerClient,
  type BridgeError,
  type GcEventRow,
  type GcLogAnalysisResult,
} from "@/api/analyzerClient";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { FileDock, type FileDockSelection } from "@/components/FileDock";
import { MetricCard } from "@/components/MetricCard";
import { D3BarChart, type BarDatum } from "@/components/charts/D3BarChart";
import {
  D3TimelineChart,
  type TimelineEvent,
  type TimelineSelection,
  type TimelineSeries,
} from "@/components/charts/D3TimelineChart";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import { exportChartsInContainer } from "@/lib/batchExport";
import { saveDashboardResult } from "@/pages/DashboardPage";
import {
  formatMilliseconds,
  formatNumber,
  formatPercent,
} from "@/utils/formatters";

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

const MAX_EVENT_ROWS = 200;

export function GcLogAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(null);
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<GcLogAnalysisResult | null>(null);
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const [savingAll, setSavingAll] = useState(false);
  const [selection, setSelection] = useState<TimelineSelection | null>(null);
  const currentRequestIdRef = useRef<string | null>(null);
  const chartsContainerRef = useRef<HTMLDivElement | null>(null);

  const filePath = selectedFile?.filePath ?? "";
  const canAnalyze = Boolean(filePath) && state !== "running";
  const summary = result?.summary;
  const eventRows = result?.tables?.events?.slice(0, MAX_EVENT_ROWS) ?? [];
  const findings = result?.metadata?.findings ?? [];

  const pauseSeries = useMemo<TimelineSeries[]>(() => {
    const series = result?.series.pause_timeline ?? [];
    return [
      {
        id: "pause_ms",
        label: t("pauseMsAxis"),
        color: "#ef4444",
        data: series.map((p) => ({ time: p.time, value: p.value })),
        showPoints: series.length <= 200,
      },
    ];
  }, [result, t]);

  const fullGcEvents = useMemo<TimelineEvent[]>(() => {
    return (result?.tables?.events ?? [])
      .filter((row) => row.gc_type && /full/i.test(row.gc_type) && row.timestamp)
      .map((row) => ({
        time: row.timestamp as string,
        label: `${t("fullGcMarker")} (${row.gc_type})`,
        color: "#f97316",
        payload: {
          cause: row.cause ?? "—",
          pause_ms: row.pause_ms != null ? row.pause_ms.toFixed(2) : "—",
          heap_before_mb: row.heap_before_mb != null ? row.heap_before_mb.toFixed(1) : "—",
          heap_after_mb: row.heap_after_mb != null ? row.heap_after_mb.toFixed(1) : "—",
          heap_committed_mb:
            row.heap_committed_mb != null ? row.heap_committed_mb.toFixed(1) : "—",
        },
      }));
  }, [result, t]);

  const allEvents = result?.tables?.events ?? [];

  // Aggregate stats for the brushed window over the pause timeline series.
  const selectionSummary = useMemo(() => {
    if (!selection || allEvents.length === 0) return null;
    const startMs = selection.start.getTime();
    const endMs = selection.end.getTime();
    const inWindow = allEvents.filter((row) => {
      if (!row.timestamp) return false;
      const ts = new Date(row.timestamp).getTime();
      return Number.isFinite(ts) && ts >= startMs && ts <= endMs;
    });
    if (inWindow.length === 0) return { count: 0, avg: 0, max: 0, p95: 0 };
    const pauses = inWindow
      .map((row) => row.pause_ms ?? 0)
      .filter((value) => Number.isFinite(value)) as number[];
    pauses.sort((a, b) => a - b);
    const sum = pauses.reduce((acc, value) => acc + value, 0);
    const avg = pauses.length > 0 ? sum / pauses.length : 0;
    const max = pauses.length > 0 ? pauses[pauses.length - 1] : 0;
    const p95Index = Math.max(0, Math.ceil(pauses.length * 0.95) - 1);
    const p95 = pauses[p95Index] ?? 0;
    return { count: inWindow.length, avg, max, p95 };
  }, [selection, allEvents]);

  // T-215: aggregate pause statistics per GC collector type from the events table.
  const algorithmRows = useMemo(() => {
    const buckets = new Map<string, number[]>();
    for (const row of allEvents) {
      if (!row.gc_type || row.pause_ms == null || !Number.isFinite(row.pause_ms)) continue;
      const list = buckets.get(row.gc_type) ?? [];
      list.push(row.pause_ms);
      buckets.set(row.gc_type, list);
    }
    return Array.from(buckets.entries())
      .map(([gcType, pauses]) => {
        pauses.sort((a, b) => a - b);
        const sum = pauses.reduce((acc, value) => acc + value, 0);
        const avg = sum / pauses.length;
        const max = pauses[pauses.length - 1];
        const p95Index = Math.max(0, Math.ceil(pauses.length * 0.95) - 1);
        const p95 = pauses[p95Index];
        return {
          gc_type: gcType,
          count: pauses.length,
          avg_ms: avg,
          p95_ms: p95,
          max_ms: max,
          total_ms: sum,
        };
      })
      .sort((a, b) => b.total_ms - a.total_ms);
  }, [allEvents]);

  const algorithmAvgBars = useMemo<BarDatum[]>(
    () =>
      algorithmRows.map((row) => ({
        id: `${row.gc_type}-avg`,
        label: row.gc_type,
        value: Number(row.avg_ms.toFixed(2)),
      })),
    [algorithmRows],
  );

  const algorithmMaxBars = useMemo<BarDatum[]>(
    () =>
      algorithmRows.map((row) => ({
        id: `${row.gc_type}-max`,
        label: row.gc_type,
        value: Number(row.max_ms.toFixed(2)),
      })),
    [algorithmRows],
  );

  type HeapSeriesId =
    | "heap_before"
    | "heap_after"
    | "heap_committed"
    | "young_before"
    | "young_after"
    | "old_before"
    | "old_after"
    | "metaspace_before"
    | "metaspace_after";

  const HEAP_SERIES_DEFS = useMemo<
    Array<{ id: HeapSeriesId; field: string; label: string; color: string; area?: boolean }>
  >(
    () => [
      { id: "heap_committed", field: "heap_committed_mb", label: t("gcHeapCommitted"), color: "#64748b" },
      { id: "heap_before", field: "heap_before_mb", label: t("gcHeapTotalBefore"), color: "#0ea5e9", area: true },
      { id: "heap_after", field: "heap_after_mb", label: t("gcHeapTotalAfter"), color: "#14b8a6" },
      { id: "young_before", field: "young_before_mb", label: t("gcHeapYoungBefore"), color: "#22c55e" },
      { id: "young_after", field: "young_after_mb", label: t("gcHeapYoungAfter"), color: "#84cc16" },
      { id: "old_before", field: "old_before_mb", label: t("gcHeapOldBefore"), color: "#f59e0b" },
      { id: "old_after", field: "old_after_mb", label: t("gcHeapOldAfter"), color: "#a855f7" },
      { id: "metaspace_before", field: "metaspace_before_mb", label: t("gcHeapMetaspaceBefore"), color: "#ec4899" },
      { id: "metaspace_after", field: "metaspace_after_mb", label: t("gcHeapMetaspaceAfter"), color: "#d946ef" },
    ],
    [t],
  );

  const [visibleHeapSeries, setVisibleHeapSeries] = useState<Set<HeapSeriesId>>(
    () => new Set<HeapSeriesId>(["heap_committed", "heap_before", "heap_after"]),
  );
  const [showPauseOverlay, setShowPauseOverlay] = useState(false);

  const heapSeriesAvailability = useMemo<Record<HeapSeriesId, boolean>>(() => {
    const series = (result?.series ?? {}) as Record<string, Array<{ time: string; value: number }> | undefined>;
    const out = {} as Record<HeapSeriesId, boolean>;
    for (const def of HEAP_SERIES_DEFS) {
      out[def.id] = (series[def.field]?.length ?? 0) > 0;
    }
    return out;
  }, [result, HEAP_SERIES_DEFS]);

  const heapSeries = useMemo<TimelineSeries[]>(() => {
    const series = (result?.series ?? {}) as Record<string, Array<{ time: string; value: number }> | undefined>;
    const out: TimelineSeries[] = [];
    for (const def of HEAP_SERIES_DEFS) {
      if (!visibleHeapSeries.has(def.id)) continue;
      const points = series[def.field] ?? [];
      if (points.length === 0) continue;
      out.push({
        id: def.id,
        label: def.label,
        color: def.color,
        area: def.area,
        data: points.map((p) => ({ time: p.time, value: p.value })),
      });
    }
    return out;
  }, [result, visibleHeapSeries, HEAP_SERIES_DEFS]);

  const heapPauseOverlay = useMemo<TimelineSeries[] | undefined>(() => {
    if (!showPauseOverlay) return undefined;
    const points = result?.series.pause_timeline ?? [];
    if (points.length === 0) return undefined;
    return [
      {
        id: "pause_ms_overlay",
        label: t("pauseMsAxis"),
        color: "#ef4444",
        data: points.map((p) => ({ time: p.time, value: p.value })),
      },
    ];
  }, [showPauseOverlay, result, t]);

  const allocationSeries = useMemo<TimelineSeries[]>(() => {
    const alloc = result?.series.allocation_rate_mb_per_sec ?? [];
    const prom = result?.series.promotion_rate_mb_per_sec ?? [];
    return [
      {
        id: "alloc",
        label: t("allocationRateLabel"),
        color: "#6366f1",
        area: true,
        data: alloc.map((p) => ({ time: p.time, value: p.value })),
      },
      {
        id: "prom",
        label: t("promotionRateLabel"),
        color: "#a855f7",
        data: prom.map((p) => ({ time: p.time, value: p.value })),
      },
    ];
  }, [result, t]);

  const typeBars = useMemo<BarDatum[]>(() => {
    return (result?.series.gc_type_breakdown ?? []).map((row) => ({
      id: row.gc_type,
      label: row.gc_type,
      value: row.count,
    }));
  }, [result]);

  const causeBars = useMemo<BarDatum[]>(() => {
    return (result?.series.cause_breakdown ?? []).map((row) => ({
      id: row.cause,
      label: row.cause,
      value: row.count,
    }));
  }, [result]);

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
      const response = await getAnalyzerClient().execute({
        requestId,
        type: "gc_log",
        params: { filePath },
      });

      if (currentRequestIdRef.current !== requestId) return;
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
        prefix: "gc-log",
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
        label={t("selectGcLogFile")}
        description={t("dropOrBrowseGcLog")}
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

      <ErrorPanel
        error={error}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />
      <EngineMessagesPanel messages={engineMessages} title={t("engineMessages")} />

      <Tabs defaultValue="jvm" className="w-full">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <TabsList>
            <TabsTrigger value="jvm">{t("tabJvmInfo")}</TabsTrigger>
            <TabsTrigger value="summary">{t("tabSummary")}</TabsTrigger>
            <TabsTrigger value="pauses">{t("tabPauses")}</TabsTrigger>
            <TabsTrigger value="heap">{t("tabHeap")}</TabsTrigger>
            <TabsTrigger value="algorithm">{t("tabAlgorithmComparison")}</TabsTrigger>
            <TabsTrigger value="breakdown">{t("tabBreakdown")}</TabsTrigger>
            <TabsTrigger value="events">{t("tabEvents")}</TabsTrigger>
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

        <TabsContent value="jvm" className="mt-4">
          <JvmInfoPanel
            info={(result?.metadata as { jvm_info?: JvmInfoData } | undefined)?.jvm_info}
            t={t}
          />
        </TabsContent>

        <TabsContent value="summary" className="mt-4">
          <section className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-4">
            <MetricCard
              label={t("throughputPercent")}
              value={formatPercent(summary?.throughput_percent)}
            />
            <MetricCard label={t("totalEvents")} value={formatNumber(summary?.total_events)} />
            <MetricCard label={t("p50PauseMs")} value={formatMilliseconds(summary?.p50_pause_ms)} />
            <MetricCard label={t("p95PauseMs")} value={formatMilliseconds(summary?.p95_pause_ms)} />
            <MetricCard label={t("p99PauseMs")} value={formatMilliseconds(summary?.p99_pause_ms)} />
            <MetricCard label={t("maxPauseMs")} value={formatMilliseconds(summary?.max_pause_ms)} />
            <MetricCard label={t("avgPauseMs")} value={formatMilliseconds(summary?.avg_pause_ms)} />
            <MetricCard
              label={t("totalPauseMs")}
              value={formatMilliseconds(summary?.total_pause_ms)}
            />
            <MetricCard
              label={t("youngGcCount")}
              value={formatNumber(summary?.young_gc_count)}
            />
            <MetricCard label={t("fullGcCount")} value={formatNumber(summary?.full_gc_count)} />
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
          {findings.length > 0 && (
            <Card className="mt-4">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">{t("findings")}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-1 pt-0 text-sm">
                {findings.map((f, idx) => (
                  <p key={idx} className="leading-relaxed">
                    <span className="mr-2 inline-block rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase">
                      {f.severity}
                    </span>
                    {f.message}
                  </p>
                ))}
              </CardContent>
            </Card>
          )}
        </TabsContent>

        <TabsContent value="pauses" className="mt-4">
          <div ref={chartsContainerRef} className="grid gap-4">
            <D3TimelineChart
              title={t("gcPauseTimeline")}
              exportName="gc-pause-timeline"
              series={pauseSeries}
              events={fullGcEvents}
              yLabel={t("pauseMsAxis")}
              height={300}
              interactive
              onSelectionChange={setSelection}
            />
            {selection && selectionSummary && (
              <Card>
                <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
                  <CardTitle className="text-sm">{t("selectionSummary")}</CardTitle>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() => setSelection(null)}
                  >
                    {t("clearSelection")}
                  </Button>
                </CardHeader>
                <CardContent className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                  <SelectionStat
                    label={t("eventsInSelection")}
                    value={selectionSummary.count.toLocaleString()}
                  />
                  <SelectionStat
                    label={t("avgPauseInSelection")}
                    value={`${selectionSummary.avg.toFixed(2)} ms`}
                  />
                  <SelectionStat
                    label={t("p95PauseInSelection")}
                    value={`${selectionSummary.p95.toFixed(2)} ms`}
                  />
                  <SelectionStat
                    label={t("maxPauseInSelection")}
                    value={`${selectionSummary.max.toFixed(2)} ms`}
                  />
                </CardContent>
              </Card>
            )}
            <D3TimelineChart
              title={t("allocationRateLabel")}
              exportName="gc-allocation-rate"
              series={allocationSeries}
              yLabel="MB/s"
              height={240}
            />
          </div>
        </TabsContent>

        <TabsContent value="heap" className="mt-4">
          <div className="grid gap-4">
            <Card className="p-3">
              <div className="flex flex-col gap-2.5 text-xs">
                <div className="flex items-center justify-between">
                  <span className="font-medium text-foreground/80">{t("gcHeapSeriesPick")}</span>
                  <label className="flex cursor-pointer select-none items-center gap-1.5">
                    <input
                      type="checkbox"
                      className="h-3.5 w-3.5"
                      checked={showPauseOverlay}
                      onChange={(event) => setShowPauseOverlay(event.target.checked)}
                    />
                    <span
                      className="inline-block h-2 w-3 rounded-sm"
                      style={{ background: "#ef4444" }}
                      aria-hidden
                    />
                    <span>{t("gcHeapPauseOverlay")}</span>
                  </label>
                </div>
                <div className="grid grid-cols-2 gap-x-4 gap-y-1.5 sm:grid-cols-3 lg:grid-cols-5">
                  {HEAP_SERIES_DEFS.map((def) => {
                    const enabled = heapSeriesAvailability[def.id];
                    const checked = visibleHeapSeries.has(def.id);
                    return (
                      <label
                        key={def.id}
                        className={`flex min-w-0 items-center gap-1.5 select-none ${
                          enabled ? "cursor-pointer" : "cursor-not-allowed opacity-40"
                        }`}
                        title={enabled ? def.label : `${def.label} (—)`}
                      >
                        <input
                          type="checkbox"
                          className="h-3.5 w-3.5 shrink-0"
                          style={{ accentColor: def.color }}
                          disabled={!enabled}
                          checked={checked && enabled}
                          onChange={(event) => {
                            setVisibleHeapSeries((prev) => {
                              const next = new Set(prev);
                              if (event.target.checked) next.add(def.id);
                              else next.delete(def.id);
                              return next;
                            });
                          }}
                        />
                        <span
                          className="inline-block h-2 w-3 shrink-0 rounded-sm"
                          style={{ background: def.color }}
                          aria-hidden
                        />
                        <span className="truncate">{def.label}</span>
                      </label>
                    );
                  })}
                </div>
              </div>
            </Card>
            <D3TimelineChart
              title={t("gcHeapUsage")}
              exportName="gc-heap-usage"
              series={heapSeries}
              seriesRight={heapPauseOverlay}
              events={fullGcEvents}
              yLabel={t("heapMbAxis")}
              yLabelRight={t("pauseMsAxis")}
              height={320}
              interactive
              onSelectionChange={setSelection}
            />
          </div>
        </TabsContent>

        <TabsContent value="algorithm" className="mt-4">
          <div className="grid gap-4">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">
                  {t("gcAlgorithmComparisonTitle")}
                </CardTitle>
                <p className="text-xs text-muted-foreground">
                  {t("gcAlgorithmComparisonHint")}
                </p>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                      <th className="px-3 py-2 text-left font-medium">{t("gcTypeLabel")}</th>
                      <th className="px-3 py-2 text-right font-medium">count</th>
                      <th className="px-3 py-2 text-right font-medium">avg ms</th>
                      <th className="px-3 py-2 text-right font-medium">p95 ms</th>
                      <th className="px-3 py-2 text-right font-medium">max ms</th>
                      <th className="px-3 py-2 text-right font-medium">total ms</th>
                    </tr>
                  </thead>
                  <tbody>
                    {algorithmRows.length === 0 ? (
                      <tr>
                        <td className="px-3 py-3 text-muted-foreground" colSpan={6}>
                          —
                        </td>
                      </tr>
                    ) : (
                      algorithmRows.map((row) => (
                        <tr
                          key={row.gc_type}
                          className="border-b border-border last:border-0"
                        >
                          <td className="px-3 py-2 font-mono text-xs">{row.gc_type}</td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {row.count}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {row.avg_ms.toFixed(2)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {row.p95_ms.toFixed(2)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {row.max_ms.toFixed(2)}
                          </td>
                          <td className="px-3 py-2 text-right tabular-nums">
                            {row.total_ms.toFixed(2)}
                          </td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </CardContent>
            </Card>
            <div className="grid gap-4 lg:grid-cols-2">
              <D3BarChart
                title="avg pause (ms) by collector"
                exportName="gc-algorithm-avg-pause"
                data={algorithmAvgBars}
                orientation="horizontal"
                sort
                height={Math.max(180, algorithmAvgBars.length * 28 + 40)}
              />
              <D3BarChart
                title="max pause (ms) by collector"
                exportName="gc-algorithm-max-pause"
                data={algorithmMaxBars}
                orientation="horizontal"
                sort
                height={Math.max(180, algorithmMaxBars.length * 28 + 40)}
              />
            </div>
          </div>
        </TabsContent>

        <TabsContent value="breakdown" className="mt-4">
          <div className="grid gap-4 lg:grid-cols-2">
            <D3BarChart
              title={t("collectorCauseBreakdown")}
              exportName="gc-type-breakdown"
              data={typeBars}
              orientation="horizontal"
              sort
              height={Math.max(180, typeBars.length * 28 + 40)}
            />
            <D3BarChart
              title={t("gcCauseLabel")}
              exportName="gc-cause-breakdown"
              data={causeBars}
              orientation="horizontal"
              sort
              height={Math.max(180, causeBars.length * 28 + 40)}
            />
          </div>
        </TabsContent>

        <TabsContent value="events" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("gcEventTable")}</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">{t("timestampLabel")}</th>
                    <th className="px-3 py-2 text-right font-medium">{t("uptimeSec")}</th>
                    <th className="px-3 py-2 text-left font-medium">{t("gcTypeLabel")}</th>
                    <th className="px-3 py-2 text-left font-medium">{t("gcCauseLabel")}</th>
                    <th className="px-3 py-2 text-right font-medium">{t("pauseMsAxis")}</th>
                    <th className="px-3 py-2 text-right font-medium">{t("heapBeforeMb")}</th>
                    <th className="px-3 py-2 text-right font-medium">{t("heapAfterMb")}</th>
                    <th className="px-3 py-2 text-right font-medium">{t("heapCommittedMb")}</th>
                  </tr>
                </thead>
                <tbody>
                  {eventRows.length > 0 ? (
                    eventRows.map((row, idx) => (
                      <EventRowItem key={idx} row={row} />
                    ))
                  ) : (
                    <tr>
                      <td className="px-3 py-3 text-muted-foreground" colSpan={8}>
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
            diagnostics={result?.metadata?.diagnostics}
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

function EventRowItem({ row }: { row: GcEventRow }): JSX.Element {
  return (
    <tr className="border-b border-border last:border-0">
      <td className="px-3 py-2 font-mono text-xs">{row.timestamp ?? "-"}</td>
      <td className="px-3 py-2 text-right tabular-nums">
        {row.uptime_sec != null ? row.uptime_sec.toFixed(3) : "-"}
      </td>
      <td className="px-3 py-2">{row.gc_type ?? "-"}</td>
      <td className="px-3 py-2 text-xs text-muted-foreground">{row.cause ?? "-"}</td>
      <td className="px-3 py-2 text-right tabular-nums">
        {row.pause_ms != null ? row.pause_ms.toFixed(2) : "-"}
      </td>
      <td className="px-3 py-2 text-right tabular-nums">
        {row.heap_before_mb != null ? row.heap_before_mb.toFixed(1) : "-"}
      </td>
      <td className="px-3 py-2 text-right tabular-nums">
        {row.heap_after_mb != null ? row.heap_after_mb.toFixed(1) : "-"}
      </td>
      <td className="px-3 py-2 text-right tabular-nums">
        {row.heap_committed_mb != null ? row.heap_committed_mb.toFixed(1) : "-"}
      </td>
    </tr>
  );
}

function formatMbPerSec(value: number | null | undefined): string {
  return typeof value === "number" ? `${value.toLocaleString()} MB/s` : "-";
}

function SelectionStat({
  label,
  value,
}: {
  label: string;
  value: string;
}): JSX.Element {
  return (
    <div className="rounded-md border border-border bg-muted/30 px-3 py-2">
      <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
        {label}
      </p>
      <p className="mt-0.5 text-sm font-semibold tabular-nums">{value}</p>
    </div>
  );
}

type JvmInfoData = {
  vm_banner?: string;
  vm_version?: string;
  vm_build?: string;
  platform?: string;
  collector?: string;
  cpus_total?: number;
  cpus_available?: number;
  memory_mb?: number;
  page_size_kb?: number;
  heap_min_mb?: number;
  heap_initial_mb?: number;
  heap_max_mb?: number;
  heap_region_size_mb?: number;
  parallel_workers?: number;
  concurrent_workers?: number;
  concurrent_refinement_workers?: number;
  large_pages?: string;
  numa?: string;
  compressed_oops?: string;
  pre_touch?: string;
  periodic_gc?: string;
  command_line?: string;
  raw_lines?: string[];
};

function JvmInfoPanel({
  info,
  t,
}: {
  info: JvmInfoData | undefined;
  t: (key: MessageKey) => string;
}): JSX.Element {
  const [copied, setCopied] = useState(false);

  if (!info || !Object.keys(info).length) {
    return (
      <Card>
        <CardContent className="py-10 text-center text-sm text-muted-foreground">
          {t("jvmInfoEmpty")}
        </CardContent>
      </Card>
    );
  }

  const cpus =
    typeof info.cpus_total === "number"
      ? info.cpus_available != null && info.cpus_available !== info.cpus_total
        ? `${info.cpus_total} (avail ${info.cpus_available})`
        : `${info.cpus_total}`
      : null;

  const workerWarning =
    typeof info.cpus_total === "number" &&
    typeof info.parallel_workers === "number" &&
    info.parallel_workers < info.cpus_total
      ? t("jvmInfoWorkerWarning")
          .replace("{n}", String(info.parallel_workers))
          .replace("{cpus}", String(info.cpus_total))
      : null;

  const runtimeFields: Array<[string, string | null]> = [
    [t("jvmInfoCollector"), info.collector ?? null],
    [t("jvmInfoVersion"), info.vm_version ?? null],
    [t("jvmInfoVmBuild"), info.vm_build ?? null],
    [t("jvmInfoPlatform"), info.platform ?? null],
  ];
  const systemFields: Array<[string, string | null]> = [
    [t("jvmInfoCpus"), cpus],
    [t("jvmInfoMemory"), formatMb(info.memory_mb)],
    [t("jvmInfoPageSize"), info.page_size_kb != null ? `${info.page_size_kb} KB` : null],
  ];
  const heapFields: Array<[string, string | null]> = [
    [t("jvmInfoHeapMin"), formatMb(info.heap_min_mb)],
    [t("jvmInfoHeapInitial"), formatMb(info.heap_initial_mb)],
    [t("jvmInfoHeapMax"), formatMb(info.heap_max_mb)],
    [t("jvmInfoHeapRegion"), formatMb(info.heap_region_size_mb)],
  ];
  const workerFields: Array<[string, string | null]> = [
    [
      t("jvmInfoParallelWorkers"),
      info.parallel_workers != null ? String(info.parallel_workers) : null,
    ],
    [
      t("jvmInfoConcurrentWorkers"),
      info.concurrent_workers != null ? String(info.concurrent_workers) : null,
    ],
    [
      t("jvmInfoConcurrentRefinement"),
      info.concurrent_refinement_workers != null
        ? String(info.concurrent_refinement_workers)
        : null,
    ],
  ];
  const flagFields: Array<[string, string | null]> = [
    [t("jvmInfoLargePages"), info.large_pages ?? null],
    [t("jvmInfoNuma"), info.numa ?? null],
    [t("jvmInfoCompressedOops"), info.compressed_oops ?? null],
    [t("jvmInfoPreTouch"), info.pre_touch ?? null],
    [t("jvmInfoPeriodicGc"), info.periodic_gc ?? null],
  ];

  const handleCopy = async () => {
    if (!info.command_line) return;
    try {
      await navigator.clipboard.writeText(info.command_line);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      /* ignore */
    }
  };

  return (
    <div className="flex flex-col gap-4">
      {workerWarning && (
        <div className="rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs text-amber-900 dark:border-amber-700 dark:bg-amber-950/40 dark:text-amber-200">
          ⚠ {workerWarning}
        </div>
      )}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">{t("jvmInfoTitle")}</CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3">
          <JvmInfoSection title={t("jvmInfoSectionRuntime")} fields={runtimeFields} />
          <JvmInfoSection title={t("jvmInfoSectionSystem")} fields={systemFields} />
          <JvmInfoSection title={t("jvmInfoSectionHeap")} fields={heapFields} />
          <JvmInfoSection title={t("jvmInfoSectionWorkers")} fields={workerFields} />
          <JvmInfoSection title={t("jvmInfoSectionFlags")} fields={flagFields} />
        </CardContent>
      </Card>

      {info.command_line && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
            <CardTitle className="text-sm">{t("jvmInfoSectionCommandLine")}</CardTitle>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              onClick={() => void handleCopy()}
            >
              {copied ? t("jvmInfoCopied") : t("jvmInfoCopyCommandLine")}
            </Button>
          </CardHeader>
          <CardContent>
            <pre className="overflow-auto whitespace-pre-wrap break-all rounded-md border border-border bg-muted/40 p-3 font-mono text-[11px] leading-relaxed">
              {info.command_line.split(/\s+(?=-)/).join("\n")}
            </pre>
          </CardContent>
        </Card>
      )}

      {info.raw_lines && info.raw_lines.length > 0 && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-sm">{t("jvmInfoSectionRaw")}</CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="max-h-72 overflow-auto whitespace-pre rounded-md border border-border bg-muted/40 p-3 font-mono text-[11px] leading-relaxed">
              {info.raw_lines.join("\n")}
            </pre>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function JvmInfoSection({
  title,
  fields,
}: {
  title: string;
  fields: Array<[string, string | null]>;
}): JSX.Element | null {
  const visible = fields.filter(([, v]) => v != null && v !== "");
  if (visible.length === 0) return null;
  return (
    <section>
      <h3 className="mb-1.5 text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
        {title}
      </h3>
      <dl className="space-y-1 text-xs">
        {visible.map(([label, value]) => (
          <div key={label} className="flex items-baseline justify-between gap-3">
            <dt className="shrink-0 text-muted-foreground">{label}</dt>
            <dd className="min-w-0 truncate text-right tabular-nums" title={value ?? ""}>
              {value}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

function formatMb(mb: number | null | undefined): string | null {
  if (mb == null) return null;
  if (mb >= 1024) {
    const gb = mb / 1024;
    return `${gb.toLocaleString(undefined, { maximumFractionDigits: 2 })} GB`;
  }
  return `${mb.toLocaleString(undefined, { maximumFractionDigits: 2 })} MB`;
}
