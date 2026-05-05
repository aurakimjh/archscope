import { Camera, Loader2, Play, Square } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";
import type { EChartsOption } from "echarts";

import {
  createAnalyzerRequestId,
  getAnalyzerClient,
  type BridgeError,
  type ExecutionBreakdownRow,
  type FlameNode,
  type ProfilerCollapsedAnalysisResult,
  type ProfilerTimelineScope,
  type ProfilerTopStackTableRow,
  type TimelineAnalysisRow,
} from "@/api/analyzerClient";
import type { ProfileFormat, ProfileKind } from "@/api/analyzerContract";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { ChartPanel } from "@/components/ChartPanel";
import { FileDock, type FileDockSelection } from "@/components/FileDock";
import { MetricCard } from "@/components/MetricCard";
import { CanvasFlameGraph } from "@/components/charts/CanvasFlameGraph";
import {
  D3FlameGraph,
  type FlameGraphNode,
} from "@/components/charts/D3FlameGraph";
import { FlameTreeTable } from "@/components/charts/FlameTreeTable";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import { exportChartsInContainer } from "@/lib/batchExport";
import {
  formatMilliseconds,
  formatNumber,
  formatPercent,
  formatSeconds,
} from "@/utils/formatters";

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";
type FilterType = "include_text" | "exclude_text" | "regex_include" | "regex_exclude";
type MatchMode = "anywhere" | "ordered" | "subtree";
type ViewMode = "preserve_full_path" | "reroot_at_match";

type DrilldownStageView = {
  label: string;
  breadcrumb: string[];
  flamegraph: FlameNode;
  metrics: {
    matched_samples: number;
    estimated_seconds: number;
    total_ratio: number;
    parent_stage_ratio: number;
    elapsed_ratio: number | null;
  };
  breakdown: ExecutionBreakdownRow[];
};

type TimelineAccumulator = {
  segment: string;
  label: string;
  samples: number;
  methods: Map<string, number>;
  chains: Map<string, number>;
  stacks: Map<string, number>;
};

type TimelineAnalysisView = {
  rows: TimelineAnalysisRow[];
  scope: ProfilerTimelineScope | null;
};

const TIMELINE_ORDER = [
  "STARTUP_FRAMEWORK",
  "INTERNAL_METHOD",
  "SQL_EXECUTION",
  "DB_NETWORK_WAIT",
  "EXTERNAL_CALL",
  "EXTERNAL_NETWORK_WAIT",
  "CONNECTION_POOL_WAIT",
  "LOCK_SYNCHRONIZATION_WAIT",
  "NETWORK_IO_WAIT",
  "FILE_IO",
  "JVM_GC_RUNTIME",
  "UNKNOWN",
] as const;

