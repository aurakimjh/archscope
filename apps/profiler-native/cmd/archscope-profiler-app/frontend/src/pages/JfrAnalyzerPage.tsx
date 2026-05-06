import { Loader2, Play, Square } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import { engine } from "@/bridge/engine";
import type {
  BridgeError,
  JfrAnalysisMode,
  JfrAnalysisResult,
  JfrHeatmapStrip,
  NativeMemoryAnalysisResult,
  NativeMemoryFlameNode,
} from "@/bridge/types";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import {
  CanvasFlameGraph,
  type FlameGraphNode,
} from "@/components/CanvasFlameGraph";
import { MetricCard } from "@/components/MetricCard";
import {
  WailsFileDock,
  type FileDockSelection,
} from "@/components/WailsFileDock";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";

// Wails port of apps/frontend/src/pages/JfrAnalyzerPage.tsx.
//
// Mirrors the Phase-1 AccessLog pattern (file-dock + Tabs + result
// surfaces) but layers two analyzer modes on a single page:
//
//   • "JFR Recording"  → engine.analyzeJfr(req)
//        Multi-event mode filter, time-window filter, wall-clock
//        heatmap strip, notable-events table, event-type breakdown.
//   • "Native Memory"  → engine.analyzeNativeMemory(req)
//        Leak detection, tail-ratio cutoff, top-call-sites table,
//        flame tree of unfreed allocations.
//
// The two share the file-picker but each has its own option strip,
// in-flight state, and result panels. The mode selector is a top-level
// Tabs strip so the same .jfr/.json file can drive both surfaces
// without re-uploading.
//
// Behavioural deltas vs. the web original:
//   • File path comes from a Wails native dialog (or drop) rather
//     than an HTTP multipart upload — same as the AccessLog port.
//   • The native heatmap strip uses a CSS-grid renderer (no D3) so
//     we don't need the `D3HeatmapStrip` web component. Selection
//     drag-to-filter is deferred (clicks set fromTime; reset clears).
//   • The flame graph reuses the existing `CanvasFlameGraph` (already
//     ported for the profiler page) instead of a separate D3 widget.

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

const MODE_OPTIONS: Array<{ value: JfrAnalysisMode; labelKey: MessageKey }> = [
  { value: "all", labelKey: "jfrModeAll" },
  { value: "cpu", labelKey: "jfrModeCpu" },
  { value: "wall", labelKey: "jfrModeWall" },
  { value: "alloc", labelKey: "jfrModeAlloc" },
  { value: "lock", labelKey: "jfrModeLock" },
  { value: "gc", labelKey: "jfrModeGc" },
  { value: "exception", labelKey: "jfrModeException" },
  { value: "io", labelKey: "jfrModeIo" },
];

const TAIL_RATIO_OPTIONS: Array<{ value: number; label: string }> = [
  { value: 0, label: "0%" },
  { value: 0.05, label: "5%" },
  { value: 0.1, label: "10%" },
  { value: 0.2, label: "20%" },
  { value: 0.3, label: "30%" },
];

