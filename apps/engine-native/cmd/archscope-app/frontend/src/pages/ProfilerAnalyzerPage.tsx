// ─────────────────────────────────────────────────────────────────────
// [한글] pages/ProfilerAnalyzerPage.tsx — 메인 프로파일러 분석 페이지.
//
// 책임/목적:
//   - collapsed stack / Jennifer flamegraph CSV / FlameGraph SVG 등 다양한
//     포맷의 프로파일을 받아 ProfilerService.Analyze(AnalyzeRequest) 호출
//     → AnalysisResult + FlameNode 트리 수신.
//   - 결과를 CanvasFlameGraph(전체 트리), HorizontalBarChart(execution
//     breakdown / timeline segments), DrilldownPanel(필터 누적) 으로 렌더.
//   - ExportPprofRequest 로 pprof 형식 내보내기 지원.
//
// 다루는 결과 형식: AnalysisResult + FlameNode 트리 (Wails bindings 의
// internal/profiler/models 타입). web 셸의 거대한 ProfilerAnalyzerPage
// (2k+ LOC) 를 셸 친화적으로 슬림 다운한 버전.
//
// 데이터 흐름: ProfilerService.Analyze → 결과 setResult.
// 또한 @wailsio/runtime Events 를 listen 해 비동기 진행도/이벤트 수신.
// ─────────────────────────────────────────────────────────────────────
// Wails port of apps/frontend/src/pages/ProfilerAnalyzerPage.tsx.
//
// The web original drives 2k+ lines of correlation, drilldown, diff and tree
// views. This port mirrors the visual structure (FileDock + options Card +
// Tabs) and the most-used analyses, but reuses the simpler Wails-side
// charting (CanvasFlameGraph, HorizontalBarChart, DrilldownPanel) so we ship
// a consistent profiler page without porting every web sub-component.

import { CircleHelp, Loader2, Play, Square } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";

import {
  ProfilerService,
  AnalyzeRequest,
  ExportPprofRequest,
} from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app";
import type {
  AnalysisResult,
  FlameNode,
} from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/internal/profiler/models";

import { CanvasFlameGraph, type FlameGraphNode } from "../components/CanvasFlameGraph";
import { AnalyzerOptionsDock } from "../components/AnalyzerOptionsDock";
import {
  CustomCategoriesEditor,
  type CategoryRules,
  type Preset,
  type SegmentSpec,
} from "../components/CustomCategoriesEditor";
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
import { RecentFilesPanel } from "../components/RecentFilesPanel";
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

// PROFILER_TIMELINE_SEGMENTS lists the segments the user can override.
// The IDs are the canonical segment keys consumed by buildTimeline on
// the Go side; user patterns flow through Options.TimelineCategories.
const PROFILER_TIMELINE_SEGMENTS: SegmentSpec[] = [
  { id: "STARTUP_FRAMEWORK", label: "Startup / framework" },
  { id: "INTERNAL_METHOD", label: "Internal method" },
  { id: "SQL_EXECUTION", label: "SQL execution" },
  { id: "DB_NETWORK_WAIT", label: "DB network wait" },
  { id: "NETWORK_PREP", label: "External call prep (network)" },
  { id: "EXTERNAL_CALL", label: "External call" },
  { id: "EXTERNAL_NETWORK_WAIT", label: "External network wait" },
  { id: "CONNECTION_POOL_WAIT", label: "Connection pool wait" },
  { id: "LOCK_SYNCHRONIZATION_WAIT", label: "Lock / synchronization wait" },
  { id: "FILE_IO", label: "File I/O" },
];

// PROFILER_TIMELINE_PRESETS — built-in libraries the user can seed
// and then refine. Patterns are case-insensitive substrings; they
// merge into the existing per-segment rule list rather than replacing.
const PROFILER_TIMELINE_PRESETS = (
  t: (key: any) => string,
): Preset[] => [
  {
    id: "msa-network-prep",
    label: t("presetMsaNetworkPrep"),
    rules: {
      NETWORK_PREP: [
        "IntegrationUtil.sendToService",
        "sendToService(",
        "RestTemplate.exchange",
      ],
    },
  },
  {
    id: "msa-2pc",
    label: t("presetMsa2pc"),
    rules: {
      EXTERNAL_CALL: ["xa_start", "xa_end", "xa_prepare", "xa_commit", "xa_rollback"],
    },
  },
];