export function ProfilerAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(null);
  const [wallIntervalMs, setWallIntervalMs] = useState(100);
  const [elapsedSec, setElapsedSec] = useState("");
  const [topN, setTopN] = useState(20);
  const [profileKind, setProfileKind] = useState<ProfileKind>("wall");
  const [profileFormat, setProfileFormat] = useState<ProfileFormat>("collapsed");
  const [timelineBaseMethod, setTimelineBaseMethod] = useState("");
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<ProfilerCollapsedAnalysisResult | null>(null);
  const [stages, setStages] = useState<DrilldownStageView[]>([]);
  const [filterPattern, setFilterPattern] = useState("");
  const [filterType, setFilterType] = useState<FilterType>("include_text");
  const [matchMode, setMatchMode] = useState<MatchMode>("anywhere");
  const [viewMode, setViewMode] = useState<ViewMode>("preserve_full_path");
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const [savingAll, setSavingAll] = useState(false);

  // Display toolbar (shared between flame and diff tabs).
  const [recentFiles, setRecentFiles] = useState<RecentFileEntry[]>(() =>
    loadRecentFiles(),
  );
  const recordRecentFile = useCallback(
    (file: FileDockSelection | null, format: ProfileFormat) => {
      if (!file?.filePath) return;
      setRecentFiles((prev) => {
        const next: RecentFileEntry[] = [
          {
            path: file.filePath!,
            originalName: file.originalName ?? file.filePath!,
            size: file.size ?? 0,
            format,
            addedAt: Date.now(),
          },
          ...prev.filter((entry) => entry.path !== file.filePath),
        ].slice(0, 10);
        saveRecentFiles(next);
        return next;
      });
    },
    [],
  );

  const [highlightPattern, setHighlightPattern] = useState("");
  const [simplifyNames, setSimplifyNames] = useState(false);
  const [normalizeLambdas, setNormalizeLambdas] = useState(false);
  const [dottedNames, setDottedNames] = useState(false);
  const [inverted, setInverted] = useState(false);
  const [minWidthPercent, setMinWidthPercent] = useState(0);
  const displayOptions = useMemo(
    () => ({ simplifyNames, normalizeLambdas, dottedNames }),
    [simplifyNames, normalizeLambdas, dottedNames],
  );

  // Differential profiler state.
  const [baselineFile, setBaselineFile] = useState<FileDockSelection | null>(null);
  const [targetFile, setTargetFile] = useState<FileDockSelection | null>(null);
  const [baselineFormat, setBaselineFormat] = useState<ProfileFormat>("collapsed");
  const [targetFormat, setTargetFormat] = useState<ProfileFormat>("collapsed");
  const [normalize, setNormalize] = useState(true);
  const [diffResult, setDiffResult] = useState<{
    flame: FlameGraphNode | null;
    summary: Record<string, unknown>;
    biggestIncreases: Array<Record<string, unknown>>;
    biggestDecreases: Array<Record<string, unknown>>;
  } | null>(null);
  const [diffState, setDiffState] = useState<AnalyzerState>("idle");
  const [diffError, setDiffError] = useState<BridgeError | null>(null);

  const currentRequestIdRef = useRef<string | null>(null);
  const chartsContainerRef = useRef<HTMLDivElement | null>(null);

  const wallPath = selectedFile?.filePath ?? "";
  const canAnalyze = Boolean(wallPath) && wallIntervalMs > 0 && state !== "running";
  const summary = result?.summary;
  const topStacks = result?.tables.top_stacks ?? [];
  const currentStage = stages[stages.length - 1];
  const breakdownRows = currentStage?.breakdown ?? result?.series.execution_breakdown ?? [];
  const parsedElapsedForDerived = useMemo(
    () => parseOptionalPositiveNumber(elapsedSec),
    [elapsedSec],
  );
  const timelineAnalysis = useMemo<TimelineAnalysisView>(() => {
    if (currentStage) {
      return buildTimeline(
        currentStage.flamegraph,
        wallIntervalMs,
        parsedElapsedForDerived ?? undefined,
        timelineBaseMethod,
        rootSamples(currentStage),
      );
    }
    return {
      rows: result?.series.timeline_analysis ?? [],
      scope: result?.metadata.timeline_scope ?? null,
    };
  }, [
    currentStage,
    parsedElapsedForDerived,
    result?.metadata.timeline_scope,
    result?.series.timeline_analysis,
    timelineBaseMethod,
    wallIntervalMs,
  ]);
  const timelineRows = timelineAnalysis.rows;
  const timelineScope = timelineAnalysis.scope;
  const [selectedThread, setSelectedThread] = useState<string | null>(null);
  const detectedThreads = (result?.series as { threads?: Array<{ name: string; samples: number; ratio: number }> } | undefined)?.threads ?? [];

  const flameRoot = useMemo(() => {
    if (!currentStage) return null;
    const full = toFlameGraphNode(currentStage.flamegraph);
    if (!selectedThread) return full;
    const target = `[${selectedThread}]`;
    const child = (full.children ?? []).find((c) => c.name === target);
    if (!child) return full;
    // Reroot at the chosen thread so the flame shows just its stacks. The
    // sample/ratio math still works since FlameGraphNode treats `value` as
    // total of leaves under the displayed root.
    return {
      ...full,
      name: target,
      value: child.value,
      children: child.children ?? null,
    } as FlameGraphNode;
  }, [currentStage, selectedThread]);
  const flameNodeCount = useMemo(
    () => (flameRoot ? countFlameNodes(flameRoot) : 0),
    [flameRoot],
  );
  // T-217: SVG handles small-to-medium trees with crisp text; switch to
  // the Canvas painter when the tree gets large enough that hit-testing
  // and text layout in SVG starts to drag.
  const useCanvasFlame = flameNodeCount >= 4000;
  const timelineLabel = useCallback(
    (segment: string, fallback?: string) => timelineSegmentLabel(segment, t, fallback),
    [t],
  );

  const fileLabel = useMemo(() => {
    if (profileFormat === "jennifer_csv") return t("selectJenniferCsvFile");
    if (profileFormat === "flamegraph_svg") return t("profileFormatFlamegraphSvg");
    if (profileFormat === "flamegraph_html") return t("profileFormatFlamegraphHtml");
    if (profileKind === "cpu") return t("selectCpuCollapsedFile");
    if (profileKind === "lock") return t("selectLockCollapsedFile");
    return t("selectWallCollapsedFile");
  }, [profileFormat, profileKind, t]);

  const fileAccept = useMemo(() => {
    if (profileFormat === "jennifer_csv") return ".csv";
    if (profileFormat === "flamegraph_svg") return ".svg";
    if (profileFormat === "flamegraph_html") return ".html,.htm";
    return ".collapsed,.txt";
  }, [profileFormat]);

  const handleFileSelected = useCallback((file: FileDockSelection) => {
    setSelectedFile(file);
    setResult(null);
    setStages([]);
    setState("ready");
    setError(null);
    setEngineMessages([]);
  }, []);

  const handleClearFile = useCallback(() => {
    setSelectedFile(null);
    setResult(null);
    setStages([]);
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
      const parsedElapsedSec = parseOptionalPositiveNumber(elapsedSec);
      if (parsedElapsedSec === null || !Number.isFinite(topN) || topN <= 0) {
        setError({
          code: "INVALID_OPTION",
          message: t("invalidAnalyzerOptions"),
        });
        setState("error");
        return;
      }

      const response = await getAnalyzerClient().analyzeCollapsedProfile({
        requestId,
        wallPath,
        wallIntervalMs,
        elapsedSec: parsedElapsedSec,
        topN,
        profileKind,
        profileFormat,
        timelineBaseMethod: timelineBaseMethod.trim() || undefined,
      });

      if (currentRequestIdRef.current !== requestId) return;
      currentRequestIdRef.current = null;

      if (response.ok) {
        setResult(response.result);
        setStages(createInitialStages(response.result));
        setEngineMessages(response.engine_messages ?? []);
        setState("success");
        recordRecentFile(selectedFile, profileFormat);
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

  function applyFilter(): void {
    const pattern = filterPattern.trim();
    if (!pattern || !currentStage) return;
    const next = applyStageFilter(currentStage, {
      pattern,
      filterType,
      matchMode,
      viewMode,
      intervalMs: wallIntervalMs,
      elapsedSec: parseOptionalPositiveNumber(elapsedSec) ?? undefined,
    });
    setStages([...stages, next]);
    setFilterPattern("");
  }

  function resetDrilldown(): void {
    if (result) setStages(createInitialStages(result));
  }

  const [exportingPprof, setExportingPprof] = useState(false);

  async function exportPprof(): Promise<void> {
    if (!selectedFile?.filePath || exportingPprof) return;
    setExportingPprof(true);
    try {
      const baseUrl = (window as { archscope?: { engineUrl?: string } }).archscope?.engineUrl ?? "";
      const url = `${baseUrl.replace(/\/$/, "")}/api/profiler/export-pprof`;
      const response = await fetch(url, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ filePath: selectedFile.filePath, format: profileFormat }),
      });
      if (!response.ok) throw new Error(`export failed: ${response.status}`);
      const blob = await response.blob();
      const downloadName =
        (selectedFile.originalName ?? "profile").replace(/\.[^.]+$/, "") + ".pb.gz";
      const a = document.createElement("a");
      a.href = URL.createObjectURL(blob);
      a.download = downloadName;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(a.href);
    } catch (caught) {
      setError({
        code: "EXPORT_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
    } finally {
      setExportingPprof(false);
    }
  }

  async function runDiff(): Promise<void> {
    const baselinePath = baselineFile?.filePath ?? "";
    const targetPath = targetFile?.filePath ?? "";
    if (!baselinePath || !targetPath) return;
    setDiffState("running");
    setDiffError(null);
    const requestId = createAnalyzerRequestId();
    try {
      const response = await getAnalyzerClient().execute({
        requestId,
        type: "profiler_diff",
        params: {
          requestId,
          baselinePath,
          targetPath,
          baselineFormat,
          targetFormat,
          normalize,
        },
      });
      if (response.ok) {
        const r = response.result as { charts?: { flamegraph?: unknown }; summary: Record<string, unknown>; tables: Record<string, unknown> };
        const flame = (r.charts?.flamegraph as FlameNode | undefined) ?? null;
        setDiffResult({
          flame: flame ? toDiffFlameGraphNode(flame) : null,
          summary: (r.summary as Record<string, unknown>) ?? {},
          biggestIncreases: ((r.tables?.biggest_increases as Array<Record<string, unknown>>) ?? []),
          biggestDecreases: ((r.tables?.biggest_decreases as Array<Record<string, unknown>>) ?? []),
        });
        setDiffState("success");
      } else {
        setDiffError(response.error);
        setDiffState("error");
      }
    } catch (caught) {
      setDiffError({
        code: "IPC_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
      setDiffState("error");
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
        prefix: "profiler",
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
        label={fileLabel}
        description={t("dropOrBrowseProfiler")}
        accept={fileAccept}
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
        <CardContent className="grid grid-cols-1 gap-3 md:grid-cols-3 xl:grid-cols-6">
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("profileFormat")}</span>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={profileFormat}
              onChange={(event) => setProfileFormat(event.target.value as ProfileFormat)}
            >
              <option value="collapsed">{t("profileFormatCollapsed")}</option>
              <option value="jennifer_csv">{t("profileFormatJenniferCsv")}</option>
              <option value="flamegraph_svg">{t("profileFormatFlamegraphSvg")}</option>
              <option value="flamegraph_html">{t("profileFormatFlamegraphHtml")}</option>
            </select>
          </label>
          {profileFormat === "collapsed" && (
            <label className="flex flex-col gap-1.5 text-xs">
              <span className="font-medium text-foreground/80">{t("profileKind")}</span>
              <select
                className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
                value={profileKind}
                onChange={(event) => setProfileKind(event.target.value as ProfileKind)}
              >
                <option value="wall">{t("profileKindWall")}</option>
                <option value="cpu">{t("profileKindCpu")}</option>
                <option value="lock">{t("profileKindLock")}</option>
              </select>
            </label>
          )}
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {profileKind === "cpu"
                ? t("cpuIntervalMs")
                : profileKind === "lock"
                ? t("lockIntervalMs")
                : t("wallIntervalMs")}
            </span>
            <Input
              type="number"
              min={1}
              value={wallIntervalMs}
              onChange={(event) => setWallIntervalMs(Number(event.target.value))}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("elapsedSeconds")}</span>
            <Input
              type="number"
              min={0}
              step="0.001"
              placeholder="1336.559"
              value={elapsedSec}
              onChange={(event) => setElapsedSec(event.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("topN")}</span>
            <Input
              type="number"
              min={1}
              value={topN}
              onChange={(event) => setTopN(Number(event.target.value))}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs md:col-span-2 xl:col-span-2">
            <span className="font-medium text-foreground/80">
              {t("timelineBaseMethod")}
            </span>
            <Input
              type="text"
              placeholder={t("timelineBaseMethodPlaceholder")}
              value={timelineBaseMethod}
              onChange={(event) => setTimelineBaseMethod(event.target.value)}
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
            <TabsTrigger value="flame">{t("tabFlame")}</TabsTrigger>
            <TabsTrigger value="tree">{t("tabProfilerTree")}</TabsTrigger>
            <TabsTrigger value="diff">{t("tabProfilerDiff")}</TabsTrigger>
            <TabsTrigger value="drilldown">{t("tabDrilldown")}</TabsTrigger>
            <TabsTrigger value="timeline">{t("tabTimelineAnalysis")}</TabsTrigger>
            <TabsTrigger value="breakdown">{t("tabBreakdown")}</TabsTrigger>
            <TabsTrigger value="top-stacks">{t("tabTopStacks")}</TabsTrigger>
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
              label={
                profileKind === "cpu"
                  ? t("totalCpuSamples")
                  : profileKind === "lock"
                  ? t("totalLockSamples")
                  : t("totalWallSamples")
              }
              value={formatNumber(summary?.total_samples)}
            />
            <MetricCard
              label={
                profileKind === "cpu"
                  ? t("cpuIntervalMs")
                  : profileKind === "lock"
                  ? t("lockIntervalMs")
                  : t("wallIntervalMs")
              }
              value={formatMilliseconds(summary?.interval_ms)}
            />
            <MetricCard
              label={
                profileKind === "cpu"
                  ? t("estimatedCpuTime")
                  : profileKind === "lock"
                  ? t("estimatedLockTime")
                  : t("estimatedWallTime")
              }
              value={formatSeconds(summary?.estimated_seconds)}
            />
            <MetricCard
              label={t("elapsedSeconds")}
              value={formatSeconds(summary?.elapsed_seconds)}
            />
          </section>
          {currentStage && (
            <section className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <MetricCard
                label={t("matchedSamples")}
                value={formatNumber(currentStage.metrics.matched_samples)}
              />
              <MetricCard
                label={t("estimatedSeconds")}
                value={formatSeconds(currentStage.metrics.estimated_seconds)}
              />
              <MetricCard
                label={t("totalRatio")}
                value={formatPercent(currentStage.metrics.total_ratio)}
              />
              <MetricCard
                label={t("parentStageRatio")}
                value={formatPercent(currentStage.metrics.parent_stage_ratio)}
              />
            </section>
          )}
        </TabsContent>

        <TabsContent value="flame" className="mt-4">
          <div ref={chartsContainerRef} className="grid gap-4">
            {detectedThreads.length > 0 ? (
              <ThreadFilter
                t={t}
                threads={detectedThreads}
                selected={selectedThread}
                onChange={setSelectedThread}
              />
            ) : null}
            <div className="flex justify-end">
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={!flameRoot || exportingPprof || !selectedFile?.filePath}
                onClick={() => void exportPprof()}
              >
                {exportingPprof ? (
                  <>
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    {t("flameExportPprofBusy")}
                  </>
                ) : (
                  t("flameExportPprof")
                )}
              </Button>
            </div>
            <FlameDisplayToolbar
              t={t}
              highlightPattern={highlightPattern}
              onHighlightChange={setHighlightPattern}
              simplifyNames={simplifyNames}
              onSimplifyChange={setSimplifyNames}
              normalizeLambdas={normalizeLambdas}
              onNormalizeLambdasChange={setNormalizeLambdas}
              dottedNames={dottedNames}
              onDottedChange={setDottedNames}
              inverted={inverted}
              onInvertedChange={setInverted}
              minWidthPercent={minWidthPercent}
              onMinWidthChange={setMinWidthPercent}
            />
            {useCanvasFlame ? (
              <CanvasFlameGraph
                title={t("flameGraph")}
                exportName="profiler-flamegraph"
                data={flameRoot}
                emptyLabel={t("flameGraphEmpty")}
              />
            ) : (
              <D3FlameGraph
                title={t("flameGraph")}
                exportName="profiler-flamegraph"
                data={flameRoot}
                emptyLabel={t("flameGraphEmpty")}
                highlightPattern={highlightPattern || undefined}
                displayOptions={displayOptions}
                inverted={inverted}
                minWidthPercent={minWidthPercent}
              />
            )}
          </div>
        </TabsContent>

        <TabsContent value="tree" className="mt-4">
          <div className="grid gap-4">
            {detectedThreads.length > 0 ? (
              <ThreadFilter
                t={t}
                threads={detectedThreads}
                selected={selectedThread}
                onChange={setSelectedThread}
              />
            ) : null}
            <FlameDisplayToolbar
              t={t}
              highlightPattern={highlightPattern}
              onHighlightChange={setHighlightPattern}
              simplifyNames={simplifyNames}
              onSimplifyChange={setSimplifyNames}
              normalizeLambdas={normalizeLambdas}
              onNormalizeLambdasChange={setNormalizeLambdas}
              dottedNames={dottedNames}
              onDottedChange={setDottedNames}
              inverted={inverted}
              onInvertedChange={setInverted}
              minWidthPercent={minWidthPercent}
              onMinWidthChange={setMinWidthPercent}
            />
            <p className="text-xs text-muted-foreground">{t("profilerTreeHint")}</p>
            <FlameTreeTable
              data={flameRoot}
              emptyLabel={t("flameGraphEmpty")}
              highlightPattern={highlightPattern || undefined}
              displayOptions={displayOptions}
              labels={{
                frame: t("profilerTreeFrame"),
                samples: t("profilerTreeSamples"),
                self: t("profilerTreeSelf"),
                ratio: t("profilerTreeRatio"),
              }}
            />
          </div>
        </TabsContent>

        <TabsContent value="diff" className="mt-4">
          <ProfilerDiffSection
            t={t}
            baselineFile={baselineFile}
            onBaselineSelected={setBaselineFile}
            onBaselineCleared={() => setBaselineFile(null)}
            baselineFormat={baselineFormat}
            onBaselineFormatChange={setBaselineFormat}
            targetFile={targetFile}
            onTargetSelected={setTargetFile}
            onTargetCleared={() => setTargetFile(null)}
            targetFormat={targetFormat}
            onTargetFormatChange={setTargetFormat}
            normalize={normalize}
            onNormalizeChange={setNormalize}
            diffState={diffState}
            diffError={diffError}
            diffResult={diffResult}
            onRun={() => void runDiff()}
            recentFiles={recentFiles}
            onClearRecent={() => {
              clearRecentFiles();
              setRecentFiles([]);
            }}
            onPickBaseline={(entry) => {
              setBaselineFile({
                filePath: entry.path,
                originalName: entry.originalName,
                size: entry.size,
              });
              setBaselineFormat(entry.format);
            }}
            onPickTarget={(entry) => {
              setTargetFile({
                filePath: entry.path,
                originalName: entry.originalName,
                size: entry.size,
              });
              setTargetFormat(entry.format);
            }}
            highlightPattern={highlightPattern}
            displayOptions={displayOptions}
            onHighlightChange={setHighlightPattern}
            simplifyNames={simplifyNames}
            onSimplifyChange={setSimplifyNames}
            normalizeLambdas={normalizeLambdas}
            onNormalizeLambdasChange={setNormalizeLambdas}
            dottedNames={dottedNames}
            onDottedChange={setDottedNames}
            inverted={inverted}
            onInvertedChange={setInverted}
            minWidthPercent={minWidthPercent}
            onMinWidthChange={setMinWidthPercent}
          />
        </TabsContent>

        <TabsContent value="drilldown" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("flamegraphDrilldown")}</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {currentStage && currentStage.breadcrumb.length > 0 && (
                <div className="flex flex-wrap items-center gap-1 rounded-md border border-border bg-muted/30 px-3 py-2 text-xs">
                  <span className="text-muted-foreground">
                    {t("drilldownStageBreadcrumb")}:
                  </span>
                  {currentStage.breadcrumb.map((item, index) => (
                    <span key={`${item}-${index}`} className="rounded bg-background px-1.5 py-0.5 font-mono">
                      {item}
                    </span>
                  ))}
                </div>
              )}
              <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
                <label className="flex flex-col gap-1.5 text-xs">
                  <span className="font-medium text-foreground/80">{t("drilldownFilter")}</span>
                  <Input
                    type="text"
                    placeholder="oracle.jdbc"
                    value={filterPattern}
                    onChange={(event) => setFilterPattern(event.target.value)}
                  />
                </label>
                <label className="flex flex-col gap-1.5 text-xs">
                  <span className="font-medium text-foreground/80">{t("filterType")}</span>
                  <select
                    className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
                    value={filterType}
                    onChange={(event) => setFilterType(event.target.value as FilterType)}
                  >
                    <option value="include_text">include text</option>
                    <option value="exclude_text">exclude text</option>
                    <option value="regex_include">regex include</option>
                    <option value="regex_exclude">regex exclude</option>
                  </select>
                </label>
                <label className="flex flex-col gap-1.5 text-xs">
                  <span className="font-medium text-foreground/80">{t("matchMode")}</span>
                  <select
                    className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
                    value={matchMode}
                    onChange={(event) => setMatchMode(event.target.value as MatchMode)}
                  >
                    <option value="anywhere">anywhere</option>
                    <option value="ordered">ordered</option>
                    <option value="subtree">subtree</option>
                  </select>
                </label>
                <label className="flex flex-col gap-1.5 text-xs">
                  <span className="font-medium text-foreground/80">{t("viewMode")}</span>
                  <select
                    className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
                    value={viewMode}
                    onChange={(event) => setViewMode(event.target.value as ViewMode)}
                  >
                    <option value="preserve_full_path">preserve full path</option>
                    <option value="reroot_at_match">re-root at matched frame</option>
                  </select>
                </label>
              </div>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  size="sm"
                  disabled={!currentStage || !filterPattern.trim()}
                  onClick={applyFilter}
                >
                  {t("applyFilter")}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  disabled={stages.length <= 1}
                  onClick={resetDrilldown}
                >
                  {t("resetDrilldown")}
                </Button>
              </div>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="breakdown" className="mt-4">
          <div className="grid gap-4 lg:grid-cols-2">
            <ChartPanel
              title={t("breakdownDonut")}
              option={breakdownDonutOption(breakdownRows)}
            />
            <ChartPanel
              title={t("breakdownBars")}
              option={breakdownBarOption(breakdownRows)}
            />
          </div>
          <div className="mt-4">
            <BreakdownTable
              rows={breakdownRows}
              labels={{
                title: t("categoryTopStacks"),
                category: t("category"),
                samples: t("samples"),
                ratio: t("ratio"),
                waitReason: t("waitReason"),
                topMethods: t("topMethods"),
              }}
            />
          </div>
        </TabsContent>

        <TabsContent value="timeline" className="mt-4">
          <div className="grid gap-4">
            {timelineScope?.mode === "base_method" ? (
              <TimelineScopeCard scope={timelineScope} t={t} />
            ) : null}
            <ChartPanel
              title={t("timelineCompositionChart")}
              option={timelineCompositionOption(timelineRows, timelineLabel)}
            />
            <TimelineTable
              rows={timelineRows}
              labelForSegment={timelineLabel}
              labels={{
                title: t("timelineEvidenceTable"),
                segment: t("timelineSegment"),
                samples: t("samples"),
                estimatedSeconds: t("estimatedSeconds"),
                ratio: t("ratio"),
                methodChain: t("timelineMethodChain"),
                topMethods: t("topMethods"),
              }}
            />
          </div>
        </TabsContent>

        <TabsContent value="top-stacks" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("topStackTable")}</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                    <th className="px-4 py-2 text-left font-medium">{t("stack")}</th>
                    <th className="px-4 py-2 text-right font-medium">{t("samples")}</th>
                    <th className="px-4 py-2 text-right font-medium">
                      {t("estimatedSeconds")}
                    </th>
                    <th className="px-4 py-2 text-right font-medium">{t("ratio")}</th>
                  </tr>
                </thead>
                <tbody>
                  {topStacks.length > 0 ? (
                    topStacks.map((row: ProfilerTopStackTableRow) => (
                      <tr key={row.stack} className="border-b border-border last:border-0">
                        <td className="px-4 py-2 font-mono text-xs">{row.stack}</td>
                        <td className="px-4 py-2 text-right tabular-nums">
                          {row.samples.toLocaleString()}
                        </td>
                        <td className="px-4 py-2 text-right tabular-nums">
                          {formatSeconds(row.estimated_seconds)}
                        </td>
                        <td className="px-4 py-2 text-right tabular-nums">
                          {formatPercent(row.sample_ratio)}
                        </td>
                      </tr>
                    ))
                  ) : (
                    <tr>
                      <td className="px-4 py-3 text-muted-foreground" colSpan={4}>
                        {t("waitingProfilerResult")}
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

function toFlameGraphNode(node: FlameNode): FlameGraphNode {
  return {
    name: node.name,
    value: node.samples,
    category: node.category,
    color: node.color,
    children: node.children?.length ? node.children.map(toFlameGraphNode) : null,
  };
}

/** Like ``toFlameGraphNode`` but preserves the per-node metadata (a/b/delta)
 *  that the diff analyzer attaches. */
function toDiffFlameGraphNode(node: FlameNode | (FlameNode & { metadata?: Record<string, unknown> })): FlameGraphNode {
  const metadata = (node as { metadata?: Record<string, unknown> }).metadata;
  return {
    name: node.name,
    value: node.samples,
    category: node.category,
    color: node.color,
    metadata: metadata ?? null,
    children: node.children?.length ? node.children.map(toDiffFlameGraphNode) : null,
  };
}

function countFlameNodes(node: FlameGraphNode): number {
  let count = 1;
  if (node.children) {
    for (const child of node.children) count += countFlameNodes(child);
  }
  return count;
}

function parseOptionalPositiveNumber(value: string): number | undefined | null {
  if (!value.trim()) return undefined;
  const parsed = Number(value);
  if (!Number.isFinite(parsed) || parsed < 0) return null;
  return parsed;
}

function createInitialStages(
  result: ProfilerCollapsedAnalysisResult,
): DrilldownStageView[] {
  const flamegraph = result.charts.flamegraph;
  if (!flamegraph) return [];
  return [
    {
      label: "All",
      breadcrumb: ["All"],
      flamegraph,
      metrics: {
        matched_samples: flamegraph.samples,
        estimated_seconds: result.summary.estimated_seconds,
        total_ratio: 100,
        parent_stage_ratio: 100,
        elapsed_ratio: result.summary.elapsed_seconds
          ? (result.summary.estimated_seconds / result.summary.elapsed_seconds) * 100
          : null,
      },
      breakdown: result.series.execution_breakdown ?? [],
    },
  ];
}

function applyStageFilter(
  stage: DrilldownStageView,
  options: {
    pattern: string;
    filterType: FilterType;
    matchMode: MatchMode;
    viewMode: ViewMode;
    intervalMs: number;
    elapsedSec?: number;
  },
): DrilldownStageView {
  const leaves = extractLeaves(stage.flamegraph);
  const filtered = leaves.flatMap((leaf) => {
    const matchIndex = matchedIndex(leaf.path, options.pattern, options.filterType, options.matchMode);
    const matched = matchIndex >= 0;
    const include = options.filterType.includes("exclude") ? !matched : matched;
    if (!include) return [];
    const path =
      options.viewMode === "reroot_at_match" &&
      matchIndex >= 0 &&
      !options.filterType.includes("exclude")
        ? leaf.path.slice(matchIndex)
        : leaf.path;
    return [{ path, samples: leaf.samples }];
  });
  const flamegraph = buildTree(filtered, options.pattern);
  const estimatedSeconds = flamegraph.samples * (options.intervalMs / 1000);
  return {
    label: options.pattern,
    breadcrumb: [...stage.breadcrumb, options.pattern],
    flamegraph,
    metrics: {
      matched_samples: flamegraph.samples,
      estimated_seconds: estimatedSeconds,
      total_ratio:
        stage.breadcrumb.length === 1
          ? flamegraph.ratio
          : (flamegraph.samples / Math.max(rootSamples(stage), 1)) * 100,
      parent_stage_ratio:
        (flamegraph.samples / Math.max(stage.flamegraph.samples, 1)) * 100,
      elapsed_ratio: options.elapsedSec
        ? (estimatedSeconds / options.elapsedSec) * 100
        : null,
    },
    breakdown: buildBreakdown(flamegraph, options.intervalMs, options.elapsedSec),
  };
}

function BreakdownTable({
  rows,
  labels,
}: {
  rows: ExecutionBreakdownRow[];
  labels: {
    title: string;
    category: string;
    samples: string;
    ratio: string;
    waitReason: string;
    topMethods: string;
  };
}): JSX.Element {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{labels.title}</CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
              <th className="px-4 py-2 text-left font-medium">{labels.category}</th>
              <th className="px-4 py-2 text-right font-medium">{labels.samples}</th>
              <th className="px-4 py-2 text-right font-medium">{labels.ratio}</th>
              <th className="px-4 py-2 text-left font-medium">{labels.waitReason}</th>
              <th className="px-4 py-2 text-left font-medium">{labels.topMethods}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length > 0 ? (
              rows.map((row) => (
                <tr key={row.category} className="border-b border-border last:border-0">
                  <td className="px-4 py-2">{row.executive_label}</td>
                  <td className="px-4 py-2 text-right tabular-nums">
                    {formatNumber(row.samples)}
                  </td>
                  <td className="px-4 py-2 text-right tabular-nums">
                    {formatPercent(row.total_ratio)}
                  </td>
                  <td className="px-4 py-2 text-xs text-muted-foreground">
                    {row.wait_reason ?? "-"}
                  </td>
                  <td className="px-4 py-2 text-xs">
                    {row.top_methods.map((item) => item.name).join(", ") || "-"}
                  </td>
                </tr>
              ))
            ) : (
              <tr>
                <td className="px-4 py-3 text-muted-foreground" colSpan={5}>
                  —
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

function TimelineTable({
  rows,
  labelForSegment,
  labels,
}: {
  rows: TimelineAnalysisRow[];
  labelForSegment: (segment: string, fallback?: string) => string;
  labels: {
    title: string;
    segment: string;
    samples: string;
    estimatedSeconds: string;
    ratio: string;
    methodChain: string;
    topMethods: string;
  };
}): JSX.Element {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{labels.title}</CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
              <th className="px-4 py-2 text-left font-medium">{labels.segment}</th>
              <th className="px-4 py-2 text-right font-medium">{labels.samples}</th>
              <th className="px-4 py-2 text-right font-medium">
                {labels.estimatedSeconds}
              </th>
              <th className="px-4 py-2 text-right font-medium">{labels.ratio}</th>
              <th className="px-4 py-2 text-left font-medium">{labels.methodChain}</th>
              <th className="px-4 py-2 text-left font-medium">{labels.topMethods}</th>
            </tr>
          </thead>
          <tbody>
            {rows.length > 0 ? (
              rows.map((row) => (
                <tr key={row.segment} className="border-b border-border last:border-0">
                  <td className="px-4 py-2">
                    {labelForSegment(row.segment, row.label)}
                  </td>
                  <td className="px-4 py-2 text-right tabular-nums">
                    {formatNumber(row.samples)}
                  </td>
                  <td className="px-4 py-2 text-right tabular-nums">
                    {formatSeconds(row.estimated_seconds)}
                  </td>
                  <td className="px-4 py-2 text-right tabular-nums">
                    {formatPercent(row.stage_ratio)}
                  </td>
                  <td className="max-w-[520px] px-4 py-2 font-mono text-xs">
                    {row.method_chains[0]?.chain ?? "-"}
                  </td>
                  <td className="px-4 py-2 text-xs">
                    {row.top_methods.map((item) => item.name).join(", ") || "-"}
                  </td>
                </tr>
              ))
            ) : (
              <tr>
                <td className="px-4 py-3 text-muted-foreground" colSpan={6}>
                  —
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

function TimelineScopeCard({
  scope,
  t,
}: {
  scope: ProfilerTimelineScope;
  t: (key: MessageKey) => string;
}): JSX.Element {
  return (
    <Card className="p-3">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs">
        <span className="font-medium text-foreground/80">
          {t("timelineBaseScope")}
        </span>
        <span className="font-mono text-foreground">
          {scope.base_method ?? "-"}
        </span>
        <span className="text-muted-foreground">
          {formatNumber(scope.base_samples)} {t("samples")} /{" "}
          {formatPercent(scope.base_ratio_of_total)} {t("totalRatio")}
        </span>
        {scope.warnings.map((warning) => (
          <span key={warning.code} className="text-destructive">
            {warning.code}: {warning.message}
          </span>
        ))}
      </div>
    </Card>
  );
}

function breakdownDonutOption(rows: ExecutionBreakdownRow[]): EChartsOption {
  return {
    tooltip: { trigger: "item" },
    legend: { bottom: 0 },
    series: [
      {
        type: "pie",
        radius: ["48%", "72%"],
        data: rows.map((row) => ({ name: row.executive_label, value: row.samples })),
      },
    ],
  };
}

function breakdownBarOption(rows: ExecutionBreakdownRow[]): EChartsOption {
  const largeOptions =
    rows.length >= 1_000
      ? { large: true, progressive: 800, progressiveThreshold: 2_000 }
      : {};
  return {
    tooltip: { trigger: "axis", axisPointer: { type: "shadow" } },
    grid: { left: 144, right: 24, top: 24, bottom: 36 },
    xAxis: { type: "value" },
    yAxis: { type: "category", data: rows.map((row) => row.executive_label) },
    series: [{ type: "bar", data: rows.map((row) => row.samples), ...largeOptions }],
  };
}

function timelineCompositionOption(
  rows: TimelineAnalysisRow[],
  labelForSegment: (segment: string, fallback?: string) => string,
): EChartsOption {
  return {
    tooltip: {
      trigger: "item",
      valueFormatter: (value) => `${Number(value).toFixed(2)}%`,
    },
    legend: { bottom: 0 },
    grid: { left: 24, right: 24, top: 24, bottom: 64 },
    xAxis: {
      type: "value",
      max: 100,
      axisLabel: { formatter: "{value}%" },
    },
    yAxis: { type: "category", data: [""] },
    series: rows.map((row) => ({
      name: labelForSegment(row.segment, row.label),
      type: "bar",
      stack: "timeline",
      data: [row.stage_ratio],
      emphasis: { focus: "series" },
    })),
  };
}

function extractLeaves(root: FlameNode): Array<{ path: string[]; samples: number }> {
  const leaves: Array<{ path: string[]; samples: number }> = [];
  const stack = [root];
  while (stack.length > 0) {
    const node = stack.pop();
    if (!node) continue;
    if (node.children.length === 0 && node.path.length > 0) {
      leaves.push({ path: node.path, samples: node.samples });
      continue;
    }
    for (let index = node.children.length - 1; index >= 0; index -= 1) {
      stack.push(node.children[index]);
    }
  }
  return leaves;
}

function buildTree(leaves: Array<{ path: string[]; samples: number }>, label: string): FlameNode {
  const root: FlameNode = {
    id: `stage:${label}`,
    parentId: null,
    name: label,
    samples: leaves.reduce((sum, leaf) => sum + leaf.samples, 0),
    ratio: 100,
    category: null,
    color: null,
    children: [],
    path: [],
  };
  for (const leaf of leaves) {
    let current = root;
    leaf.path.forEach((frame, index) => {
      let child = current.children.find((item) => item.name === frame);
      if (!child) {
        child = {
          id: `${current.id}/${index}-${frame}`,
          parentId: current.id,
          name: frame,
          samples: 0,
          ratio: 0,
          category: null,
          color: null,
          children: [],
          path: leaf.path.slice(0, index + 1),
        };
        current.children.push(child);
      }
      child.samples += leaf.samples;
      current = child;
    });
  }
  assignRatios(root, Math.max(root.samples, 1));
  return root;
}

function assignRatios(node: FlameNode, total: number): void {
  node.ratio = (node.samples / total) * 100;
  node.children.sort((a, b) => b.samples - a.samples);
  node.children.forEach((child) => assignRatios(child, total));
}

function matchedIndex(
  path: string[],
  pattern: string,
  filterType: FilterType,
  matchMode: MatchMode,
): number {
  if (matchMode === "ordered") {
    const terms = pattern
      .split(/[>;]/)
      .map((term) => term.trim())
      .filter(Boolean);
    let termIndex = 0;
    let startIndex = -1;
    for (let index = 0; index < path.length; index += 1) {
      if (frameMatches(path[index], terms[termIndex], filterType)) {
        if (startIndex < 0) startIndex = index;
        termIndex += 1;
        if (termIndex === terms.length) return startIndex;
      }
    }
    return -1;
  }
  return path.findIndex((frame) => frameMatches(frame, pattern, filterType));
}

function frameMatches(frame: string, pattern: string, filterType: FilterType): boolean {
  if (filterType.startsWith("regex")) {
    try {
      return new RegExp(pattern).test(frame);
    } catch {
      return false;
    }
  }
  return frame.toLowerCase().includes(pattern.toLowerCase());
}

function buildBreakdown(
  root: FlameNode,
  intervalMs: number,
  elapsedSec?: number,
): ExecutionBreakdownRow[] {
  const categories = new Map<string, ExecutionBreakdownRow>();
  for (const leaf of extractLeaves(root)) {
    const classification = classifyFrames(leaf.path);
    const existing = categories.get(classification.category) ?? {
      category: classification.category,
      executive_label: classification.label,
      primary_category: classification.category,
      wait_reason: classification.waitReason,
      samples: 0,
      estimated_seconds: 0,
      total_ratio: 0,
      parent_stage_ratio: 0,
      elapsed_ratio: null,
      top_methods: [],
      top_stacks: [],
    };
    existing.samples += leaf.samples;
    existing.estimated_seconds = existing.samples * (intervalMs / 1000);
    existing.total_ratio = (existing.samples / Math.max(root.samples, 1)) * 100;
    existing.parent_stage_ratio = existing.total_ratio;
    existing.elapsed_ratio = elapsedSec ? (existing.estimated_seconds / elapsedSec) * 100 : null;
    existing.top_methods = [
      { name: leaf.path[leaf.path.length - 1] ?? "-", samples: leaf.samples },
    ];
    existing.top_stacks = [{ name: leaf.path.join(";"), samples: leaf.samples }];
    categories.set(classification.category, existing);
  }
  return [...categories.values()].sort((a, b) => b.samples - a.samples);
}

function buildTimeline(
  root: FlameNode,
  intervalMs: number,
  elapsedSec?: number,
  timelineBaseMethod?: string,
  totalSamples?: number,
): TimelineAnalysisView {
  const accumulators = new Map<string, TimelineAccumulator>();
  const baseMethod = timelineBaseMethod?.trim() ?? "";
  let stageTotal = baseMethod ? 0 : root.samples;
  const originalTotal = totalSamples ?? root.samples;
  for (const leaf of extractLeaves(root)) {
    const scopedPath = timelineScopedPath(leaf.path, baseMethod);
    if (!scopedPath) continue;
    if (baseMethod) stageTotal += leaf.samples;
    const segment = timelineSegment(scopedPath);
    const existing = accumulators.get(segment) ?? {
      segment,
      label: timelineDefaultLabel(segment),
      samples: 0,
      methods: new Map<string, number>(),
      chains: new Map<string, number>(),
      stacks: new Map<string, number>(),
    };
    existing.samples += leaf.samples;
    incrementMap(existing.methods, scopedPath[scopedPath.length - 1] ?? "(no-frame)", leaf.samples);
    incrementMap(existing.chains, timelineMethodChain(scopedPath, segment), leaf.samples);
    incrementMap(existing.stacks, scopedPath.join(";"), leaf.samples);
    accumulators.set(segment, existing);
  }

  const scope = timelineScope(baseMethod, stageTotal, originalTotal);
  if (baseMethod && stageTotal <= 0) {
    return { rows: [], scope };
  }

  const stageDenominator = Math.max(stageTotal, 1);
  const totalDenominator = Math.max(originalTotal, 1);
  const rows = TIMELINE_ORDER.flatMap((segment, index) => {
    const item = accumulators.get(segment);
    if (!item || item.samples <= 0) return [];
    const estimatedSeconds = item.samples * (intervalMs / 1000);
    return [
      {
        index,
        segment,
        label: item.label,
        samples: item.samples,
        estimated_seconds: estimatedSeconds,
        stage_ratio: (item.samples / stageDenominator) * 100,
        total_ratio: (item.samples / totalDenominator) * 100,
        elapsed_ratio: elapsedSec ? (estimatedSeconds / elapsedSec) * 100 : null,
        top_methods: topMapRows(item.methods, 5),
        method_chains: topChainRows(item.chains, 5),
        top_stacks: topMapRows(item.stacks, 5),
      },
    ];
  });
  return { rows, scope };
}

function timelineScopedPath(path: string[], baseMethod: string): string[] | null {
  if (!baseMethod) return path;
  const needle = baseMethod.toLowerCase();
  const matchIndex = path.findIndex((frame) => frame.toLowerCase().includes(needle));
  if (matchIndex < 0) return null;
  return path.slice(matchIndex);
}

function timelineScope(
  baseMethod: string,
  baseSamples: number,
  totalSamples: number,
): ProfilerTimelineScope {
  const warnings =
    baseMethod && baseSamples <= 0
      ? [
          {
            code: "TIMELINE_BASE_METHOD_NOT_FOUND",
            message: "No profiler stack matched the configured timeline base method.",
          },
        ]
      : [];
  return {
    mode: baseMethod ? "base_method" : "full_profile",
    base_method: baseMethod || null,
    match_mode: "frame_contains_case_insensitive",
    view_mode: baseMethod ? "reroot_at_base_frame" : "preserve_full_path",
    base_samples: baseSamples,
    total_samples: totalSamples,
    base_ratio_of_total: totalSamples > 0 ? (baseSamples / totalSamples) * 100 : null,
    warnings,
  };
}

function timelineSegment(path: string[]): string {
  const classification = classifyFrames(path);
  if (classification.category === "SQL_DATABASE" && classification.waitReason === "NETWORK_IO_WAIT") {
    return "DB_NETWORK_WAIT";
  }
  if (classification.category === "EXTERNAL_API_HTTP" && classification.waitReason === "NETWORK_IO_WAIT") {
    return "EXTERNAL_NETWORK_WAIT";
  }
  if (classification.category === "SQL_DATABASE") return "SQL_EXECUTION";
  if (classification.category === "EXTERNAL_API_HTTP") return "EXTERNAL_CALL";
  if (classification.category === "CONNECTION_POOL_WAIT") return "CONNECTION_POOL_WAIT";
  if (classification.category === "LOCK_SYNCHRONIZATION_WAIT") {
    return "LOCK_SYNCHRONIZATION_WAIT";
  }
  if (classification.category === "NETWORK_IO_WAIT") return "NETWORK_IO_WAIT";
  if (looksLikeStartup(path)) return "STARTUP_FRAMEWORK";
  if (looksLikeInternalMethod(path)) return "INTERNAL_METHOD";
  return "UNKNOWN";
}

function looksLikeStartup(path: string[]): boolean {
  const stack = path.join(";").toLowerCase();
  return /springapplication\.run|joblauncher|commandlinejobrunner|simplejoblauncher|batchapplication|\.main|application\.run/.test(stack);
}

function looksLikeInternalMethod(path: string[]): boolean {
  const stack = path.join(";").toLowerCase();
  return /(^|;)com\.|(^|;)org\.|service|controller|processor|writer|reader|tasklet|job/.test(stack);
}

function timelineMethodChain(path: string[], segment: string): string {
  const frames = selectTimelineFrames(path, segment);
  return frames.length > 0 ? frames.join(" -> ") : "(no-frame)";
}

function selectTimelineFrames(path: string[], segment: string): string[] {
  if (path.length <= 6) return path;
  const tokens = timelineFrameTokens(segment);
  if (tokens.length > 0) {
    const selected = path.filter((frame) =>
      tokens.some((token) => frame.toLowerCase().includes(token)),
    );
    if (selected.length > 0) return selected.slice(0, 6);
  }
  return path.slice(-6);
}

function timelineFrameTokens(segment: string): string[] {
  if (segment === "SQL_EXECUTION" || segment === "DB_NETWORK_WAIT") {
    return [
      "oracle.jdbc",
      "java.sql",
      "t4cpreparedstatement",
      "t4cmarengine",
      "executequery",
      "executeupdate",
      "socketinputstream.socketread",
    ];
  }
  if (segment === "EXTERNAL_CALL" || segment === "EXTERNAL_NETWORK_WAIT") {
    return [
      "resttemplate",
      "webclient",
      "httpclient",
      "okhttp",
      "urlconnection",
      "mainclientexec",
      "socketinputstream.socketread",
    ];
  }
  if (segment === "CONNECTION_POOL_WAIT") {
    return ["hikaripool.getconnection", "concurrentbag", "synchronousqueue"];
  }
  if (segment === "LOCK_SYNCHRONIZATION_WAIT") {
    return ["locksupport.park", "unsafe.park", "object.wait", "future.get"];
  }
  return [];
}

function incrementMap(map: Map<string, number>, key: string, value: number): void {
  map.set(key, (map.get(key) ?? 0) + value);
}

function topMapRows(map: Map<string, number>, limit: number): Array<{ name: string; samples: number }> {
  return [...map.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, limit)
    .map(([name, samples]) => ({ name, samples }));
}

function topChainRows(
  map: Map<string, number>,
  limit: number,
): TimelineAnalysisRow["method_chains"] {
  return [...map.entries()]
    .sort((a, b) => b[1] - a[1])
    .slice(0, limit)
    .map(([chain, samples]) => ({
      chain,
      samples,
      frames: chain === "(no-frame)" ? [] : chain.split(" -> "),
    }));
}

function timelineDefaultLabel(segment: string): string {
  const labels: Record<string, string> = {
    STARTUP_FRAMEWORK: "Startup / framework",
    INTERNAL_METHOD: "Internal method",
    SQL_EXECUTION: "SQL execution",
    DB_NETWORK_WAIT: "DB network wait",
    EXTERNAL_CALL: "External call",
    EXTERNAL_NETWORK_WAIT: "External network wait",
    CONNECTION_POOL_WAIT: "Connection pool wait",
    LOCK_SYNCHRONIZATION_WAIT: "Lock / synchronization wait",
    NETWORK_IO_WAIT: "Network / I/O wait",
    FILE_IO: "File I/O",
    JVM_GC_RUNTIME: "JVM / GC runtime",
    UNKNOWN: "Unclassified",
  };
  return labels[segment] ?? segment;
}

function timelineSegmentLabel(
  segment: string,
  t: (key: MessageKey) => string,
  fallback?: string,
): string {
  const keys: Record<string, MessageKey> = {
    STARTUP_FRAMEWORK: "timelineSegmentStartup",
    INTERNAL_METHOD: "timelineSegmentInternal",
    SQL_EXECUTION: "timelineSegmentSql",
    DB_NETWORK_WAIT: "timelineSegmentDbNetwork",
    EXTERNAL_CALL: "timelineSegmentExternal",
    EXTERNAL_NETWORK_WAIT: "timelineSegmentExternalNetwork",
    CONNECTION_POOL_WAIT: "timelineSegmentPoolWait",
    LOCK_SYNCHRONIZATION_WAIT: "timelineSegmentLockWait",
    NETWORK_IO_WAIT: "timelineSegmentNetworkWait",
    FILE_IO: "timelineSegmentFileIo",
    JVM_GC_RUNTIME: "timelineSegmentJvm",
    UNKNOWN: "timelineSegmentUnknown",
  };
  const key = keys[segment];
  return key ? t(key) : fallback ?? segment;
}

function classifyFrames(path: string[]): { category: string; label: string; waitReason: string | null } {
  const stack = path.join(";").toLowerCase();
  const hasHttp = /resttemplate|webclient|httpclient|okhttp|urlconnection|axios|fetch|requests\.|urllib|net\/http/.test(stack);
  const hasNetwork = /socketinputstream\.socketread|niosocketimpl|socketchannelimpl\.read|socketdispatcher\.read|epollwait|poll|recv|read0/.test(stack);
  if (/hikaripool\.getconnection|concurrentbag|synchronousqueue|datasource\.getconnection/.test(stack)) {
    return { category: "CONNECTION_POOL_WAIT", label: "Lock / Pool Wait", waitReason: null };
  }
  if (/locksupport\.park|unsafe\.park|object\.wait|countdownlatch\.await|future\.get/.test(stack)) {
    return { category: "LOCK_SYNCHRONIZATION_WAIT", label: "Lock / Pool Wait", waitReason: null };
  }
  if (/oracle\.jdbc|java\.sql|javax\.sql|executequery|executeupdate|resultset|com\.mysql|org\.postgresql/.test(stack)) {
    return { category: "SQL_DATABASE", label: "SQL / DB", waitReason: hasNetwork ? "NETWORK_IO_WAIT" : null };
  }
  if (hasHttp) {
    return {
      category: "EXTERNAL_API_HTTP",
      label: "External Call",
      waitReason: hasNetwork ? "NETWORK_IO_WAIT" : null,
    };
  }
  if (hasNetwork) {
    return { category: "NETWORK_IO_WAIT", label: "Network / I/O Wait", waitReason: null };
  }
  return { category: "UNKNOWN", label: "Others", waitReason: null };
}

function rootSamples(stage: DrilldownStageView): number {
  return stage.metrics.total_ratio > 0
    ? stage.flamegraph.samples / (stage.metrics.total_ratio / 100)
    : stage.flamegraph.samples;
}

const RECENT_FILES_KEY = "archscope.profiler.recentFiles";
const MAX_RECENT_FILES = 10;

type RecentFileEntry = {
  path: string;
  originalName: string;
  size: number;
  format: ProfileFormat;
  addedAt: number;
};

function loadRecentFiles(): RecentFileEntry[] {
  try {
    const raw = window.localStorage.getItem(RECENT_FILES_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter(
        (entry): entry is RecentFileEntry =>
          entry &&
          typeof entry.path === "string" &&
          typeof entry.originalName === "string" &&
          typeof entry.size === "number" &&
          typeof entry.format === "string" &&
          typeof entry.addedAt === "number",
      )
      .slice(0, MAX_RECENT_FILES);
  } catch {
    return [];
  }
}

function saveRecentFiles(entries: RecentFileEntry[]): void {
  try {
    window.localStorage.setItem(RECENT_FILES_KEY, JSON.stringify(entries));
  } catch {
    /* quota or access errors — accept silently. */
  }
}

function clearRecentFiles(): void {
  try {
    window.localStorage.removeItem(RECENT_FILES_KEY);
  } catch {
    /* ignore */
  }
}

type RecentFilesPanelProps = {
  t: (key: MessageKey) => string;
  files: RecentFileEntry[];
  onClear: () => void;
  onPickBaseline: (entry: RecentFileEntry) => void;
  onPickTarget: (entry: RecentFileEntry) => void;
};

function RecentFilesPanel({
  t,
  files,
  onClear,
  onPickBaseline,
  onPickTarget,
}: RecentFilesPanelProps): JSX.Element {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
        <div>
          <CardTitle className="text-sm">{t("profilerSessionsTitle")}</CardTitle>
          <p className="text-xs text-muted-foreground">{t("profilerSessionsHint")}</p>
        </div>
        {files.length > 0 ? (
          <Button type="button" variant="outline" size="sm" onClick={onClear}>
            {t("profilerSessionsClear")}
          </Button>
        ) : null}
      </CardHeader>
      <CardContent className="p-0">
        {files.length === 0 ? (
          <p className="px-6 py-4 text-center text-xs text-muted-foreground">
            {t("profilerSessionsEmpty")}
          </p>
        ) : (
          <ul className="divide-y divide-border">
            {files.map((entry) => (
              <li
                key={entry.path}
                className="cursor-pointer px-4 py-2 text-xs hover:bg-muted/40"
                title={entry.path}
                onClick={(e) => {
                  if (e.shiftKey) onPickTarget(entry);
                  else onPickBaseline(entry);
                }}
              >
                <div className="flex items-center justify-between gap-3">
                  <span className="truncate font-mono text-[11px]">{entry.originalName}</span>
                  <span className="shrink-0 text-[10px] uppercase text-muted-foreground">
                    {entry.format}
                  </span>
                </div>
                <div className="mt-0.5 flex items-center justify-between text-[10px] text-muted-foreground">
                  <span className="truncate">{entry.path}</span>
                  <span className="ml-2 shrink-0 tabular-nums">
                    {(entry.size / 1024).toFixed(1)} KB ·{" "}
                    {new Date(entry.addedAt).toLocaleString()}
                  </span>
                </div>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

type ThreadFilterProps = {
  t: (key: MessageKey) => string;
  threads: Array<{ name: string; samples: number; ratio: number }>;
  selected: string | null;
  onChange: (value: string | null) => void;
};

function ThreadFilter({ t, threads, selected, onChange }: ThreadFilterProps): JSX.Element {
  return (
    <Card className="p-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-xs">
        <span className="font-medium text-foreground/80">{t("profilerThreadFilterLabel")}</span>
        <select
          className="h-7 max-w-[420px] rounded-md border border-border bg-background px-2 text-xs"
          value={selected ?? ""}
          onChange={(e) => onChange(e.target.value || null)}
        >
          <option value="">
            {t("profilerThreadFilterAll")} (
            {t("profilerThreadFilterDetected").replace("{n}", String(threads.length))})
          </option>
          {threads.slice(0, 100).map((th) => (
            <option key={th.name} value={th.name}>
              {th.name} — {(th.ratio * 100).toFixed(2)}%
            </option>
          ))}
        </select>
      </div>
    </Card>
  );
}

type FlameDisplayToolbarProps = {
  t: (key: MessageKey) => string;
  highlightPattern: string;
  onHighlightChange: (value: string) => void;
  simplifyNames: boolean;
  onSimplifyChange: (value: boolean) => void;
  normalizeLambdas: boolean;
  onNormalizeLambdasChange: (value: boolean) => void;
  dottedNames: boolean;
  onDottedChange: (value: boolean) => void;
  inverted: boolean;
  onInvertedChange: (value: boolean) => void;
  minWidthPercent: number;
  onMinWidthChange: (value: number) => void;
};

function FlameDisplayToolbar({
  t,
  highlightPattern,
  onHighlightChange,
  simplifyNames,
  onSimplifyChange,
  normalizeLambdas,
  onNormalizeLambdasChange,
  dottedNames,
  onDottedChange,
  inverted,
  onInvertedChange,
  minWidthPercent,
  onMinWidthChange,
}: FlameDisplayToolbarProps): JSX.Element {
  return (
    <Card className="p-3">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs">
        <span className="font-medium text-foreground/80">{t("flameDisplayOptions")}</span>
        <label className="flex items-center gap-2">
          <span className="text-muted-foreground">{t("flameHighlightLabel")}</span>
          <Input
            type="search"
            placeholder={t("flameHighlightHint")}
            className="h-7 w-64 text-xs"
            value={highlightPattern}
            onChange={(e) => onHighlightChange(e.target.value)}
          />
        </label>
        <label className="flex cursor-pointer items-center gap-1.5">
          <input
            type="checkbox"
            checked={simplifyNames}
            onChange={(e) => onSimplifyChange(e.target.checked)}
          />
          {t("flameSimplifyNames")}
        </label>
        <label className="flex cursor-pointer items-center gap-1.5">
          <input
            type="checkbox"
            checked={normalizeLambdas}
            onChange={(e) => onNormalizeLambdasChange(e.target.checked)}
          />
          {t("flameNormalizeLambdas")}
        </label>
        <label className="flex cursor-pointer items-center gap-1.5">
          <input
            type="checkbox"
            checked={dottedNames}
            onChange={(e) => onDottedChange(e.target.checked)}
          />
          {t("flameDottedNames")}
        </label>
        <label className="flex cursor-pointer items-center gap-1.5">
          <input
            type="checkbox"
            checked={inverted}
            onChange={(e) => onInvertedChange(e.target.checked)}
          />
          {t("flameInverted")}
        </label>
        <label className="flex items-center gap-1.5">
          <span className="text-muted-foreground">{t("flameMinWidth")}</span>
          <select
            className="h-7 rounded-md border border-border bg-background px-2 text-xs"
            value={minWidthPercent}
            onChange={(e) => onMinWidthChange(Number(e.target.value))}
          >
            <option value={0}>0%</option>
            <option value={0.1}>0.1%</option>
            <option value={0.5}>0.5%</option>
            <option value={1}>1%</option>
            <option value={2}>2%</option>
            <option value={5}>5%</option>
          </select>
        </label>
      </div>
    </Card>
  );
}

type ProfilerDiffSectionProps = FlameDisplayToolbarProps & {
  baselineFile: FileDockSelection | null;
  onBaselineSelected: (file: FileDockSelection) => void;
  onBaselineCleared: () => void;
  baselineFormat: ProfileFormat;
  onBaselineFormatChange: (value: ProfileFormat) => void;
  targetFile: FileDockSelection | null;
  onTargetSelected: (file: FileDockSelection) => void;
  onTargetCleared: () => void;
  targetFormat: ProfileFormat;
  onTargetFormatChange: (value: ProfileFormat) => void;
  normalize: boolean;
  onNormalizeChange: (value: boolean) => void;
  diffState: AnalyzerState;
  diffError: BridgeError | null;
  diffResult: {
    flame: FlameGraphNode | null;
    summary: Record<string, unknown>;
    biggestIncreases: Array<Record<string, unknown>>;
    biggestDecreases: Array<Record<string, unknown>>;
  } | null;
  onRun: () => void;
  displayOptions: { simplifyNames: boolean; normalizeLambdas: boolean; dottedNames: boolean };
  recentFiles: RecentFileEntry[];
  onClearRecent: () => void;
  onPickBaseline: (entry: RecentFileEntry) => void;
  onPickTarget: (entry: RecentFileEntry) => void;
};

function ProfilerDiffSection(props: ProfilerDiffSectionProps): JSX.Element {
  const {
    t,
    baselineFile,
    onBaselineSelected,
    onBaselineCleared,
    baselineFormat,
    onBaselineFormatChange,
    targetFile,
    onTargetSelected,
    onTargetCleared,
    targetFormat,
    onTargetFormatChange,
    normalize,
    onNormalizeChange,
    diffState,
    diffError,
    diffResult,
    onRun,
    highlightPattern,
    displayOptions,
  } = props;

  const canRun =
    Boolean(baselineFile?.filePath) &&
    Boolean(targetFile?.filePath) &&
    diffState !== "running";

  return (
    <div className="grid gap-4">
      <RecentFilesPanel
        t={t}
        files={props.recentFiles}
        onClear={props.onClearRecent}
        onPickBaseline={props.onPickBaseline}
        onPickTarget={props.onPickTarget}
      />

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">{t("profilerDiffTitle")}</CardTitle>
          <p className="text-xs text-muted-foreground">{t("profilerDiffDescription")}</p>
        </CardHeader>
        <CardContent className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div className="space-y-2">
            <FileDock
              label={t("profilerDiffBaseline")}
              selected={baselineFile}
              onSelect={onBaselineSelected}
              onClear={onBaselineCleared}
              browseLabel={t("browseFile")}
              accept=".collapsed,.txt,.svg,.html,.csv"
            />
            <label className="flex items-center gap-2 text-xs">
              <span className="text-muted-foreground">{t("profilerDiffFormat")}</span>
              <select
                className="h-8 rounded-md border border-border bg-background px-2 text-sm"
                value={baselineFormat}
                onChange={(e) => onBaselineFormatChange(e.target.value as ProfileFormat)}
              >
                <option value="collapsed">collapsed</option>
                <option value="flamegraph_svg">flamegraph_svg</option>
                <option value="flamegraph_html">flamegraph_html</option>
                <option value="jennifer_csv">jennifer_csv</option>
              </select>
            </label>
          </div>
          <div className="space-y-2">
            <FileDock
              label={t("profilerDiffTarget")}
              selected={targetFile}
              onSelect={onTargetSelected}
              onClear={onTargetCleared}
              browseLabel={t("browseFile")}
              accept=".collapsed,.txt,.svg,.html,.csv"
            />
            <label className="flex items-center gap-2 text-xs">
              <span className="text-muted-foreground">{t("profilerDiffFormat")}</span>
              <select
                className="h-8 rounded-md border border-border bg-background px-2 text-sm"
                value={targetFormat}
                onChange={(e) => onTargetFormatChange(e.target.value as ProfileFormat)}
              >
                <option value="collapsed">collapsed</option>
                <option value="flamegraph_svg">flamegraph_svg</option>
                <option value="flamegraph_html">flamegraph_html</option>
                <option value="jennifer_csv">jennifer_csv</option>
              </select>
            </label>
          </div>
        </CardContent>
        <CardContent className="flex flex-wrap items-center gap-3 pt-0">
          <label className="flex cursor-pointer items-center gap-1.5 text-xs">
            <input
              type="checkbox"
              checked={normalize}
              onChange={(e) => onNormalizeChange(e.target.checked)}
            />
            {t("profilerDiffNormalize")}
          </label>
          <Button type="button" size="sm" disabled={!canRun} onClick={onRun}>
            {diffState === "running" ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                {t("analyzing")}
              </>
            ) : (
              <>
                <Play className="h-3.5 w-3.5" />
                {t("profilerDiffRun")}
              </>
            )}
          </Button>
        </CardContent>
      </Card>

      <ErrorPanel
        error={diffError}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />

      <FlameDisplayToolbar
        t={t}
        highlightPattern={highlightPattern}
        onHighlightChange={props.onHighlightChange}
        simplifyNames={props.simplifyNames}
        onSimplifyChange={props.onSimplifyChange}
        normalizeLambdas={props.normalizeLambdas}
        onNormalizeLambdasChange={props.onNormalizeLambdasChange}
        dottedNames={props.dottedNames}
        onDottedChange={props.onDottedChange}
        inverted={props.inverted}
        onInvertedChange={props.onInvertedChange}
        minWidthPercent={props.minWidthPercent}
        onMinWidthChange={props.onMinWidthChange}
      />

      {diffResult?.flame ? (
        <D3FlameGraph
          title={t("profilerDiffTitle")}
          exportName="profiler-diff-flamegraph"
          data={diffResult.flame}
          emptyLabel={t("flameGraphEmpty")}
          colorMode="diff"
          highlightPattern={highlightPattern || undefined}
          displayOptions={displayOptions}
          inverted={props.inverted}
          minWidthPercent={props.minWidthPercent}
        />
      ) : null}

      {diffResult ? (
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <DiffTable
            title={t("profilerDiffBiggestIncreases")}
            rows={diffResult.biggestIncreases}
            t={t}
            sign="positive"
          />
          <DiffTable
            title={t("profilerDiffBiggestDecreases")}
            rows={diffResult.biggestDecreases}
            t={t}
            sign="negative"
          />
        </div>
      ) : null}
    </div>
  );
}

function DiffTable({
  title,
  rows,
  t,
  sign,
}: {
  title: string;
  rows: Array<Record<string, unknown>>;
  t: (key: MessageKey) => string;
  sign: "positive" | "negative";
}): JSX.Element {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        {rows.length === 0 ? (
          <p className="px-6 py-6 text-center text-sm text-muted-foreground">—</p>
        ) : (
          <table className="w-full text-xs">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">{t("profilerDiffStack")}</th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  {t("profilerDiffBaselineValue")}
                </th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  {t("profilerDiffTargetValue")}
                </th>
                <th className="w-[80px] px-3 py-2 text-right font-medium">
                  {t("profilerDiffDelta")}
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.slice(0, 15).map((row, index) => {
                const stack = String(row.stack ?? "");
                const baseline = Number(row.baseline ?? 0);
                const target = Number(row.target ?? 0);
                const delta = Number(row.delta ?? 0);
                return (
                  <tr key={`${stack}-${index}`} className="border-b border-border last:border-0">
                    <td className="max-w-[420px] truncate px-3 py-2 font-mono" title={stack}>
                      {stack}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">{baseline.toFixed(4)}</td>
                    <td className="px-3 py-2 text-right tabular-nums">{target.toFixed(4)}</td>
                    <td
                      className={`px-3 py-2 text-right font-semibold tabular-nums ${
                        sign === "positive" ? "text-rose-600 dark:text-rose-400" : "text-sky-600 dark:text-sky-400"
                      }`}
                    >
                      {(delta >= 0 ? "+" : "") + delta.toFixed(4)}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        )}
      </CardContent>
    </Card>
  );
}
