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
  const timelineRows = useMemo(() => {
    if (currentStage) {
      return buildTimeline(
        currentStage.flamegraph,
        wallIntervalMs,
        parsedElapsedForDerived ?? undefined,
      );
    }
    return result?.series.timeline_analysis ?? [];
  }, [currentStage, parsedElapsedForDerived, result?.series.timeline_analysis, wallIntervalMs]);
  const flameRoot = useMemo(
    () => (currentStage ? toFlameGraphNode(currentStage.flamegraph) : null),
    [currentStage],
  );
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
      });

      if (currentRequestIdRef.current !== requestId) return;
      currentRequestIdRef.current = null;

      if (response.ok) {
        setResult(response.result);
        setStages(createInitialStages(response.result));
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
        <CardContent className="grid grid-cols-1 gap-3 md:grid-cols-3 xl:grid-cols-5">
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
              />
            )}
          </div>
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
): TimelineAnalysisRow[] {
  const accumulators = new Map<string, TimelineAccumulator>();
  for (const leaf of extractLeaves(root)) {
    const segment = timelineSegment(leaf.path);
    const existing = accumulators.get(segment) ?? {
      segment,
      label: timelineDefaultLabel(segment),
      samples: 0,
      methods: new Map<string, number>(),
      chains: new Map<string, number>(),
      stacks: new Map<string, number>(),
    };
    existing.samples += leaf.samples;
    incrementMap(existing.methods, leaf.path[leaf.path.length - 1] ?? "(no-frame)", leaf.samples);
    incrementMap(existing.chains, timelineMethodChain(leaf.path, segment), leaf.samples);
    incrementMap(existing.stacks, leaf.path.join(";"), leaf.samples);
    accumulators.set(segment, existing);
  }

  const stageTotal = Math.max(root.samples, 1);
  return TIMELINE_ORDER.flatMap((segment, index) => {
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
        stage_ratio: (item.samples / stageTotal) * 100,
        total_ratio: (item.samples / stageTotal) * 100,
        elapsed_ratio: elapsedSec ? (estimatedSeconds / elapsedSec) * 100 : null,
        top_methods: topMapRows(item.methods, 5),
        method_chains: topChainRows(item.chains, 5),
        top_stacks: topMapRows(item.stacks, 5),
      },
    ];
  });
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
