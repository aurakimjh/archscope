// Wails port of apps/frontend/src/pages/ProfilerAnalyzerPage.tsx.
//
// The web original drives 2k+ lines of correlation, drilldown, diff and tree
// views. This port mirrors the visual structure (FileDock + options Card +
// Tabs) and the most-used analyses, but reuses the simpler Wails-side
// charting (CanvasFlameGraph, HorizontalBarChart, DrilldownPanel) so we ship
// a consistent profiler page without porting every web sub-component.

import { Loader2, Play, Square } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";

import {
  ProfilerService,
  AnalyzeRequest,
  ExportPprofRequest,
} from "../../bindings/github.com/aurakimjh/archscope/apps/profiler-native/cmd/archscope-profiler-app";
import type {
  AnalysisResult,
  FlameNode,
} from "../../bindings/github.com/aurakimjh/archscope/apps/profiler-native/internal/profiler/models";

import { CanvasFlameGraph, type FlameGraphNode } from "../components/CanvasFlameGraph";
import { DiagnosticsPanel } from "../components/DiagnosticsPanel";
import { DrilldownPanel } from "../components/DrilldownPanel";
import { HorizontalBarChart, type BarRow } from "../components/HorizontalBarChart";
import { MetricCard } from "../components/MetricCard";
import {
  WailsFileDock,
  type FileDockSelection,
} from "../components/WailsFileDock";
import { Button } from "../components/ui/button";
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "../components/ui/card";
import { Input } from "../components/ui/input";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "../components/ui/tabs";
import { useRecentFiles } from "../hooks/useRecentFiles";
import { useShortcuts } from "../hooks/useShortcuts";
import { useI18n } from "../i18n/I18nProvider";
import { loadDefaults } from "../state/defaults";

type ProfileFormat =
  | "collapsed"
  | "jennifer"
  | "flamegraph_svg"
  | "flamegraph_html";

type ProfileKind = "wall" | "cpu" | "lock";

const FILE_FILTERS = [
  {
    displayName: "All profiler inputs",
    pattern: "*.collapsed;*.txt;*.csv;*.svg;*.html;*.htm",
  },
  { displayName: "Collapsed stacks", pattern: "*.collapsed;*.txt" },
  { displayName: "Jennifer flamegraph CSV", pattern: "*.csv" },
  { displayName: "FlameGraph SVG", pattern: "*.svg" },
  { displayName: "FlameGraph HTML", pattern: "*.html;*.htm" },
];

function detectFormat(filePath: string): ProfileFormat {
  const lower = filePath.toLowerCase();
  if (lower.endsWith(".csv")) return "jennifer";
  if (lower.endsWith(".svg")) return "flamegraph_svg";
  if (lower.endsWith(".html") || lower.endsWith(".htm")) return "flamegraph_html";
  return "collapsed";
}

function adaptFlameNode(node: FlameNode | null | undefined): FlameGraphNode | null {
  if (!node) return null;
  const children = (node.children ?? [])
    .map(adaptFlameNode)
    .filter((child): child is FlameGraphNode => child !== null);
  return {
    name: node.name ?? "",
    value: node.samples ?? 0,
    category: node.category ?? null,
    color: node.color ?? null,
    children,
  };
}