export function JfrAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(
    null,
  );

  // ── JFR-recording state ──────────────────────────────────────────
  const [mode, setMode] = useState<JfrAnalysisMode>("all");
  const [fromTime, setFromTime] = useState("");
  const [toTime, setToTime] = useState("");
  const [stateFilter, setStateFilter] = useState("");
  const [topN, setTopN] = useState<number>(20);
  const [jfrState, setJfrState] = useState<AnalyzerState>("idle");
  const [jfrError, setJfrError] = useState<BridgeError | null>(null);
  const [jfrResult, setJfrResult] = useState<JfrAnalysisResult | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const inflightRef = useRef<symbol | null>(null);

  // ── Native-memory state ─────────────────────────────────────────
  const [leakOnly, setLeakOnly] = useState<boolean>(true);
  const [tailRatio, setTailRatio] = useState<number>(0.1);
  const [nativeMemState, setNativeMemState] = useState<AnalyzerState>("idle");
  const [nativeMemError, setNativeMemError] = useState<BridgeError | null>(
    null,
  );
  const [nativeMemResult, setNativeMemResult] =
    useState<NativeMemoryAnalysisResult | null>(null);
  const nativeMemInflightRef = useRef<symbol | null>(null);

  const filePath = selectedFile?.filePath ?? "";
  const canAnalyzeJfr = Boolean(filePath) && jfrState !== "running";
  const canAnalyzeNativeMem = Boolean(filePath) && nativeMemState !== "running";

  const summary = jfrResult?.summary;
  const metadata = jfrResult?.metadata;
  const availableModes = metadata?.available_modes ?? [];
  const availableStates = metadata?.available_states ?? [];

  const eventTypeRows = jfrResult?.series?.events_by_type ?? [];
  const heatmap: JfrHeatmapStrip | undefined =
    jfrResult?.series?.heatmap_strip;
  const notableRows = jfrResult?.tables?.notable_events ?? [];

  const flameGraph = useMemo<FlameGraphNode | null>(() => {
    const node = nativeMemResult?.charts?.flamegraph;
    if (!node) return null;
    return adaptFlameNode(node);
  }, [nativeMemResult]);

  const nativeSummary = nativeMemResult?.summary;
  const topSites = nativeMemResult?.tables?.top_call_sites ?? [];

  const handleFileSelected = useCallback((file: FileDockSelection) => {
    setSelectedFile(file);
    setJfrState("ready");
    setJfrError(null);
    setEngineMessages([]);
    if (nativeMemState !== "running") setNativeMemState("ready");
    setNativeMemError(null);
  }, [nativeMemState]);

  const handleClearFile = useCallback(() => {
    setSelectedFile(null);
    setJfrResult(null);
    setNativeMemResult(null);
    if (jfrState !== "running") setJfrState("idle");
    if (nativeMemState !== "running") setNativeMemState("idle");
    setEngineMessages([]);
  }, [jfrState, nativeMemState]);

  // ── JFR analysis ────────────────────────────────────────────────
  const analyzeJfr = useCallback(async () => {
    if (!canAnalyzeJfr) return;
    setJfrState("running");
    setJfrError(null);
    setEngineMessages([]);
    const token = Symbol("jfr-analysis");
    inflightRef.current = token;

    try {
      const response = await engine.analyzeJfr({
        path: filePath,
        topN,
        mode,
        fromTime: fromTime.trim() || undefined,
        toTime: toTime.trim() || undefined,
        state: stateFilter.trim() || undefined,
      });
      if (inflightRef.current !== token) return;
      inflightRef.current = null;
      setJfrResult(response);
      setJfrState("success");
    } catch (caught) {
      if (inflightRef.current !== token) return;
      inflightRef.current = null;
      const message = caught instanceof Error ? caught.message : String(caught);
      setJfrError({ code: "ENGINE_FAILED", message });
      setJfrState("error");
    }
  }, [canAnalyzeJfr, filePath, fromTime, mode, stateFilter, toTime, topN]);

  const cancelJfr = useCallback(() => {
    inflightRef.current = null;
    setJfrState("ready");
    setEngineMessages([t("analysisCanceled")]);
  }, [t]);

  // ── Native memory analysis ──────────────────────────────────────
  const analyzeNativeMemory = useCallback(async () => {
    if (!canAnalyzeNativeMem) return;
    setNativeMemState("running");
    setNativeMemError(null);
    const token = Symbol("native-memory");
    nativeMemInflightRef.current = token;

    try {
      const response = await engine.analyzeNativeMemory({
        path: filePath,
        leakOnly,
        tailRatio,
      });
      if (nativeMemInflightRef.current !== token) return;
      nativeMemInflightRef.current = null;
      setNativeMemResult(response);
      setNativeMemState("success");
    } catch (caught) {
      if (nativeMemInflightRef.current !== token) return;
      nativeMemInflightRef.current = null;
      const message = caught instanceof Error ? caught.message : String(caught);
      setNativeMemError({ code: "ENGINE_FAILED", message });
      setNativeMemState("error");
    }
  }, [canAnalyzeNativeMem, filePath, leakOnly, tailRatio]);

  const cancelNativeMem = useCallback(() => {
    nativeMemInflightRef.current = null;
    setNativeMemState("ready");
  }, []);

  return (
    <main className="flex flex-col gap-5 p-5">
      <WailsFileDock
        label={t("selectJfrFile")}
        description={t("dropOrBrowseJfr")}
        accept=".jfr,.json"
        selected={selectedFile}
        onSelect={handleFileSelected}
        onClear={handleClearFile}
        browseLabel={t("browseFile")}
        dropHereLabel={t("dropHere")}
        errorLabel={t("error")}
        fileFilters={[
          { displayName: "JFR recordings", pattern: "*.jfr;*.json" },
          { displayName: "All files", pattern: "*.*" },
        ]}
      />

      <Tabs defaultValue="recording" className="w-full">
        <TabsList>
          <TabsTrigger value="recording">{t("jfrModeRecording")}</TabsTrigger>
          <TabsTrigger value="native">{t("jfrModeNativeMemory")}</TabsTrigger>
        </TabsList>

        {/* ── Mode 1: JFR Recording ──────────────────────────────── */}
        <TabsContent value="recording" className="mt-4 flex flex-col gap-5">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
              <CardTitle className="text-sm">{t("analyzerOptions")}</CardTitle>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  size="sm"
                  disabled={!canAnalyzeJfr}
                  onClick={() => void analyzeJfr()}
                >
                  {jfrState === "running" ? (
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
                {jfrState === "running" && (
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => cancelJfr()}
                  >
                    <Square className="h-3.5 w-3.5" />
                    {t("cancelAnalysis")}
                  </Button>
                )}
              </div>
            </CardHeader>
            <CardContent className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
              <label className="flex flex-col gap-1.5 text-xs">
                <span className="font-medium text-foreground/80">
                  {t("jfrModeLabel")}
                </span>
                <select
                  className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
                  value={mode}
                  onChange={(e) =>
                    setMode(e.target.value as JfrAnalysisMode)
                  }
                >
                  {MODE_OPTIONS.map((option) => {
                    const enabled =
                      availableModes.length === 0 ||
                      option.value === "all" ||
                      availableModes.includes(option.value);
                    return (
                      <option
                        key={option.value}
                        value={option.value}
                        disabled={!enabled}
                      >
                        {t(option.labelKey)}
                        {!enabled ? " (—)" : ""}
                      </option>
                    );
                  })}
                </select>
              </label>
              <label className="flex flex-col gap-1.5 text-xs">
                <span className="font-medium text-foreground/80">
                  {t("jfrFromTimeLabel")}
                </span>
                <Input
                  type="text"
                  placeholder={t("jfrTimeHint")}
                  value={fromTime}
                  onChange={(event) => setFromTime(event.target.value)}
                />
              </label>
              <label className="flex flex-col gap-1.5 text-xs">
                <span className="font-medium text-foreground/80">
                  {t("jfrToTimeLabel")}
                </span>
                <Input
                  type="text"
                  placeholder={t("jfrTimeHint")}
                  value={toTime}
                  onChange={(event) => setToTime(event.target.value)}
                />
              </label>
              <label className="flex flex-col gap-1.5 text-xs">
                <span className="font-medium text-foreground/80">
                  {t("topN")}
                </span>
                <Input
                  type="number"
                  min={1}
                  value={topN}
                  onChange={(event) =>
                    setTopN(Number(event.target.value) || 20)
                  }
                />
              </label>
              <label className="flex flex-col gap-1.5 text-xs">
                <span className="font-medium text-foreground/80">
                  {t("jfrStateLabel")}
                </span>
                <select
                  className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
                  value={stateFilter}
                  onChange={(e) => setStateFilter(e.target.value)}
                >
                  <option value="">{t("jfrStateAll")}</option>
                  {availableStates.map((s) => (
                    <option key={s} value={s}>
                      {s}
                    </option>
                  ))}
                </select>
              </label>
            </CardContent>
          </Card>

          <ErrorPanel
            error={jfrError}
            labels={{ title: t("analysisError"), code: t("errorCode") }}
          />
          <EngineMessagesPanel
            messages={engineMessages}
            title={t("engineMessages")}
          />

          <section className="grid grid-cols-2 gap-3 md:grid-cols-4">
            <MetricCard
              label={t("jfrEventCountFiltered")}
              value={formatCount(summary?.event_count)}
            />
            <MetricCard
              label={t("jfrEventCountTotal")}
              value={formatCount(summary?.event_count_total)}
            />
            <MetricCard
              label={t("jfrSelectedMode")}
              value={summary?.selected_mode ?? mode}
            />
            <MetricCard
              label={t("jfrAvailableModes")}
              value={
                availableModes.filter((m) => m !== "all").join(", ") || "—"
              }
            />
          </section>

          <Tabs defaultValue="events" className="w-full">
            <TabsList>
              <TabsTrigger value="events">{t("jfrTabEvents")}</TabsTrigger>
              <TabsTrigger value="heatmap">{t("jfrHeatmap")}</TabsTrigger>
              <TabsTrigger value="breakdown">
                {t("jfrEventTypeBreakdown")}
              </TabsTrigger>
              <TabsTrigger value="diagnostics">
                {t("tabDiagnostics")}
              </TabsTrigger>
            </TabsList>

            <TabsContent value="events" className="mt-4">
              <NotableEventsPanel rows={notableRows} t={t} />
            </TabsContent>

            <TabsContent value="heatmap" className="mt-4">
              <HeatmapPanel
                heatmap={heatmap}
                t={t}
                onPickStart={(time) => setFromTime(time)}
                onClear={() => {
                  setFromTime("");
                  setToTime("");
                }}
              />
            </TabsContent>

            <TabsContent value="breakdown" className="mt-4">
              <EventTypeBreakdownPanel rows={eventTypeRows} t={t} />
            </TabsContent>

            <TabsContent value="diagnostics" className="mt-4">
              <DiagnosticsPanel
                diagnostics={metadata?.diagnostics}
                labels={{
                  title: t("parserDiagnostics"),
                  parsedRecords: t("parsedRecords"),
                  skippedLines: t("skippedLines"),
                  samples: t("diagnosticSamples"),
                }}
              />
            </TabsContent>
          </Tabs>
        </TabsContent>

        {/* ── Mode 2: Native Memory ──────────────────────────────── */}
        <TabsContent value="native" className="mt-4 flex flex-col gap-5">
          <Card>
            <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
              <CardTitle className="text-sm">
                {t("jfrNativeMemTitle")}
              </CardTitle>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  size="sm"
                  disabled={!canAnalyzeNativeMem}
                  onClick={() => void analyzeNativeMemory()}
                >
                  {nativeMemState === "running" ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      {t("analyzing")}
                    </>
                  ) : (
                    <>
                      <Play className="h-3.5 w-3.5" />
                      {t("jfrNativeMemRun")}
                    </>
                  )}
                </Button>
                {nativeMemState === "running" && (
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    onClick={() => cancelNativeMem()}
                  >
                    <Square className="h-3.5 w-3.5" />
                    {t("cancelAnalysis")}
                  </Button>
                )}
              </div>
            </CardHeader>
            <CardContent className="flex flex-wrap items-center gap-4 text-xs">
              <p className="w-full text-xs text-muted-foreground">
                {t("jfrNativeMemDesc")}
              </p>
              <label className="flex cursor-pointer items-center gap-1.5">
                <input
                  type="checkbox"
                  checked={leakOnly}
                  onChange={(e) => setLeakOnly(e.target.checked)}
                />
                {t("jfrNativeMemLeakOnly")}
              </label>
              <label className="flex items-center gap-2">
                <span className="text-muted-foreground">
                  {t("jfrNativeMemTailRatio")}
                </span>
                <select
                  className="h-7 rounded-md border border-input bg-transparent px-2 text-xs"
                  value={String(tailRatio)}
                  onChange={(e) => setTailRatio(Number(e.target.value))}
                >
                  {TAIL_RATIO_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>
                      {opt.label}
                    </option>
                  ))}
                </select>
                <span className="text-[10px] text-muted-foreground">
                  {t("jfrNativeMemTailHint")}
                </span>
              </label>
            </CardContent>
          </Card>

          <ErrorPanel
            error={nativeMemError}
            labels={{ title: t("analysisError"), code: t("errorCode") }}
          />

          {nativeMemResult && (
            <>
              <section className="grid grid-cols-2 gap-3 md:grid-cols-3 xl:grid-cols-5">
                <MetricCard
                  label={t("jfrNativeMemAllocCount")}
                  value={formatCount(nativeSummary?.alloc_event_count)}
                />
                <MetricCard
                  label={t("jfrNativeMemFreeCount")}
                  value={formatCount(nativeSummary?.free_event_count)}
                />
                <MetricCard
                  label={t("jfrNativeMemAllocBytes")}
                  value={formatBytes(nativeSummary?.alloc_bytes_total)}
                />
                <MetricCard
                  label={t("jfrNativeMemUnfreedCount")}
                  value={formatCount(nativeSummary?.unfreed_event_count)}
                />
                <MetricCard
                  label={t("jfrNativeMemUnfreedBytes")}
                  value={formatBytes(nativeSummary?.unfreed_bytes_total)}
                />
              </section>

              {flameGraph && (
                <Card>
                  <CardHeader className="pb-3">
                    <CardTitle className="text-sm">
                      {t("jfrNativeMemTitle")}
                    </CardTitle>
                  </CardHeader>
                  <CardContent>
                    <CanvasFlameGraph
                      data={flameGraph}
                      exportName="native-memory-flamegraph"
                    />
                  </CardContent>
                </Card>
              )}

              <Card>
                <CardHeader className="pb-3">
                  <CardTitle className="text-sm">
                    {t("jfrNativeMemTopSites")}
                  </CardTitle>
                </CardHeader>
                <CardContent className="overflow-x-auto p-0">
                  {topSites.length === 0 ? (
                    <p className="px-6 py-6 text-center text-sm text-muted-foreground">
                      —
                    </p>
                  ) : (
                    <table className="w-full text-xs">
                      <thead>
                        <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                          <th className="px-3 py-2 text-left font-medium">
                            {t("jfrNativeMemSiteStack")}
                          </th>
                          <th className="w-[140px] px-3 py-2 text-right font-medium">
                            {t("jfrNativeMemSiteBytes")}
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {topSites.slice(0, 50).map((row, idx) => (
                          <tr
                            key={`${row.stack}-${idx}`}
                            className="border-b border-border last:border-0"
                          >
                            <td
                              className="max-w-[480px] truncate px-3 py-2 font-mono"
                              title={row.stack}
                            >
                              {row.stack}
                            </td>
                            <td className="px-3 py-2 text-right tabular-nums">
                              {formatBytes(row.bytes)}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  )}
                </CardContent>
              </Card>
            </>
          )}
        </TabsContent>
      </Tabs>
    </main>
  );
}

// ──────────────────────────────────────────────────────────────────
// Sub-panels
// ──────────────────────────────────────────────────────────────────

function NotableEventsPanel({
  rows,
  t,
}: {
  rows: JfrAnalysisResult["tables"]["notable_events"] extends infer R
    ? Exclude<R, undefined>
    : never;
  t: (key: MessageKey) => string;
}): JSX.Element {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{t("jfrTabEvents")}</CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        {rows.length === 0 ? (
          <p className="px-6 py-6 text-center text-sm text-muted-foreground">
            —
          </p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[11px] uppercase tracking-wider text-muted-foreground">
                <th className="w-[180px] px-3 py-2 text-left font-medium">
                  {t("jfrColTime")}
                </th>
                <th className="w-[200px] px-3 py-2 text-left font-medium">
                  {t("jfrColEventType")}
                </th>
                <th className="w-[120px] px-3 py-2 text-right font-medium">
                  {t("jfrColDurationMs")}
                </th>
                <th className="w-[160px] px-3 py-2 text-left font-medium">
                  {t("jfrColThread")}
                </th>
                <th className="px-3 py-2 text-left font-medium">
                  {t("jfrColMessage")}
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row, index) => (
                <tr
                  key={`${row.time ?? ""}-${index}`}
                  className="border-b border-border last:border-0"
                >
                  <td className="px-3 py-2 font-mono text-[11px] text-muted-foreground">
                    {row.time || "—"}
                  </td>
                  <td className="px-3 py-2 font-mono text-[11px]">
                    {row.event_type ?? "—"}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {typeof row.duration_ms === "number"
                      ? row.duration_ms.toLocaleString()
                      : "—"}
                  </td>
                  <td
                    className="max-w-[200px] truncate px-3 py-2 font-mono text-[11px]"
                    title={row.thread ?? ""}
                  >
                    {row.thread ?? "—"}
                  </td>
                  <td
                    className="max-w-md truncate px-3 py-2 text-xs"
                    title={row.message}
                  >
                    {row.message || "—"}
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

function EventTypeBreakdownPanel({
  rows,
  t,
}: {
  rows: { event_type: string; count: number }[];
  t: (key: MessageKey) => string;
}): JSX.Element {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{t("jfrEventTypeBreakdown")}</CardTitle>
      </CardHeader>
      <CardContent className="p-0">
        {rows.length === 0 ? (
          <p className="px-6 py-6 text-center text-sm text-muted-foreground">
            —
          </p>
        ) : (
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-[11px] uppercase tracking-wider text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">
                  {t("jfrColEventType")}
                </th>
                <th className="w-[120px] px-3 py-2 text-right font-medium">
                  {t("count")}
                </th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr
                  key={row.event_type}
                  className="border-b border-border last:border-0"
                >
                  <td className="px-3 py-2 font-mono text-xs">
                    {row.event_type}
                  </td>
                  <td className="px-3 py-2 text-right tabular-nums">
                    {row.count.toLocaleString()}
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

// HeatmapPanel renders the wall-clock event heatmap as a CSS-grid strip.
// The web app uses a D3 selection-rect interaction; here we settle for
// click-to-fill the JFR `from time` field, which is plenty for a v1.
function HeatmapPanel({
  heatmap,
  t,
  onPickStart,
  onClear,
}: {
  heatmap: JfrHeatmapStrip | undefined;
  t: (key: MessageKey) => string;
  onPickStart: (time: string) => void;
  onClear: () => void;
}): JSX.Element {
  const buckets = heatmap?.buckets ?? [];
  const maxCount = heatmap?.max_count ?? 0;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
        <div>
          <CardTitle className="text-sm">{t("jfrHeatmap")}</CardTitle>
          <p className="mt-1 text-xs text-muted-foreground">
            {t("jfrHeatmapHint")}
          </p>
        </div>
        {heatmap && buckets.length > 0 && (
          <Button type="button" size="sm" variant="ghost" onClick={onClear}>
            {t("jfrHeatmapClear")}
          </Button>
        )}
      </CardHeader>
      <CardContent>
        {!heatmap || buckets.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">—</p>
        ) : (
          <>
            <div className="flex items-stretch gap-px overflow-x-auto rounded-md border border-border bg-muted/20 p-1">
              {buckets.map((bucket) => {
                const ratio = maxCount > 0 ? bucket.count / maxCount : 0;
                const opacity = bucket.count > 0 ? 0.15 + ratio * 0.85 : 0.05;
                return (
                  <button
                    key={bucket.index}
                    type="button"
                    onClick={() => onPickStart(bucket.time)}
                    title={`${bucket.time} — ${bucket.count.toLocaleString()} events`}
                    className="h-12 min-w-[6px] flex-1 cursor-pointer rounded-sm bg-primary transition-opacity hover:ring-2 hover:ring-primary/40"
                    style={{ opacity }}
                  />
                );
              })}
            </div>
            <div className="mt-2 flex items-center justify-between text-[11px] text-muted-foreground">
              <span className="font-mono">{heatmap.start_time ?? "—"}</span>
              <span>
                {t("jfrHeatmapBucketSeconds")}:{" "}
                {heatmap.bucket_seconds.toFixed(2)}s · max {maxCount}
              </span>
              <span className="font-mono">{heatmap.end_time ?? "—"}</span>
            </div>
            <p className="mt-2 text-[11px] text-muted-foreground">
              {t("jfrHeatmapApplyHint")}
            </p>
          </>
        )}
      </CardContent>
    </Card>
  );
}

// ──────────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────────

function adaptFlameNode(node: NativeMemoryFlameNode): FlameGraphNode {
  return {
    name: node.name,
    value: node.samples,
    category: node.category ?? null,
    color: node.color ?? null,
    children: Array.isArray(node.children)
      ? node.children.map(adaptFlameNode)
      : [],
  };
}

function formatCount(value: number | undefined | null): string {
  if (value == null || !Number.isFinite(value)) return "—";
  return Number(value).toLocaleString();
}

function formatBytes(value: number | undefined | null): string {
  if (value == null || !Number.isFinite(value)) return "—";
  const v = Number(value);
  if (v < 1024) return `${v} B`;
  if (v < 1024 * 1024) return `${(v / 1024).toFixed(1)} KB`;
  if (v < 1024 * 1024 * 1024) return `${(v / 1024 / 1024).toFixed(1)} MB`;
  return `${(v / 1024 / 1024 / 1024).toFixed(2)} GB`;
}
