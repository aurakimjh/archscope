import { Loader2, Play, Square } from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import {
  createAnalyzerRequestId,
  getAnalyzerClient,
  type BridgeError,
  type JfrAnalysisMode,
  type JfrAnalysisResult,
} from "@/api/analyzerClient";
import {
  DiagnosticsPanel,
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { FileDock, type FileDockSelection } from "@/components/FileDock";
import {
  D3FlameGraph,
  type FlameGraphNode,
} from "@/components/charts/D3FlameGraph";
import { D3HeatmapStrip, type HeatmapBucket } from "@/components/charts/D3HeatmapStrip";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";

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

type JfrSummary = {
  event_count?: number;
  event_count_total?: number;
  duration_ms?: number;
  gc_pause_total_ms?: number;
  blocked_thread_events?: number;
  selected_mode?: string;
};

type JfrMetadata = {
  selected_mode?: string;
  available_modes?: string[];
  available_states?: string[];
  selected_state?: string | null;
  min_duration_ms?: number | null;
  filter_window?: {
    from?: string | null;
    to?: string | null;
    effective_start?: string | null;
    effective_end?: string | null;
  };
  source_format?: string;
  jfr_cli?: string | null;
  diagnostics?: unknown;
};

type JfrEventTypeRow = { event_type: string; count: number };

type HeatmapStripData = {
  bucket_seconds: number;
  start_time: string | null;
  end_time: string | null;
  max_count: number;
  buckets: HeatmapBucket[];
};

type NotableEventRow = {
  time?: string;
  event_type?: string;
  duration_ms?: number | null;
  thread?: string | null;
  message?: string;
  sampling_type?: string;
};

export function JfrAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [selectedFile, setSelectedFile] = useState<FileDockSelection | null>(null);
  const [mode, setMode] = useState<JfrAnalysisMode>("all");
  const [fromTime, setFromTime] = useState("");
  const [toTime, setToTime] = useState("");
  const [stateFilter, setStateFilter] = useState("");
  const [minDurationMs, setMinDurationMs] = useState("");
  const [topN, setTopN] = useState(20);
  const [state, setState] = useState<AnalyzerState>("idle");
  const [error, setError] = useState<BridgeError | null>(null);
  const [result, setResult] = useState<JfrAnalysisResult | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);

  // Native memory analysis runs against the same JFR file but uses a
  // different analyzer endpoint, so it has its own state.
  const [nativeMemLeakOnly, setNativeMemLeakOnly] = useState(true);
  const [nativeMemTailRatio, setNativeMemTailRatio] = useState(0.10);
  const [nativeMemState, setNativeMemState] = useState<AnalyzerState>("idle");
  const [nativeMemError, setNativeMemError] = useState<BridgeError | null>(null);
  const [nativeMemResult, setNativeMemResult] = useState<{
    summary: Record<string, unknown>;
    topSites: Array<{ stack: string; bytes: number }>;
    flame: FlameGraphNode | null;
  } | null>(null);

  const currentRequestIdRef = useRef<string | null>(null);

  const filePath = selectedFile?.filePath ?? "";
  const canAnalyze = Boolean(filePath) && state !== "running";

  const handleFileSelected = useCallback((file: FileDockSelection) => {
    setSelectedFile(file);
    setResult(null);
    setEngineMessages([]);
    setState("ready");
    setError(null);
  }, []);

  const handleClearFile = useCallback(() => {
    setSelectedFile(null);
    setResult(null);
    setEngineMessages([]);
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
      const minDuration = minDurationMs.trim() ? Number(minDurationMs) : undefined;
      const response = await getAnalyzerClient().execute({
        requestId,
        type: "jfr_recording",
        params: {
          requestId,
          filePath,
          topN,
          mode,
          fromTime: fromTime.trim() || undefined,
          toTime: toTime.trim() || undefined,
          state: stateFilter.trim() || undefined,
          minDurationMs: typeof minDuration === "number" && !Number.isNaN(minDuration) ? minDuration : undefined,
        },
      });
      if (currentRequestIdRef.current !== requestId) return;
      currentRequestIdRef.current = null;
      if (response.ok) {
        setResult(response.result as JfrAnalysisResult);
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

  async function runNativeMemory(): Promise<void> {
    if (!filePath || nativeMemState === "running") return;
    setNativeMemState("running");
    setNativeMemError(null);
    const requestId = createAnalyzerRequestId();
    try {
      const response = await getAnalyzerClient().execute({
        requestId,
        type: "native_memory",
        params: {
          requestId,
          filePath,
          leakOnly: nativeMemLeakOnly,
          tailRatio: nativeMemTailRatio,
        },
      });
      if (response.ok) {
        const r = response.result as {
          summary: Record<string, unknown>;
          tables: Record<string, unknown>;
          charts?: { flamegraph?: unknown };
        };
        const flameNode = r.charts?.flamegraph as RawFlameNode | undefined;
        setNativeMemResult({
          summary: r.summary ?? {},
          topSites: ((r.tables?.top_call_sites as Array<{ stack: string; bytes: number }>) ?? []),
          flame: flameNode ? toFlameGraphFromEngine(flameNode) : null,
        });
        setNativeMemState("success");
      } else {
        setNativeMemError(response.error);
        setNativeMemState("error");
      }
    } catch (caught) {
      setNativeMemError({
        code: "IPC_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
      setNativeMemState("error");
    }
  }

  const summary = result?.summary as JfrSummary | undefined;
  const metadata = result?.metadata as JfrMetadata | undefined;
  const eventTypeRows = useMemo<JfrEventTypeRow[]>(() => {
    const series = result?.series as
      | Record<string, JfrEventTypeRow[] | undefined>
      | undefined;
    return (series?.events_by_type as JfrEventTypeRow[] | undefined) ?? [];
  }, [result]);
  const heatmap = useMemo<HeatmapStripData | null>(() => {
    const series = result?.series as Record<string, unknown> | undefined;
    const value = series?.heatmap_strip as HeatmapStripData | undefined;
    return value ?? null;
  }, [result]);
  const notableRows = useMemo<NotableEventRow[]>(() => {
    const tables = result?.tables as
      | Record<string, NotableEventRow[] | undefined>
      | undefined;
    return (tables?.notable_events ?? []) as NotableEventRow[];
  }, [result]);
  const availableModes = metadata?.available_modes ?? [];

  return (
    <div className="flex flex-col gap-5">
      <FileDock
        label={t("selectJfrJsonFile")}
        description={t("jfrFileDescription")}
        accept=".jfr,.json"
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
        <CardContent className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("jfrModeLabel")}</span>
            <select
              className="h-8 rounded-md border border-border bg-background px-2 text-sm"
              value={mode}
              onChange={(e) => setMode(e.target.value as JfrAnalysisMode)}
            >
              {MODE_OPTIONS.map((option) => {
                const enabled =
                  availableModes.length === 0 ||
                  option.value === "all" ||
                  availableModes.includes(option.value);
                return (
                  <option key={option.value} value={option.value} disabled={!enabled}>
                    {t(option.labelKey)}
                    {!enabled ? " (—)" : ""}
                  </option>
                );
              })}
            </select>
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("jfrFromTimeLabel")}</span>
            <Input
              type="text"
              placeholder={t("jfrTimeHint")}
              value={fromTime}
              onChange={(event) => setFromTime(event.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("jfrToTimeLabel")}</span>
            <Input
              type="text"
              placeholder={t("jfrTimeHint")}
              value={toTime}
              onChange={(event) => setToTime(event.target.value)}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("topN")}</span>
            <Input
              type="number"
              min={1}
              value={topN}
              onChange={(event) => setTopN(Number(event.target.value) || 20)}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("jfrStateLabel")}</span>
            <select
              className="h-8 rounded-md border border-border bg-background px-2 text-sm"
              value={stateFilter}
              onChange={(e) => setStateFilter(e.target.value)}
            >
              <option value="">{t("jfrStateAll")}</option>
              {(metadata?.available_states ?? []).map((s) => (
                <option key={s} value={s}>
                  {s}
                </option>
              ))}
            </select>
          </label>
          <label className="flex flex-col gap-1.5 text-xs">
            <span className="font-medium text-foreground/80">{t("jfrLatencyLabel")}</span>
            <Input
              type="number"
              min={0}
              step="0.1"
              placeholder={t("jfrLatencyHint")}
              value={minDurationMs}
              onChange={(event) => setMinDurationMs(event.target.value)}
            />
          </label>
        </CardContent>
      </Card>

      <ErrorPanel
        error={error}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />
      <EngineMessagesPanel messages={engineMessages} title={t("engineMessages")} />

      {summary && (
        <section className="grid grid-cols-2 gap-3 md:grid-cols-4">
          <Metric label={t("jfrEventCountFiltered")} value={summary.event_count} />
          <Metric label={t("jfrEventCountTotal")} value={summary.event_count_total} />
          <Metric label={t("jfrSelectedMode")} value={summary.selected_mode ?? mode} />
          <Metric
            label={t("jfrAvailableModes")}
            value={(metadata?.available_modes ?? []).filter((m) => m !== "all").join(", ") || "—"}
          />
        </section>
      )}

      <Tabs defaultValue="events" className="w-full">
        <TabsList>
          <TabsTrigger value="events">{t("tabEvents")}</TabsTrigger>
          <TabsTrigger value="heatmap">{t("jfrHeatmap")}</TabsTrigger>
          <TabsTrigger value="nativemem">{t("jfrNativeMemTab")}</TabsTrigger>
          <TabsTrigger value="breakdown">{t("jfrEventTypeBreakdown")}</TabsTrigger>
          <TabsTrigger value="diagnostics">{t("tabDiagnostics")}</TabsTrigger>
        </TabsList>

        <TabsContent value="events" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("tabEvents")}</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto p-0">
              {notableRows.length === 0 ? (
                <p className="px-6 py-6 text-center text-sm text-muted-foreground">—</p>
              ) : (
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-[11px] uppercase tracking-wider text-muted-foreground">
                      <th className="w-[180px] px-3 py-2 text-left font-medium">
                        {t("exceptionEventTime")}
                      </th>
                      <th className="w-[200px] px-3 py-2 text-left font-medium">
                        {t("exceptionEventType")}
                      </th>
                      <th className="w-[120px] px-3 py-2 text-right font-medium">
                        {t("pauseMsAxis")}
                      </th>
                      <th className="px-3 py-2 text-left font-medium">
                        {t("exceptionEventMessage")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {notableRows.map((row, index) => (
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
        </TabsContent>

        <TabsContent value="heatmap" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("jfrHeatmap")}</CardTitle>
              <p className="text-xs text-muted-foreground">{t("jfrHeatmapHint")}</p>
            </CardHeader>
            <CardContent>
              {heatmap && heatmap.buckets.length > 0 ? (
                <D3HeatmapStrip
                  buckets={heatmap.buckets}
                  bucketSeconds={heatmap.bucket_seconds}
                  maxCount={heatmap.max_count}
                  startTime={heatmap.start_time}
                  endTime={heatmap.end_time}
                  emptyLabel={t("flameGraphEmpty")}
                  onSelectionChange={(range) => {
                    if (!range) {
                      setFromTime("");
                      setToTime("");
                      return;
                    }
                    setFromTime(range.start);
                    setToTime(range.end);
                  }}
                />
              ) : (
                <p className="py-6 text-center text-sm text-muted-foreground">—</p>
              )}
              <p className="mt-2 text-[11px] text-muted-foreground">
                {t("jfrHeatmapApplyHint")}
              </p>
            </CardContent>
          </Card>
        </TabsContent>

        <TabsContent value="nativemem" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("jfrNativeMemTitle")}</CardTitle>
              <p className="text-xs text-muted-foreground">{t("jfrNativeMemDesc")}</p>
            </CardHeader>
            <CardContent className="flex flex-wrap items-center gap-3 text-xs">
              <label className="flex cursor-pointer items-center gap-1.5">
                <input
                  type="checkbox"
                  checked={nativeMemLeakOnly}
                  onChange={(e) => setNativeMemLeakOnly(e.target.checked)}
                />
                {t("jfrNativeMemLeakOnly")}
              </label>
              <label className="flex items-center gap-2">
                <span className="text-muted-foreground">{t("jfrNativeMemTailRatio")}</span>
                <select
                  className="h-7 rounded-md border border-border bg-background px-2 text-xs"
                  value={String(nativeMemTailRatio)}
                  onChange={(e) => setNativeMemTailRatio(Number(e.target.value))}
                >
                  <option value="0">0%</option>
                  <option value="0.05">5%</option>
                  <option value="0.1">10%</option>
                  <option value="0.2">20%</option>
                  <option value="0.3">30%</option>
                </select>
                <span className="text-[10px] text-muted-foreground">{t("jfrNativeMemTailHint")}</span>
              </label>
              <Button
                type="button"
                size="sm"
                disabled={!filePath || nativeMemState === "running"}
                onClick={() => void runNativeMemory()}
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
            </CardContent>
          </Card>

          <ErrorPanel
            error={nativeMemError}
            labels={{ title: t("analysisError"), code: t("errorCode") }}
          />

          {nativeMemResult ? (
            <div className="mt-4 grid gap-4">
              <section className="grid grid-cols-2 gap-3 md:grid-cols-5">
                <Metric
                  label={t("jfrNativeMemAllocCount")}
                  value={nativeMemResult.summary.alloc_event_count as number}
                />
                <Metric
                  label={t("jfrNativeMemFreeCount")}
                  value={nativeMemResult.summary.free_event_count as number}
                />
                <Metric
                  label={t("jfrNativeMemAllocBytes")}
                  value={formatBytes(nativeMemResult.summary.alloc_bytes_total as number)}
                />
                <Metric
                  label={t("jfrNativeMemUnfreedCount")}
                  value={nativeMemResult.summary.unfreed_event_count as number}
                />
                <Metric
                  label={t("jfrNativeMemUnfreedBytes")}
                  value={formatBytes(nativeMemResult.summary.unfreed_bytes_total as number)}
                />
              </section>

              {nativeMemResult.flame ? (
                <D3FlameGraph
                  title={t("jfrNativeMemTitle")}
                  exportName="native-memory-flamegraph"
                  data={nativeMemResult.flame}
                  emptyLabel="—"
                />
              ) : null}

              <Card>
                <CardHeader className="pb-3">
                  <CardTitle className="text-sm">{t("jfrNativeMemTopSites")}</CardTitle>
                </CardHeader>
                <CardContent className="overflow-x-auto p-0">
                  {nativeMemResult.topSites.length === 0 ? (
                    <p className="px-6 py-6 text-center text-sm text-muted-foreground">—</p>
                  ) : (
                    <table className="w-full text-xs">
                      <thead>
                        <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                          <th className="px-3 py-2 text-left font-medium">
                            {t("jfrNativeMemSiteStack")}
                          </th>
                          <th className="w-[120px] px-3 py-2 text-right font-medium">
                            {t("jfrNativeMemSiteBytes")}
                          </th>
                        </tr>
                      </thead>
                      <tbody>
                        {nativeMemResult.topSites.slice(0, 20).map((row, idx) => (
                          <tr key={`${row.stack}-${idx}`} className="border-b border-border last:border-0">
                            <td className="max-w-[480px] truncate px-3 py-2 font-mono" title={row.stack}>
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
            </div>
          ) : null}
        </TabsContent>

        <TabsContent value="breakdown" className="mt-4">
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="text-sm">{t("jfrEventTypeBreakdown")}</CardTitle>
            </CardHeader>
            <CardContent className="p-0">
              {eventTypeRows.length === 0 ? (
                <p className="px-6 py-6 text-center text-sm text-muted-foreground">—</p>
              ) : (
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-[11px] uppercase tracking-wider text-muted-foreground">
                      <th className="px-3 py-2 text-left font-medium">
                        {t("exceptionEventType")}
                      </th>
                      <th className="w-[120px] px-3 py-2 text-right font-medium">
                        {t("count")}
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {eventTypeRows.map((row) => (
                      <tr
                        key={row.event_type}
                        className="border-b border-border last:border-0"
                      >
                        <td className="px-3 py-2 font-mono text-xs">{row.event_type}</td>
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
        </TabsContent>

        <TabsContent value="diagnostics" className="mt-4">
          <DiagnosticsPanel
            diagnostics={metadata?.diagnostics as never}
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

function Metric({
  label,
  value,
}: {
  label: string;
  value: string | number | undefined | null;
}): JSX.Element {
  return (
    <div className="min-w-0 overflow-hidden rounded-md border border-border bg-muted/30 px-3 py-2 text-xs">
      <div className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
        {label}
      </div>
      <div
        className="mt-1 truncate text-sm tabular-nums"
        title={value == null ? "—" : String(value)}
      >
        {value == null || value === "" ? "—" : typeof value === "number"
          ? value.toLocaleString()
          : value}
      </div>
    </div>
  );
}

type RawFlameNode = {
  name: string;
  samples: number;
  category?: string | null;
  color?: string | null;
  metadata?: Record<string, unknown> | null;
  children?: RawFlameNode[] | null;
};

function toFlameGraphFromEngine(node: RawFlameNode): FlameGraphNode {
  return {
    name: node.name,
    value: node.samples,
    category: node.category ?? null,
    color: node.color ?? null,
    metadata: node.metadata ?? null,
    children: Array.isArray(node.children)
      ? node.children.map(toFlameGraphFromEngine)
      : null,
  };
}

function formatBytes(value: number | undefined): string {
  if (value == null || !Number.isFinite(value)) return "—";
  const v = Number(value);
  if (v < 1024) return `${v} B`;
  if (v < 1024 * 1024) return `${(v / 1024).toFixed(1)} KB`;
  if (v < 1024 * 1024 * 1024) return `${(v / 1024 / 1024).toFixed(1)} MB`;
  return `${(v / 1024 / 1024 / 1024).toFixed(2)} GB`;
}