export function ProfilerAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const recent = useRecentFiles();
  const defaults = useMemo(loadDefaults, []);

  const [selected, setSelected] = useState<FileDockSelection | null>(null);
  const [format, setFormat] = useState<ProfileFormat>("collapsed");
  const [profileKind, setProfileKind] = useState<ProfileKind>(defaults.profileKind);
  const [intervalMs, setIntervalMs] = useState<number>(defaults.intervalMs);
  const [elapsedSec, setElapsedSec] = useState<string>("");
  const [topN, setTopN] = useState<number>(defaults.topN);
  const [timelineBaseMethod, setTimelineBaseMethod] = useState<string>("");

  const [analyzing, setAnalyzing] = useState<boolean>(false);
  const [exporting, setExporting] = useState<boolean>(false);
  const [exportNotice, setExportNotice] = useState<string>("");
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [error, setError] = useState<string>("");
  const [activeTab, setActiveTab] = useState<string>("summary");
  const activeTaskRef = useRef<string | null>(null);

  const path = selected?.filePath ?? "";

  useEffect(() => {
    const offDone = Events.On("analyze:done", (event: any) => {
      const data = event?.data ?? event;
      if (!data || data.taskId !== activeTaskRef.current) return;
      activeTaskRef.current = null;
      setResult(data.result);
      setActiveTab("summary");
      setAnalyzing(false);
      if (path) recent.push(path);
    });
    const offError = Events.On("analyze:error", (event: any) => {
      const data = event?.data ?? event;
      if (!data || data.taskId !== activeTaskRef.current) return;
      activeTaskRef.current = null;
      setError(data.message ?? "analysis failed");
      setResult(null);
      setAnalyzing(false);
    });
    const offCancelled = Events.On("analyze:cancelled", (event: any) => {
      const data = event?.data ?? event;
      if (!data || data.taskId !== activeTaskRef.current) return;
      activeTaskRef.current = null;
      setAnalyzing(false);
    });
    return () => {
      offDone?.();
      offError?.();
      offCancelled?.();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [path]);

  const handleSelect = (file: FileDockSelection) => {
    setSelected(file);
    setFormat(detectFormat(file.filePath));
    setError("");
    setExportNotice("");
  };

  const handleClear = () => {
    setSelected(null);
    setResult(null);
    setError("");
    setExportNotice("");
  };

  const handleAnalyze = async () => {
    if (!path) return;
    setError("");
    setExportNotice("");
    setAnalyzing(true);
    try {
      const elapsed = elapsedSec.trim() === "" ? -1 : Number(elapsedSec);
      const request = new AnalyzeRequest({
        path,
        format,
        intervalMs,
        elapsedSec: Number.isFinite(elapsed) ? elapsed : -1,
        topN,
        profileKind,
        timelineBaseMethod,
      });
      const response = await ProfilerService.AnalyzeAsync(request);
      activeTaskRef.current = response.taskId;
    } catch (err: any) {
      setError(String(err?.message ?? err));
      setResult(null);
      setAnalyzing(false);
    }
  };

  const handleCancel = async () => {
    const taskId = activeTaskRef.current;
    if (!taskId) return;
    try {
      await ProfilerService.Cancel(taskId);
    } catch {
      // best-effort
    } finally {
      activeTaskRef.current = null;
      setAnalyzing(false);
    }
  };

  const handleExportPprof = async () => {
    if (!path) return;
    setError("");
    setExportNotice("");
    setExporting(true);
    try {
      const req = new ExportPprofRequest({
        path,
        format,
        intervalMs,
        elapsedSec: -1,
        topN,
        profileKind,
        timelineBaseMethod,
        outputPath: "",
      } as any);
      const res = await ProfilerService.ExportPprof(req);
      if (res?.outputPath) {
        setExportNotice(`${t("exportPprofSaved")} ${res.outputPath}`);
      }
    } catch (err: any) {
      setError(String(err?.message ?? err));
    } finally {
      setExporting(false);
    }
  };

  useShortcuts({
    onOpen: () => undefined,
    onAnalyze: () => {
      if (!analyzing && path) void handleAnalyze();
    },
    onCancel: () => {
      if (analyzing) void handleCancel();
    },
    onSettings: () => undefined,
    onTab: () => undefined,
  });

  // Derived view data
  const summary = result?.summary;
  const diagnostics = result?.metadata?.diagnostics ?? null;
  const timelineScope = result?.metadata?.timeline_scope;
  const topStacks = (result?.series?.top_stacks ?? []).slice(0, 50);
  const topChildFrames = result?.tables?.top_child_frames ?? [];
  const flamegraph = useMemo(
    () => adaptFlameNode(result?.charts?.flamegraph),
    [result],
  );
  const exportName = useMemo(() => {
    if (!path) return "archscope-flamegraph";
    const segments = path.split(/[\\/]/);
    const last = segments[segments.length - 1] || "archscope-flamegraph";
    return last.replace(/\.[^.]+$/, "") + "-flamegraph";
  }, [path]);

  const drilldownBase = result
    ? new AnalyzeRequest({
        path,
        format,
        intervalMs,
        elapsedSec: elapsedSec.trim() === "" ? -1 : Number(elapsedSec) || -1,
        topN,
        profileKind,
        timelineBaseMethod,
      })
    : null;

  const breakdownRows: BarRow[] = (result?.series?.execution_breakdown ?? [])
    .filter((row) => (row?.samples ?? 0) > 0)
    .map((row) => ({
      key: row.category,
      label: row.executive_label || row.category || "(unknown)",
      value: row.samples,
      ratio: typeof row.total_ratio === "number" ? row.total_ratio : undefined,
    }));

  const timelineRows: BarRow[] = (result?.series?.timeline_analysis ?? [])
    .filter((row) => (row?.samples ?? 0) > 0)
    .map((row) => ({
      key: `${row.index}-${row.segment}`,
      label: row.label || row.segment || "(unknown)",
      value: row.samples,
      ratio: typeof row.total_ratio === "number" ? row.total_ratio : undefined,
    }));

  const errorBadge =
    (diagnostics?.error_count ?? 0) + (diagnostics?.warning_count ?? 0);

  useEffect(() => {
    if (!result) setActiveTab("summary");
  }, [result]);

  const canAnalyze = !analyzing && !!path;

  return (
    <main className="flex flex-col gap-5 p-5">
      <WailsFileDock
        label={t("filePathPlaceholder")}
        description={t("dropOrBrowseProfiler") || t("filePathPlaceholder")}
        selected={selected}
        onSelect={handleSelect}
        onClear={handleClear}
        browseLabel={t("pickFile")}
        fileFilters={FILE_FILTERS}
        rightSlot={
          <div className="flex items-center gap-2">
            <Button
              type="button"
              size="sm"
              disabled={!canAnalyze}
              onClick={() => void handleAnalyze()}
            >
              {analyzing ? (
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
            {analyzing && (
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => void handleCancel()}
              >
                <Square className="h-3.5 w-3.5" />
                {t("cancel")}
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
            <span className="font-medium text-foreground/80">{t("format")}</span>
            <select
              className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
              value={format}
              onChange={(e) => setFormat(e.target.value as ProfileFormat)}
              disabled={analyzing}
            >
              <option value="collapsed">{t("formatCollapsed")}</option>
              <option value="jennifer">{t("formatJennifer")}</option>
              <option value="flamegraph_svg">{t("formatFlamegraphSvg")}</option>
              <option value="flamegraph_html">{t("formatFlamegraphHtml")}</option>
            </select>
          </label>
          {format === "collapsed" && (
            <label className="flex flex-col gap-1.5 text-xs">
              <span className="font-medium text-foreground/80">
                {t("profileKind")}
              </span>
              <select
                className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
                value={profileKind}
                onChange={(e) => setProfileKind(e.target.value as ProfileKind)}
                disabled={analyzing}
              >
                <option value="wall">wall</option>
                <option value="cpu">cpu</option>
                <option value="lock">lock</option>
              </select>
            </label>
          )}
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("intervalMs")}
            </span>
            <Input
              type="number"
              min={1}
              value={intervalMs}
              onChange={(e) => setIntervalMs(Number(e.target.value) || 100)}
              disabled={analyzing}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("elapsedSec")}
            </span>
            <Input
              type="text"
              placeholder="—"
              value={elapsedSec}
              onChange={(e) => setElapsedSec(e.target.value)}
              disabled={analyzing}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("topN")}</span>
            <Input
              type="number"
              min={1}
              max={100}
              value={topN}
              onChange={(e) => setTopN(Number(e.target.value) || 20)}
              disabled={analyzing}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs md:col-span-2 xl:col-span-1">
            <span className="font-medium text-foreground/80">
              {t("timelineBaseMethod")}
            </span>
            <Input
              type="text"
              placeholder="—"
              value={timelineBaseMethod}
              onChange={(e) => setTimelineBaseMethod(e.target.value)}
              disabled={analyzing}
            />
          </label>
        </CardContent>
      </Card>

      {error && (
        <div
          role="alert"
          className="rounded-md border border-destructive/40 bg-destructive/5 p-3 text-sm text-destructive"
        >
          <strong className="block">{t("error")}</strong>
          <p className="mt-1 text-foreground">{error}</p>
        </div>
      )}
      {exportNotice && (
        <div className="rounded-md border border-primary/30 bg-primary/5 p-3 text-sm text-foreground">
          {exportNotice}
        </div>
      )}

      <Tabs value={activeTab} onValueChange={setActiveTab} className="w-full">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <TabsList>
            <TabsTrigger value="summary" disabled={!result}>
              {t("tabSummary")}
            </TabsTrigger>
            <TabsTrigger value="flame" disabled={!flamegraph}>
              {t("tabFlamegraph")}
            </TabsTrigger>
            <TabsTrigger
              value="timeline"
              disabled={!result || timelineRows.length === 0}
            >
              {t("tabTimeline")}
            </TabsTrigger>
            <TabsTrigger value="drilldown" disabled={!drilldownBase}>
              {t("tabDrilldown")}
            </TabsTrigger>
            <TabsTrigger value="breakdown" disabled={!result}>
              {t("breakdownTitle")}
            </TabsTrigger>
            <TabsTrigger value="topstacks" disabled={!result}>
              {t("topStacks")}
            </TabsTrigger>
            <TabsTrigger value="diagnostics" disabled={!result}>
              {t("tabDiagnostics")}
              {errorBadge > 0 && (
                <span className="ml-1.5 rounded-full bg-destructive px-1.5 py-0.5 text-[10px] font-bold text-destructive-foreground">
                  {errorBadge}
                </span>
              )}
            </TabsTrigger>
          </TabsList>
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={!result || exporting || !path}
            onClick={() => void handleExportPprof()}
          >
            {exporting ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                {t("exportPprofBusy")}
              </>
            ) : (
              t("exportPprof")
            )}
          </Button>
        </div>

        <TabsContent value="summary" className="mt-4">
          {summary ? (
            <div className="grid gap-4">
              <section className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <MetricCard
                  label={t("totalSamples")}
                  value={summary.total_samples.toLocaleString()}
                />
                <MetricCard
                  label={t("estimatedSeconds")}
                  value={summary.estimated_seconds.toFixed(3)}
                />
                <MetricCard
                  label={t("intervalMsLabel")}
                  value={summary.interval_ms.toString()}
                />
                <MetricCard
                  label={t("elapsedLabel")}
                  value={
                    summary.elapsed_seconds == null
                      ? "—"
                      : summary.elapsed_seconds.toFixed(3)
                  }
                />
                <MetricCard
                  label={t("profileKindLabel")}
                  value={summary.profile_kind}
                />
                <MetricCard
                  label={t("parser")}
                  value={result?.metadata?.parser ?? ""}
                />
              </section>

              {topChildFrames.length > 0 && (
                <Card>
                  <CardHeader className="pb-3">
                    <CardTitle className="text-sm">
                      {t("topChildFrames")}
                    </CardTitle>
                  </CardHeader>
                  <CardContent className="overflow-x-auto p-0">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                          <th className="px-4 py-2 text-left font-medium">
                            {t("framesColumn")}
                          </th>
                          <th className="px-4 py-2 text-right font-medium">
                            {t("samplesColumn")}
                          </th>
                          <th className="px-4 py-2 text-right font-medium">
                            {t("ratioColumn")}
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {topChildFrames.map((row, idx) => (
                          <tr
                            key={`${row.frame}-${idx}`}
                            className="border-b border-border last:border-0"
                          >
                            <td
                              className="px-4 py-2 font-mono text-xs"
                              title={row.frame}
                            >
                              {row.frame}
                            </td>
                            <td className="px-4 py-2 text-right tabular-nums">
                              {row.samples.toLocaleString()}
                            </td>
                            <td className="px-4 py-2 text-right tabular-nums">
                              {(row.ratio * 100).toFixed(2)}%
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </CardContent>
                </Card>
              )}
            </div>
          ) : (
            <p className="text-sm text-muted-foreground">{t("nothingYet")}</p>
          )}
        </TabsContent>

        <TabsContent value="flame" className="mt-4">
          {flamegraph ? (
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">{t("flamegraphTitle")}</CardTitle>
              </CardHeader>
              <CardContent>
                <CanvasFlameGraph data={flamegraph} exportName={exportName} />
              </CardContent>
            </Card>
          ) : (
            <p className="text-sm text-muted-foreground">{t("nothingYet")}</p>
          )}
        </TabsContent>

        <TabsContent value="timeline" className="mt-4">
          <div className="grid gap-4">
            {timelineScope && (
              <Card>
                <CardHeader className="pb-3">
                  <CardTitle className="text-sm">
                    {t("timelineScope")}
                  </CardTitle>
                </CardHeader>
                <CardContent className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                  <MetricCard
                    label={t("timelineScopeMode")}
                    value={timelineScope.mode}
                  />
                  <MetricCard
                    label={t("timelineScopeBaseMethod")}
                    value={timelineScope.base_method ?? "—"}
                  />
                  <MetricCard
                    label={t("timelineScopeMatch")}
                    value={timelineScope.match_mode}
                  />
                  <MetricCard
                    label={t("timelineScopeView")}
                    value={timelineScope.view_mode}
                  />
                  <MetricCard
                    label={t("timelineScopeBaseSamples")}
                    value={timelineScope.base_samples.toLocaleString()}
                  />
                  <MetricCard
                    label={t("timelineScopeBaseRatio")}
                    value={
                      timelineScope.base_ratio_of_total == null
                        ? "—"
                        : `${timelineScope.base_ratio_of_total.toFixed(2)}%`
                    }
                  />
                </CardContent>
              </Card>
            )}
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">{t("timelineTitle")}</CardTitle>
              </CardHeader>
              <CardContent>
                <HorizontalBarChart
                  rows={timelineRows}
                  emptyLabel={t("timelineEmpty")}
                  valueSuffix={t("samples")}
                  ratioFractionDigits={2}
                />
              </CardContent>
            </Card>
            {timelineRows.length > 0 && (
              <Card>
                <CardHeader className="pb-3">
                  <CardTitle className="text-sm">
                    {t("timelineEvidenceTable")}
                  </CardTitle>
                </CardHeader>
                <CardContent className="overflow-x-auto p-0">
                  <table className="w-full text-sm">
                    <thead>
                      <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                        <th className="px-4 py-2 text-left font-medium">
                          {t("timelineSegment")}
                        </th>
                        <th className="px-4 py-2 text-right font-medium">
                          {t("samples")}
                        </th>
                        <th className="px-4 py-2 text-right font-medium">
                          {t("ratio")}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {(result?.series?.timeline_analysis ?? []).map((row, idx) => (
                        <tr
                          key={`${row.index}-${row.segment}-${idx}`}
                          className="border-b border-border last:border-0"
                        >
                          <td
                            className="px-4 py-2 text-xs"
                            title={row.label || row.segment}
                          >
                            {row.label || row.segment || "(unknown)"}
                          </td>
                          <td className="px-4 py-2 text-right tabular-nums">
                            {(row.samples ?? 0).toLocaleString()}
                          </td>
                          <td className="px-4 py-2 text-right tabular-nums">
                            {typeof row.total_ratio === "number"
                              ? `${row.total_ratio.toFixed(2)}%`
                              : "—"}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </CardContent>
              </Card>
            )}
          </div>
        </TabsContent>

        <TabsContent value="drilldown" className="mt-4">
          {drilldownBase ? (
            <DrilldownPanel baseRequest={drilldownBase} onError={setError} />
          ) : (
            <p className="text-sm text-muted-foreground">{t("nothingYet")}</p>
          )}
        </TabsContent>

        <TabsContent value="breakdown" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("breakdownTitle")}</CardTitle>
            </CardHeader>
            <CardContent>
              <HorizontalBarChart
                rows={breakdownRows}
                emptyLabel={t("breakdownEmpty")}
                valueSuffix={t("samples")}
                ratioFractionDigits={2}
              />
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="topstacks" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("topStacks")}</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto p-0">
              {topStacks.length > 0 ? (
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                      <th className="px-4 py-2 text-left font-medium">
                        {t("topStacks")}
                      </th>
                      <th className="px-4 py-2 text-right font-medium">
                        {t("samples")}
                      </th>
                      <th className="px-4 py-2 text-right font-medium">
                        {t("ratio")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {topStacks.map((row, idx) => (
                      <tr
                        key={`${row.stack}-${idx}`}
                        className="border-b border-border last:border-0"
                      >
                        <td className="px-4 py-2 font-mono text-xs" title={row.stack}>
                          {row.stack}
                        </td>
                        <td className="px-4 py-2 text-right tabular-nums">
                          {row.samples.toLocaleString()}
                        </td>
                        <td className="px-4 py-2 text-right tabular-nums">
                          {row.sample_ratio.toFixed(2)}%
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              ) : (
                <p className="px-4 py-3 text-sm text-muted-foreground">
                  {t("nothingYet")}
                </p>
              )}
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="diagnostics" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("tabDiagnostics")}</CardTitle>
            </CardHeader>
            <CardContent>
              <DiagnosticsPanel
                diagnostics={diagnostics}
                baseRequest={drilldownBase}
              />
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </main>
  );
}