const TIMELINE_COMPOSITION_COLORS = [
  "#6366f1",
  "#22c55e",
  "#f59e0b",
  "#06b6d4",
  "#a855f7",
  "#ec4899",
  "#14b8a6",
  "#ef4444",
  "#84cc16",
  "#0ea5e9",
  "#f97316",
  "#64748b",
];

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

function timelineDisplayRatio(row: any): number | undefined {
  if (typeof row?.stage_ratio === "number") return row.stage_ratio;
  if (typeof row?.total_ratio === "number") return row.total_ratio;
  return undefined;
}

function formatTimelineSeconds(value: unknown): string {
  const n = Number(value);
  if (!Number.isFinite(n)) return "—";
  return `${n.toLocaleString(undefined, {
    maximumFractionDigits: 3,
    minimumFractionDigits: n > 0 && n < 1 ? 3 : 0,
  })} s`;
}

function formatTimelinePercent(value: number | undefined): string {
  return value == null ? "—" : `${value.toFixed(2)}%`;
}

function timelineEvidenceTooltip(row: any, scope: any): string {
  const lines: string[] = [
    `구간: ${row?.label || row?.segment || "(unknown)"}`,
    `샘플: ${(row?.samples ?? 0).toLocaleString()}`,
    `비율: ${formatTimelinePercent(timelineDisplayRatio(row))}`,
    `시간: ${formatTimelineSeconds(row?.estimated_seconds)}`,
  ];
  if (scope?.base_method) {
    lines.splice(1, 0, `기준 메서드: ${scope.base_method}`);
  }
  const methods = Array.isArray(row?.top_methods) ? row.top_methods.slice(0, 6) : [];
  if (methods.length > 0) {
    lines.push("", "근거 메서드:");
    for (const item of methods) {
      lines.push(`- ${item?.name ?? "(unknown)"} (${(item?.samples ?? 0).toLocaleString()} samples)`);
    }
  }
  const chains = Array.isArray(row?.method_chains) ? row.method_chains.slice(0, 3) : [];
  if (chains.length > 0) {
    lines.push("", "대표 호출 체인:");
    for (const item of chains) {
      lines.push(`- ${item?.chain ?? "(unknown)"} (${(item?.samples ?? 0).toLocaleString()} samples)`);
    }
  }
  return lines.join("\n");
}

function TimelineCompositionCard({
  rows,
  title,
  emptyLabel,
}: {
  rows: any[];
  title: string;
  emptyLabel: string;
}): JSX.Element {
  const visibleRows = rows.filter((row) => Number(row?.samples ?? 0) > 0);
  const totalSamples =
    visibleRows.reduce((sum, row) => sum + Number(row?.samples ?? 0), 0) || 1;
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {visibleRows.length === 0 ? (
          <p className="text-xs text-muted-foreground">{emptyLabel}</p>
        ) : (
          <>
            <div className="flex h-7 overflow-hidden rounded border border-border bg-muted/30">
              {visibleRows.map((row, idx) => {
                const samples = Number(row?.samples ?? 0);
                const widthPct = Math.max(0, (samples / totalSamples) * 100);
                const ratio = timelineDisplayRatio(row);
                const label = row?.label || row?.segment || "(unknown)";
                return (
                  <div
                    key={`${row?.index ?? idx}-${row?.segment ?? idx}`}
                    className="flex min-w-[2px] items-center justify-center overflow-hidden text-[10px] font-medium text-white"
                    style={{
                      width: `${widthPct.toFixed(2)}%`,
                      backgroundColor:
                        TIMELINE_COMPOSITION_COLORS[
                          idx % TIMELINE_COMPOSITION_COLORS.length
                        ],
                    }}
                    title={`${label}: ${samples.toLocaleString()} samples · ${formatTimelinePercent(ratio)} · ${formatTimelineSeconds(row?.estimated_seconds)}`}
                  >
                    {widthPct >= 12 ? (
                      <span className="truncate px-1">{label}</span>
                    ) : null}
                  </div>
                );
              })}
            </div>
            <ul className="grid gap-x-4 gap-y-1.5 text-[11px] sm:grid-cols-2 xl:grid-cols-3">
              {visibleRows.map((row, idx) => {
                const samples = Number(row?.samples ?? 0);
                const ratio = timelineDisplayRatio(row);
                const label = row?.label || row?.segment || "(unknown)";
                return (
                  <li
                    key={`${row?.index ?? idx}-${row?.segment ?? idx}-legend`}
                    className="flex min-w-0 items-center gap-1.5"
                    title={`${label}: ${samples.toLocaleString()} samples · ${formatTimelinePercent(ratio)} · ${formatTimelineSeconds(row?.estimated_seconds)}`}
                  >
                    <span
                      className="h-2.5 w-2.5 shrink-0 rounded-sm"
                      style={{
                        backgroundColor:
                          TIMELINE_COMPOSITION_COLORS[
                            idx % TIMELINE_COMPOSITION_COLORS.length
                          ],
                      }}
                      aria-hidden
                    />
                    <span className="min-w-0 flex-1 truncate text-foreground/75">
                      {label}
                    </span>
                    <span className="shrink-0 font-mono tabular-nums text-muted-foreground">
                      {samples.toLocaleString()} · {formatTimelinePercent(ratio)} ·{" "}
                      {formatTimelineSeconds(row?.estimated_seconds)}
                    </span>
                  </li>
                );
              })}
            </ul>
          </>
        )}
      </CardContent>
    </Card>
  );
}

