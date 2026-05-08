import { Loader2, Play, Square } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import { engine } from "@/bridge/engine";
import type {
  BridgeError,
  GcEventRow,
  GcJvmInfo,
  GcLogAnalysisResult,
  JvmFinding,
} from "@/bridge/types";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { ChartPanel } from "@/components/ChartPanel";
import { MetricCard } from "@/components/MetricCard";
import { RecentFilesPanel } from "@/components/RecentFilesPanel";
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
import {
  allocationRateOption,
  causeBreakdownOption,
  gcTypeBreakdownOption,
  heapSeriesAvailability,
  heapSeriesDefs,
  heapUsageOption,
  pauseTimelineOption,
  type GcChartLabels,
  type HeapSeriesId,
} from "@/charts/gcLogCharts";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import {
  formatMilliseconds,
  formatNumber,
  formatPercent,
} from "@/utils/formatters";

// Wails port of apps/frontend/src/pages/GcLogAnalyzerPage.tsx.
//
// Mirrors the Phase-1 AccessLog pattern: file path comes from a Wails
// native dialog (or drop), the engine reads the file off disk, and the
// renderer paints a metric grid + ChartPanel-hosted echarts. The web
// original layers brushing, fullscreen, per-chart PNG export, and
// dashboard persistence on top of D3 — those land alongside the
// Save-All-Charts batch exporter in a later phase.
//
// Tabs (subset of the web page; same vocabulary):
//   summary    — metric grid + findings list
//   pauses     — pause timeline + allocation/promotion rate timeline
//   heap       — heap before/after/committed timeline
//   breakdown  — gc-type and cause horizontal bars
//   events     — first 200 raw events
//   jvm        — banner-derived runtime + flag table
//   diagnostics — parser stats / skip samples

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

const MAX_EVENT_ROWS = 200;