export function ProfilerAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  // Profiler-scoped recent-files history. The category keeps it
  // separate from the GC / Jennifer pages so users see only the
  // files they analyzed on this page.
  const recent = useRecentFiles({ category: "profiler" });
  const defaults = useMemo(loadDefaults, []);

  const [selected, setSelected] = useState<FileDockSelection | null>(null);
  const [format, setFormat] = useState<ProfileFormat>("collapsed");
  const [profileKind, setProfileKind] = useState<ProfileKind>(defaults.profileKind);
  const [intervalMs, setIntervalMs] = useState<number>(defaults.intervalMs);
  const [elapsedSec, setElapsedSec] = useState<string>("");
  const [topN, setTopN] = useState<number>(defaults.topN);
  const [timelineBaseMethod, setTimelineBaseMethod] = useState<string>("");
  // Memory caps default to 0 = "use backend defaults" (250k stacks,
  // depth 512). Surfaced so 70M+ wall files can be tuned without a
  // rebuild when the defaults still aren't enough.
  const [maxUniqueStacks, setMaxUniqueStacks] = useState<number>(0);
  const [maxStackDepth, setMaxStackDepth] = useState<number>(0);
  const [maxRssMb, setMaxRssMb] = useState<number>(0);
  const [progressLogDir, setProgressLogDir] = useState<string>("");
  const [timelineCategories, setTimelineCategories] = useState<CategoryRules>(
    {},
  );
  const [analyzing, setAnalyzing] = useState<boolean>(false);
  const [exporting, setExporting] = useState<boolean>(false);
  const [exportNotice, setExportNotice] = useState<string>("");
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [error, setError] = useState<string>("");
  // progressLogNotice surfaces the on-disk progress-log path the
  // moment AnalyzeAsync returns. Sticks around through the entire
  // analysis so a crash / OS-kill leaves the user with a pointer
  // they can hand to support without relying on stderr.
  const [progressLogNotice, setProgressLogNotice] = useState<string>("");
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
      if (path) {
        recent.push({
          path,
          analyzer: "profiler",
          meta: {
            format,
            profileKind,
            intervalMs,
            elapsedSec,
            topN,
            timelineBaseMethod,
            maxUniqueStacks,
            maxStackDepth,
            maxRssMb,
            progressLogDir,
            timelineCategories,
          },
        });
      }
    });
    const offError = Events.On("analyze:error", (event: any) => {
      const data = event?.data ?? event;
      if (!data || data.taskId !== activeTaskRef.current) return;
      activeTaskRef.current = null;
      setError(data.message ?? "analysis failed");
      if (data.progressLogPath) {
        setProgressLogNotice(`진행 로그: ${data.progressLogPath}`);
      }
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

  // handleRecentSelect restores the file path AND the analyzer
  // options that were active when the entry was last analyzed —
  // this is the "fast re-analysis" UX, so users don't have to
  // re-tune intervalMs/topN/etc on every revisit.
  const handleRecentSelect = (entry: { path: string; name?: string; meta?: Record<string, unknown> }) => {
    setSelected({
      filePath: entry.path,
      originalName: entry.name ?? entry.path,
    } as FileDockSelection);
    setFormat(detectFormat(entry.path));
    const meta = entry.meta ?? {};
    if (typeof meta.format === "string") setFormat(meta.format as ProfileFormat);
    if (typeof meta.profileKind === "string") setProfileKind(meta.profileKind as ProfileKind);
    if (typeof meta.intervalMs === "number") setIntervalMs(meta.intervalMs);
    if (typeof meta.elapsedSec === "string") setElapsedSec(meta.elapsedSec);
    if (typeof meta.topN === "number") setTopN(meta.topN);
    if (typeof meta.timelineBaseMethod === "string") setTimelineBaseMethod(meta.timelineBaseMethod);
    if (typeof meta.maxUniqueStacks === "number") setMaxUniqueStacks(meta.maxUniqueStacks);
    if (typeof meta.maxStackDepth === "number") setMaxStackDepth(meta.maxStackDepth);
    if (typeof meta.maxRssMb === "number") setMaxRssMb(meta.maxRssMb);
    if (typeof meta.progressLogDir === "string") setProgressLogDir(meta.progressLogDir);
    if (meta.timelineCategories && typeof meta.timelineCategories === "object") {
      setTimelineCategories(meta.timelineCategories as CategoryRules);
    }
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
        maxUniqueStacks,
        maxStackDepth,
        maxRssMb,
        progressLogDir,
        timelineCategories,
      } as any);
      const response = await ProfilerService.AnalyzeAsync(request);
      activeTaskRef.current = response.taskId;
      const logPath = (response as any).progressLogPath;
      if (logPath) {
        setProgressLogNotice(`진행 로그: ${logPath}`);
      } else {
        setProgressLogNotice("");
      }
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
        maxUniqueStacks,
        maxStackDepth,
        maxRssMb,
        progressLogDir,
        timelineCategories,
      } as any)
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
      ratio: timelineDisplayRatio(row),
      detail: formatTimelineSeconds(row.estimated_seconds),
    }));

  const errorBadge =
    (diagnostics?.error_count ?? 0) + (diagnostics?.warning_count ?? 0);

  useEffect(() => {
    if (!result) setActiveTab("summary");
  }, [result]);

  const canAnalyze = !analyzing && !!path;

  return (
    <main className="flex flex-col gap-5 p-5">
      <div className="flex items-stretch gap-3">
        <WailsFileDock
          className="min-w-0 flex-1"
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

        <AnalyzerOptionsDock
          title={t("analyzerOptions")}
          width={560}
          footer={
          <div className="flex justify-end">
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
          </div>
          }
        >
      <Card className="border-0 shadow-none">
        <CardContent className="grid grid-cols-1 gap-3 sm:grid-cols-2 px-0">
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
              {t("intervalMs")} <span className="text-muted-foreground">(기본: 100)</span>
            </span>
            <Input
              type="number"
              min={1}
              placeholder="100"
              value={intervalMs}
              onChange={(e) => setIntervalMs(Number(e.target.value) || 100)}
              disabled={analyzing}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("elapsedSec")} <span className="text-muted-foreground">(기본: 자동)</span>
            </span>
            <Input
              type="text"
              placeholder="비워두면 샘플 수 × interval"
              value={elapsedSec}
              onChange={(e) => setElapsedSec(e.target.value)}
              disabled={analyzing}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("topN")} <span className="text-muted-foreground">(기본: 20)</span>
            </span>
            <Input
              type="number"
              min={1}
              max={100}
              placeholder="20"
              value={topN}
              onChange={(e) => setTopN(Number(e.target.value) || 20)}
              disabled={analyzing}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs sm:col-span-2">
            <span className="font-medium text-foreground/80">
              {t("timelineBaseMethod")} <span className="text-muted-foreground">(기본: 미설정)</span>
            </span>
            <Input
              type="text"
              placeholder="예: SpringBootApplication.run"
              value={timelineBaseMethod}
              onChange={(e) => setTimelineBaseMethod(e.target.value)}
              disabled={analyzing}
            />
          </label>
        </CardContent>
        <CardContent className="grid grid-cols-1 gap-3 sm:grid-cols-2 border-t border-border pt-3 px-0">
          <label className="flex flex-col gap-1.5 text-xs sm:col-span-2">
            <span className="font-semibold text-foreground/80">
              {t("memoryGuards")}
            </span>
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("maxUniqueStacks")}
            </span>
            <Input
              type="number"
              min={0}
              placeholder="0 = default (100000)"
              value={maxUniqueStacks || ""}
              onChange={(e) => setMaxUniqueStacks(Number(e.target.value) || 0)}
              disabled={analyzing}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              {t("maxStackDepth")}
            </span>
            <Input
              type="number"
              min={0}
              placeholder="0 = default (512)"
              value={maxStackDepth || ""}
              onChange={(e) => setMaxStackDepth(Number(e.target.value) || 0)}
              disabled={analyzing}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">
              최대 RSS (MB) <span className="text-muted-foreground">(기본: 4096)</span>
            </span>
            <Input
              type="number"
              min={0}
              placeholder="0 = default (4096 MB)"
              value={maxRssMb || ""}
              onChange={(e) => setMaxRssMb(Number(e.target.value) || 0)}
              disabled={analyzing}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs sm:col-span-2">
            <span className="font-medium text-foreground/80">
              진행 로그 디렉터리{" "}
              <span className="text-muted-foreground">
                (기본: 실행파일 옆 archscope-logs/)
              </span>
            </span>
            <Input
              type="text"
              placeholder="예: D:\\analysis-logs (비우면 실행파일 옆 archscope-logs/)"
              value={progressLogDir}
              onChange={(e) => setProgressLogDir(e.target.value)}
              disabled={analyzing}
            />
          </label>
        </CardContent>
        <CardContent className="border-t border-border pt-3 px-0">
          <p className="mb-2 text-xs font-semibold text-foreground/80">
            {t("customCategoriesTitle")}
          </p>
          <CustomCategoriesEditor
            segments={PROFILER_TIMELINE_SEGMENTS}
            presets={PROFILER_TIMELINE_PRESETS(t)}
            value={timelineCategories}
            onChange={setTimelineCategories}
            disabled={analyzing}
          />
        </CardContent>
      </Card>
        </AnalyzerOptionsDock>
      </div>

      <RecentFilesPanel
        entries={recent.entries}
        onSelect={handleRecentSelect}
        onRemove={recent.remove}
        onClear={recent.clear}
      />

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
      {progressLogNotice && (
        <div className="rounded-md border border-sky-500/30 bg-sky-500/5 p-2.5 text-xs text-foreground">
          <code className="font-mono">{progressLogNotice}</code>
          <span className="ml-2 text-muted-foreground">
            (분석 중 OS가 프로세스를 종료해도 이 파일에 진행 단계가 남습니다)
          </span>
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
            <TimelineCompositionCard
              rows={result?.series?.timeline_analysis ?? []}
              title={t("executionTimeComposition")}
              emptyLabel={t("timelineEmpty")}
            />
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
                        <th className="px-4 py-2 text-right font-medium">
                          {t("estimatedSeconds")}
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
                            <span className="inline-flex max-w-[420px] items-center gap-1.5">
                              <span className="truncate">
                                {row.label || row.segment || "(unknown)"}
                              </span>
                              <span
                                aria-label="timeline evidence"
                                title={timelineEvidenceTooltip(row, timelineScope)}
                              >
                                <CircleHelp className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                              </span>
                            </span>
                          </td>
                          <td className="px-4 py-2 text-right tabular-nums">
                            {(row.samples ?? 0).toLocaleString()}
                          </td>
                          <td className="px-4 py-2 text-right tabular-nums">
                            {formatTimelinePercent(timelineDisplayRatio(row))}
                          </td>
                          <td className="px-4 py-2 text-right tabular-nums">
                            {formatTimelineSeconds(row.estimated_seconds)}
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