export function GcLogAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(
    null,
  );
  const [maxLines, setMaxLines] = useState("");
  const [topN, setTopN] = useState("");
  const [strict, setStrict] = useState(false);
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<GcLogAnalysisResult | null>(null);
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const inflightRef = useRef<symbol | null>(null);

  const filePath = selectedFile?.filePath ?? "";
  const canAnalyze = Boolean(filePath) && state !== "running";
  const summary = result?.summary;
  const findings = result?.metadata?.findings ?? ([] as JvmFinding[]);
  const eventRows = result?.tables?.events?.slice(0, MAX_EVENT_ROWS) ?? [];
  const jvmInfo = result?.metadata?.jvm_info;

  const chartLabels = useMemo<GcChartLabels>(
    () => ({
      pauseAxis: t("pauseMsAxis"),
      pauseSeries: t("pauseSeriesLabel"),
      fullGcMarker: t("fullGcMarker"),
      heapAxis: t("heapMbAxis"),
      heapBefore: t("gcHeapBefore"),
      heapAfter: t("gcHeapAfter"),
      heapCommitted: t("gcHeapCommitted"),
      heapYoungBefore: t("gcHeapYoungBefore"),
      heapYoungAfter: t("gcHeapYoungAfter"),
      heapOldBefore: t("gcHeapOldBefore"),
      heapOldAfter: t("gcHeapOldAfter"),
      heapMetaspaceBefore: t("gcHeapMetaspaceBefore"),
      heapMetaspaceAfter: t("gcHeapMetaspaceAfter"),
      heapPauseOverlay: t("gcHeapPauseOverlay"),
      rateAxis: t("rateMbPerSecAxis"),
      allocSeries: t("allocationRateLabel"),
      promSeries: t("promotionRateLabel"),
      countAxis: t("count"),
    }),
    [t],
  );

  // Heap series toggles + pause overlay state. Defaults match the
  // web page so users get heap_committed / before / after on first
  // open, then can drill into young / old / metaspace as needed.
  const [visibleHeapSeries, setVisibleHeapSeries] = useState<Set<HeapSeriesId>>(
    () => new Set<HeapSeriesId>(["heap_committed", "heap_before", "heap_after"]),
  );
  const [showPauseOverlay, setShowPauseOverlay] = useState(false);

  const heapAvailability = useMemo(
    () => heapSeriesAvailability(result, chartLabels),
    [chartLabels, result],
  );
  const heapDefs = useMemo(() => heapSeriesDefs(chartLabels), [chartLabels]);

  const pauseOption = useMemo(
    () => pauseTimelineOption(result, chartLabels),
    [chartLabels, result],
  );
  const heapOption = useMemo(
    () => heapUsageOption(result, chartLabels, visibleHeapSeries, showPauseOverlay),
    [chartLabels, result, visibleHeapSeries, showPauseOverlay],
  );
  const allocOption = useMemo(
    () => allocationRateOption(result, chartLabels),
    [chartLabels, result],
  );
  const typeOption = useMemo(
    () => gcTypeBreakdownOption(result, chartLabels),
    [chartLabels, result],
  );
  const causeOption = useMemo(
    () => causeBreakdownOption(result, chartLabels),
    [chartLabels, result],
  );

  // Per-analyzer recent-files history, persisted via localStorage.
  // The "gc" category keeps the GC list separate from profiler/jennifer.
  const recent = useRecentFiles({ category: "gc" });

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

  const handleRecentSelect = useCallback(
    (entry: { path: string; name?: string; meta?: Record<string, unknown> }) => {
      setSelectedFile({
        filePath: entry.path,
        originalName: entry.name ?? entry.path,
      } as FileDockSelection);
      // Restore prior options when the entry remembers them — this is
      // the "fast re-analysis" UX so users don't have to re-enter the
      // same maxLines / topN / strict toggle every time.
      const meta = entry.meta ?? {};
      if (typeof meta.maxLines === "string") setMaxLines(meta.maxLines);
      if (typeof meta.topN === "string") setTopN(meta.topN);
      if (typeof meta.strict === "boolean") setStrict(meta.strict);
      setState("ready");
      setError(null);
      setEngineMessages([]);
    },
    [],
  );

  const analyze = useCallback(async () => {
    if (!canAnalyze) return;
    const parsedMaxLines = parseOptionalPositiveInteger(maxLines);
    const parsedTopN = parseOptionalPositiveInteger(topN);
    if (parsedMaxLines === null || parsedTopN === null) {
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
    const token = Symbol("gc-log-analysis");
    inflightRef.current = token;

    try {
      const response = await engine.analyzeGcLog({
        path: filePath,
        maxLines: parsedMaxLines ?? undefined,
        topN: parsedTopN ?? undefined,
        strict: strict || undefined,
      });

      if (inflightRef.current !== token) return;
      inflightRef.current = null;
      // The shared bridge returns AnalysisResult — narrow it to the
      // GcLog shape now that the engine guarantees `type === "gc_log"`.
      setResult(response as unknown as GcLogAnalysisResult);
      setState("success");
      if (filePath) {
        recent.push({
          path: filePath,
          analyzer: "gc",
          meta: { maxLines, topN, strict },
        });
      }
    } catch (caught) {
      if (inflightRef.current !== token) return;
      inflightRef.current = null;
      const message = caught instanceof Error ? caught.message : String(caught);
      setError({ code: "ENGINE_FAILED", message });
      setState("error");
    }
  }, [canAnalyze, filePath, maxLines, strict, t, topN, recent]);

  const cancelAnalysis = useCallback(() => {
    // GcLog runs synchronously — no engine-side cancel hook. We just
    // discard the in-flight token so the late result is ignored.
    inflightRef.current = null;
    setState("ready");
    setEngineMessages([t("analysisCanceled")]);
  }, [t]);

  return (
    <main className="flex flex-col gap-5 p-5">
      <WailsFileDock
        label={t("selectGcLogFile")}
        description={t("dropOrBrowseGcLog")}
        accept=".log,.txt,.gz"
        selected={selectedFile}
        onSelect={handleFileSelected}
        onClear={handleClearFile}
        browseLabel={t("browseFile")}
        dropHereLabel={t("dropHere")}
        errorLabel={t("error")}
        fileFilters={[
          { displayName: "GC logs", pattern: "*.log;*.txt;*.gz" },
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
              {t("maxLines")}
            </span>
            <Input
              type="number"
              min={1}
              placeholder="100000 (기본: 무제한)"
              value={maxLines}
              onChange={(event) => setMaxLines(event.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("gcLogTopN")}
            </span>
            <Input
              type="number"
              min={1}
              placeholder="20"
              value={topN}
              onChange={(event) => setTopN(event.target.value)}
            />
          </label>
          <label className="flex select-none items-center gap-2 text-xs">
            <input
              type="checkbox"
              className="h-3.5 w-3.5"
              checked={strict}
              onChange={(event) => setStrict(event.target.checked)}
            />
            <span className="font-medium text-foreground/80">
              {t("gcLogStrict")}
            </span>
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
          <TabsTrigger value="pauses">{t("tabPauses")}</TabsTrigger>
          <TabsTrigger value="heap">{t("tabHeap")}</TabsTrigger>
          <TabsTrigger value="breakdown">{t("tabBreakdown")}</TabsTrigger>
          <TabsTrigger value="events">{t("tabEvents")}</TabsTrigger>
          <TabsTrigger value="jvm">{t("tabJvmInfo")}</TabsTrigger>
          <TabsTrigger value="diagnostics">{t("tabDiagnostics")}</TabsTrigger>
        </TabsList>

        <TabsContent value="summary" className="mt-4">
          <section className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
            <MetricCard
              label={t("totalEvents")}
              value={formatNumber(summary?.total_events)}
            />
            <MetricCard
              label={t("throughputPercent")}
              value={formatPercent(summary?.throughput_percent)}
            />
            <MetricCard
              label={t("totalPauseMs")}
              value={formatMilliseconds(summary?.total_pause_ms)}
            />
            <MetricCard
              label={t("avgPauseMs")}
              value={formatMilliseconds(summary?.avg_pause_ms)}
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
          {findings.length > 0 && (
            <Card className="mt-4">
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">{t("findings")}</CardTitle>
              </CardHeader>
              <CardContent className="space-y-1 pt-0 text-sm">
                {findings.map((finding, idx) => (
                  <p
                    key={`${finding.code}-${idx}`}
                    className="leading-relaxed"
                  >
                    <span className="mr-2 inline-block rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase">
                      {finding.severity}
                    </span>
                    {finding.message}
                  </p>
                ))}
              </CardContent>
            </Card>
          )}
        </TabsContent>

        <TabsContent value="pauses" className="mt-4">
          <div className="grid gap-4">
            <ChartPanel
              title={t("gcPauseTimeline")}
              option={pauseOption}
              busy={state === "running"}
              height={320}
            />
            <ChartPanel
              title={t("gcAllocationRate")}
              option={allocOption}
              busy={state === "running"}
              height={260}
            />
          </div>
        </TabsContent>

        <TabsContent value="heap" className="mt-4">
          <Card className="mb-3 p-3">
            <div className="flex flex-col gap-2.5 text-xs">
              <div className="flex items-center justify-between">
                <span className="font-medium text-foreground/80">
                  {t("gcHeapSeriesPick")}
                </span>
                <label className="flex cursor-pointer select-none items-center gap-1.5">
                  <input
                    type="checkbox"
                    className="h-3.5 w-3.5"
                    checked={showPauseOverlay}
                    onChange={(e) => setShowPauseOverlay(e.target.checked)}
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
                {heapDefs.map((def) => {
                  const enabled = heapAvailability[def.id];
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
                        onChange={(e) => {
                          setVisibleHeapSeries((prev) => {
                            const next = new Set(prev);
                            if (e.target.checked) next.add(def.id);
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
          <ChartPanel
            title={t("gcHeapUsage")}
            option={heapOption}
            busy={state === "running"}
            height={400}
          />
        </TabsContent>

        <TabsContent value="breakdown" className="mt-4">
          <div className="grid gap-4 lg:grid-cols-2">
            <ChartPanel
              title={t("gcTypeBreakdownTitle")}
              option={typeOption}
              busy={state === "running"}
              height={Math.max(
                240,
                ((result?.series.gc_type_breakdown?.length ?? 0) + 1) * 28 + 60,
              )}
            />
            <ChartPanel
              title={t("gcCauseBreakdownTitle")}
              option={causeOption}
              busy={state === "running"}
              height={Math.max(
                240,
                ((result?.series.cause_breakdown?.length ?? 0) + 1) * 28 + 60,
              )}
            />
          </div>
        </TabsContent>

        <TabsContent value="events" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("gcEventTable")}</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">
                      {t("timestampLabel")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {t("uptimeSec")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("gcTypeLabel")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("gcCauseLabel")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {t("pauseMsAxis")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {t("heapBeforeMb")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {t("heapAfterMb")}
                    </th>
                    <th className="px-3 py-2 text-right font-medium">
                      {t("heapCommittedMb")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {eventRows.length > 0 ? (
                    eventRows.map((row, idx) => (
                      <EventRowItem key={idx} row={row} />
                    ))
                  ) : (
                    <tr>
                      <td
                        className="px-3 py-3 text-center text-muted-foreground"
                        colSpan={8}
                      >
                        —
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="jvm" className="mt-4">
          <JvmInfoPanel info={jvmInfo} t={t} />
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
    </main>
  );
}

// ──────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────

function parseOptionalPositiveInteger(
  value: string,
): number | undefined | null {
  if (!value.trim()) return undefined;
  const parsed = Number(value);
  if (!Number.isInteger(parsed) || parsed <= 0) return null;
  return parsed;
}

function formatMbPerSec(value: number | null | undefined): string {
  if (typeof value !== "number" || !Number.isFinite(value)) return "-";
  return `${value.toLocaleString(undefined, { maximumFractionDigits: 2 })} MB/s`;
}

function formatMb(mb: number | null | undefined): string | null {
  if (mb == null) return null;
  if (mb >= 1024) {
    const gb = mb / 1024;
    return `${gb.toLocaleString(undefined, { maximumFractionDigits: 2 })} GB`;
  }
  return `${mb.toLocaleString(undefined, { maximumFractionDigits: 2 })} MB`;
}

function EventRowItem({ row }: { row: GcEventRow }): JSX.Element {
  return (
    <tr className="border-b border-border last:border-0">
      <td className="px-3 py-1.5 font-mono text-[11px]">
        {row.timestamp ?? "-"}
      </td>
      <td className="px-3 py-1.5 text-right tabular-nums">
        {row.uptime_sec != null ? row.uptime_sec.toFixed(3) : "-"}
      </td>
      <td className="px-3 py-1.5 font-mono text-[11px]">
        {row.gc_type ?? "-"}
      </td>
      <td className="px-3 py-1.5 text-[11px] text-muted-foreground">
        {row.cause ?? "-"}
      </td>
      <td className="px-3 py-1.5 text-right tabular-nums">
        {row.pause_ms != null ? row.pause_ms.toFixed(2) : "-"}
      </td>
      <td className="px-3 py-1.5 text-right tabular-nums">
        {row.heap_before_mb != null ? row.heap_before_mb.toFixed(1) : "-"}
      </td>
      <td className="px-3 py-1.5 text-right tabular-nums">
        {row.heap_after_mb != null ? row.heap_after_mb.toFixed(1) : "-"}
      </td>
      <td className="px-3 py-1.5 text-right tabular-nums">
        {row.heap_committed_mb != null ? row.heap_committed_mb.toFixed(1) : "-"}
      </td>
    </tr>
  );
}

function JvmInfoPanel({
  info,
  t,
}: {
  info: GcJvmInfo | undefined;
  t: (key: MessageKey) => string;
}): JSX.Element {
  const [copied, setCopied] = useState(false);

  if (!info || Object.keys(info).length === 0) {
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
    [
      t("jvmInfoPageSize"),
      info.page_size_kb != null ? `${info.page_size_kb} KB` : null,
    ],
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
          <JvmInfoSection
            title={t("jvmInfoSectionRuntime")}
            fields={runtimeFields}
          />
          <JvmInfoSection
            title={t("jvmInfoSectionSystem")}
            fields={systemFields}
          />
          <JvmInfoSection
            title={t("jvmInfoSectionHeap")}
            fields={heapFields}
          />
          <JvmInfoSection
            title={t("jvmInfoSectionWorkers")}
            fields={workerFields}
          />
          <JvmInfoSection
            title={t("jvmInfoSectionFlags")}
            fields={flagFields}
          />
        </CardContent>
      </Card>

      {info.command_line && (
        <Card>
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
            <CardTitle className="text-sm">
              {t("jvmInfoSectionCommandLine")}
            </CardTitle>
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
            <CardTitle className="text-sm">
              {t("jvmInfoSectionRaw")}
            </CardTitle>
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
          <div
            key={label}
            className="flex items-baseline justify-between gap-3"
          >
            <dt className="shrink-0 text-muted-foreground">{label}</dt>
            <dd
              className="min-w-0 truncate text-right tabular-nums"
              title={value ?? ""}
            >
              {value}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}
