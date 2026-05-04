import {
  AlertTriangle,
  Camera,
  FileText,
  Layers,
  Loader2,
  Play,
  Square,
  X,
} from "lucide-react";
import { useCallback, useMemo, useRef, useState } from "react";

import {
  createAnalyzerRequestId,
  getAnalyzerClient,
  type AnalysisResult,
  type BridgeError,
} from "@/api/analyzerClient";
import {
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { FileDock, type FileDockSelection } from "@/components/FileDock";
import { D3BarChart, type BarDatum } from "@/components/charts/D3BarChart";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useI18n } from "@/i18n/I18nProvider";
import { exportChartsInContainer } from "@/lib/batchExport";
import { cn } from "@/lib/utils";

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";

type LongRunningFindingRow = {
  thread_name: string;
  stack_signature: string;
  dumps: number;
  first_dump_index: number;
};

type PersistentBlockedFindingRow = {
  thread_name: string;
  dumps: number;
  first_dump_index: number;
  stack_signatures: string[];
};

type EngineFinding = {
  severity: "info" | "warning" | "critical";
  code: string;
  message: string;
};

type ThreadDumpMultiSummary = {
  total_dumps: number;
  unique_threads: number;
  long_running_threads: number;
  persistent_blocked_threads: number;
  languages_detected: string[];
  source_formats: string[];
  consecutive_dump_threshold: number;
};

type StatePerDumpRow = {
  dump_index: number;
  dump_label: string | null;
  counts: Record<string, number>;
};

type ThreadPersistenceRow = {
  thread_name: string;
  observed_in_dumps: number;
};

type DumpRow = {
  dump_index: number;
  dump_label: string | null;
  source_file: string;
  source_format: string;
  language: string;
  thread_count: number;
};

type LockRow = {
  lock_id: string;
  lock_class: string | null;
  owner_thread: string | null;
  owner_stack_signature: string | null;
  owner_count: number;
  waiter_count: number;
  top_waiters: string[];
  all_waiters: string[];
};

type DeadlockChain = {
  threads: string[];
  edges: Array<{
    from_thread: string;
    to_thread: string;
    lock_id: string | null;
    lock_class: string | null;
  }>;
};

type LockContentionResult = {
  summary: {
    contended_locks: number;
    deadlocks_detected: number;
    threads_with_locks: number;
    threads_waiting_on_lock: number;
    unique_locks: number;
  };
  tables: {
    locks: LockRow[];
    deadlock_chains: DeadlockChain[];
  };
};

const SEVERITY_COLORS: Record<EngineFinding["severity"], string> = {
  critical: "bg-destructive/10 text-destructive border-destructive/30",
  warning: "bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/30",
  info: "bg-sky-500/10 text-sky-700 dark:text-sky-400 border-sky-500/30",
};

export function ThreadDumpAnalyzerPage(): JSX.Element {
  const { t } = useI18n();
  const [files, setFiles] = useState<FileDockSelection[]>([]);
  const [pending, setPending] = useState<FileDockSelection | null>(null);
  const [threshold, setThreshold] = useState(3);
  const [formatOverride, setFormatOverride] = useState("");
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<AnalysisResult | null>(null);
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  const [savingAll, setSavingAll] = useState(false);
  const [lockResult, setLockResult] = useState<LockContentionResult | null>(null);
  const currentRequestIdRef = useRef<string | null>(null);
  const chartsContainerRef = useRef<HTMLDivElement | null>(null);

  const canAnalyze = files.length > 0 && state !== "running";

  const handleFileSelected = useCallback(
    (file: FileDockSelection) => {
      // Avoid double-adding the exact same upload (same server path).
      setFiles((prev) =>
        prev.some((existing) => existing.filePath === file.filePath)
          ? prev
          : [...prev, file],
      );
      setPending(null);
      setState("ready");
      setError(null);
    },
    [],
  );

  const removeFile = useCallback((filePath: string) => {
    setFiles((prev) => prev.filter((file) => file.filePath !== filePath));
  }, []);

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
        type: "thread_dump_multi",
        params: {
          requestId,
          filePaths: files.map((file) => file.filePath),
          consecutiveThreshold: threshold,
          format: formatOverride.trim() || undefined,
          topN: 20,
        },
      });

      if (currentRequestIdRef.current !== requestId) return;
      currentRequestIdRef.current = null;

      if (response.ok) {
        setResult(response.result);
        setEngineMessages(response.engine_messages ?? []);
        setState("success");
        // T-223: kick off the lock-contention analysis in parallel so
        // the dedicated tab populates without a second click.
        void runLockAnalysis();
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

  async function runLockAnalysis(): Promise<void> {
    if (files.length === 0) return;
    try {
      const response = await getAnalyzerClient().execute({
        type: "thread_dump_locks",
        params: {
          filePaths: files.map((file) => file.filePath),
          format: formatOverride.trim() || undefined,
          topN: 20,
        },
      });
      if (response.ok) {
        setLockResult(response.result as unknown as LockContentionResult);
      }
    } catch {
      // ignore: the multi-dump tab still renders even if lock analysis fails
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
        prefix: "thread-dump-multi",
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

  const summary = (result?.summary as unknown as ThreadDumpMultiSummary | undefined) ?? null;
  const findings = (result?.metadata?.findings as EngineFinding[] | undefined) ?? [];
  const longRunning = (result?.tables?.long_running_stacks as
    | LongRunningFindingRow[]
    | undefined) ?? [];
  const persistentBlocked = (result?.tables?.persistent_blocked_threads as
    | PersistentBlockedFindingRow[]
    | undefined) ?? [];
  const dumpsTable = (result?.tables?.dumps as DumpRow[] | undefined) ?? [];
  const statePerDump = (result?.series?.state_distribution_per_dump as
    | StatePerDumpRow[]
    | undefined) ?? [];
  const threadPersistence = (result?.series?.thread_persistence as
    | ThreadPersistenceRow[]
    | undefined) ?? [];

  const threadsPerDumpBars = useMemo<BarDatum[]>(() => {
    return dumpsTable.map((row) => ({
      id: `dump-${row.dump_index}`,
      label: row.dump_label ?? `dump-${row.dump_index}`,
      value: row.thread_count,
    }));
  }, [dumpsTable]);

  const persistenceBars = useMemo<BarDatum[]>(() => {
    return threadPersistence.slice(0, 12).map((row) => ({
      id: row.thread_name,
      label: row.thread_name,
      value: row.observed_in_dumps,
    }));
  }, [threadPersistence]);

  return (
    <div className="flex flex-col gap-5">
      <FileDock
        label={t("addThreadDumpFile")}
        description={t("dropOrBrowseThreadDump")}
        accept=".txt,.log,.dump"
        selected={pending}
        onSelect={handleFileSelected}
        onClear={() => setPending(null)}
        browseLabel={t("addThreadDumpFile")}
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
                  {t("runMultiDumpAnalysis")}
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
        <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-3">
          <div>
            <CardTitle className="text-sm">
              {t("threadDumpsSelected")} ({files.length})
            </CardTitle>
            <CardDescription>{t("multiDumpEmpty")}</CardDescription>
          </div>
          <div className="flex items-center gap-2">
            <label className="flex items-center gap-1.5 text-xs">
              <span className="font-medium text-foreground/80">
                {t("consecutiveThreshold")}
              </span>
              <Input
                type="number"
                min={1}
                value={threshold}
                onChange={(event) => setThreshold(Math.max(1, Number(event.target.value) || 1))}
                className="h-8 w-16"
              />
            </label>
            <label className="flex items-center gap-1.5 text-xs">
              <span className="font-medium text-foreground/80">
                {t("formatOverride")}
              </span>
              <Input
                type="text"
                placeholder="java_jstack"
                value={formatOverride}
                onChange={(event) => setFormatOverride(event.target.value)}
                className="h-8 w-32"
              />
            </label>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          {files.length === 0 ? (
            <div className="px-6 pb-6 text-sm text-muted-foreground">
              {t("multiDumpEmpty")}
            </div>
          ) : (
            <ul className="divide-y divide-border">
              {files.map((file, index) => (
                <li
                  key={file.filePath}
                  className="flex items-center gap-3 px-4 py-2.5 text-sm"
                >
                  <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-muted text-[10px] font-semibold tabular-nums text-muted-foreground">
                    {index}
                  </span>
                  <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                  <span className="flex-1 truncate font-mono text-xs">
                    {file.originalName}
                  </span>
                  <span className="shrink-0 text-[11px] tabular-nums text-muted-foreground">
                    {(file.size / 1024).toFixed(1)} KB
                  </span>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    aria-label={t("removeFile")}
                    onClick={() => removeFile(file.filePath)}
                  >
                    <X className="h-3 w-3" />
                  </Button>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <ErrorPanel
        error={error}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />
      <EngineMessagesPanel messages={engineMessages} title={t("engineMessages")} />

      {result && summary && (
        <Tabs defaultValue="findings" className="w-full">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <TabsList>
              <TabsTrigger value="findings">{t("tabFindings")}</TabsTrigger>
              <TabsTrigger value="charts">{t("tabCharts")}</TabsTrigger>
              <TabsTrigger value="per-dump">{t("tabPerDump")}</TabsTrigger>
              <TabsTrigger value="threads">{t("tabThreads")}</TabsTrigger>
              <TabsTrigger value="locks">{t("tabLockContention")}</TabsTrigger>
            </TabsList>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={savingAll}
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

          <TabsContent value="findings" className="mt-4 space-y-4">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">
                  {t("threadDumpMultiSummary")}
                </CardTitle>
              </CardHeader>
              <CardContent className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
                <SummaryStat
                  label={t("totalEvents")}
                  value={summary.total_dumps}
                />
                <SummaryStat
                  label="unique threads"
                  value={summary.unique_threads}
                />
                <SummaryStat
                  label={t("longRunningThreadsLabel")}
                  value={summary.long_running_threads}
                  emphasized={summary.long_running_threads > 0}
                />
                <SummaryStat
                  label={t("persistentBlockedThreadsLabel")}
                  value={summary.persistent_blocked_threads}
                  emphasized={summary.persistent_blocked_threads > 0}
                />
                <SummaryStat
                  label="languages"
                  value={summary.languages_detected.join(", ") || "—"}
                />
                <SummaryStat
                  label="threshold"
                  value={summary.consecutive_dump_threshold}
                />
              </CardContent>
            </Card>

            {findings.length > 0 ? (
              <Card>
                <CardHeader className="pb-3">
                  <CardTitle className="text-sm">{t("tabFindings")}</CardTitle>
                </CardHeader>
                <CardContent className="space-y-2">
                  {findings.map((finding, index) => (
                    <div
                      key={`${finding.code}-${index}`}
                      className={cn(
                        "rounded-md border px-3 py-2 text-xs",
                        SEVERITY_COLORS[finding.severity],
                      )}
                    >
                      <div className="flex items-center gap-2 font-semibold">
                        <AlertTriangle className="h-3.5 w-3.5" />
                        <span className="font-mono uppercase">{finding.code}</span>
                        <span className="ml-auto text-[10px] uppercase tracking-wide opacity-70">
                          {t(
                            finding.severity === "critical"
                              ? "findingSeverityCritical"
                              : finding.severity === "warning"
                              ? "findingSeverityWarning"
                              : "findingSeverityInfo",
                          )}
                        </span>
                      </div>
                      <p className="mt-1 leading-relaxed">{finding.message}</p>
                    </div>
                  ))}
                </CardContent>
              </Card>
            ) : (
              <Card>
                <CardContent className="flex items-center gap-2 px-4 py-3 text-sm text-muted-foreground">
                  <Layers className="h-4 w-4" />
                  No long-running or persistently blocked threads detected at threshold{" "}
                  {summary.consecutive_dump_threshold}.
                </CardContent>
              </Card>
            )}

            {longRunning.length > 0 && (
              <FindingTable
                title={t("longRunningThreadsLabel")}
                rows={longRunning}
                columns={["thread_name", "dumps", "first_dump_index", "stack_signature"]}
                renderCell={(row, key) => {
                  const value = (row as Record<string, unknown>)[key];
                  if (key === "stack_signature") {
                    return <span className="font-mono text-[11px]">{String(value)}</span>;
                  }
                  return <span className="tabular-nums">{String(value)}</span>;
                }}
              />
            )}

            {persistentBlocked.length > 0 && (
              <FindingTable
                title={t("persistentBlockedThreadsLabel")}
                rows={persistentBlocked}
                columns={["thread_name", "dumps", "first_dump_index", "stack_signatures"]}
                renderCell={(row, key) => {
                  const value = (row as Record<string, unknown>)[key];
                  if (key === "stack_signatures" && Array.isArray(value)) {
                    return (
                      <span className="font-mono text-[11px]">
                        {value.join(" | ")}
                      </span>
                    );
                  }
                  return <span className="tabular-nums">{String(value)}</span>;
                }}
              />
            )}
          </TabsContent>

          <TabsContent value="charts" className="mt-4">
            <div ref={chartsContainerRef} className="grid gap-4">
              <D3BarChart
                title={t("threadsPerDumpChart")}
                exportName="threads-per-dump"
                data={threadsPerDumpBars}
                orientation="vertical"
                height={240}
              />
              {persistenceBars.length > 0 && (
                <D3BarChart
                  title={t("threadPersistenceChart")}
                  exportName="thread-persistence"
                  data={persistenceBars}
                  orientation="horizontal"
                  sort
                  height={Math.max(180, persistenceBars.length * 26 + 40)}
                />
              )}
            </div>
          </TabsContent>

          <TabsContent value="per-dump" className="mt-4">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">{t("statePerDumpChart")}</CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                      <th className="px-3 py-2 text-left font-medium">dump</th>
                      <th className="px-3 py-2 text-left font-medium">label</th>
                      <th className="px-3 py-2 text-left font-medium">states</th>
                    </tr>
                  </thead>
                  <tbody>
                    {statePerDump.map((row) => (
                      <tr
                        key={row.dump_index}
                        className="border-b border-border last:border-0"
                      >
                        <td className="px-3 py-2 tabular-nums">{row.dump_index}</td>
                        <td className="px-3 py-2 font-mono text-xs">
                          {row.dump_label ?? "—"}
                        </td>
                        <td className="px-3 py-2 text-xs">
                          {Object.entries(row.counts)
                            .map(([state, count]) => `${state}=${count}`)
                            .join("  ·  ")}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          </TabsContent>

          <TabsContent value="locks" className="mt-4 space-y-4">
            {!lockResult ? (
              <Card>
                <CardContent className="px-4 py-3 text-sm text-muted-foreground">
                  {t("lockContentionEmpty")}
                </CardContent>
              </Card>
            ) : (
              <LockContentionPanel result={lockResult} />
            )}
          </TabsContent>

          <TabsContent value="threads" className="mt-4">
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-sm">
                  {t("threadPersistenceChart")}
                </CardTitle>
              </CardHeader>
              <CardContent className="overflow-x-auto p-0">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                      <th className="px-3 py-2 text-left font-medium">thread</th>
                      <th className="px-3 py-2 text-right font-medium">
                        observed in dumps
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {threadPersistence.map((row) => (
                      <tr
                        key={row.thread_name}
                        className="border-b border-border last:border-0"
                      >
                        <td className="px-3 py-2 font-mono text-xs">
                          {row.thread_name}
                        </td>
                        <td className="px-3 py-2 text-right tabular-nums">
                          {row.observed_in_dumps}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </CardContent>
            </Card>
          </TabsContent>
        </Tabs>
      )}

    </div>
  );
}

function SummaryStat({
  label,
  value,
  emphasized,
}: {
  label: string;
  value: number | string;
  emphasized?: boolean;
}): JSX.Element {
  return (
    <div
      className={cn(
        "rounded-md border border-border bg-muted/30 px-3 py-2 text-xs",
        emphasized && "border-amber-500/40 bg-amber-500/10",
      )}
    >
      <p className="text-[10px] font-semibold uppercase tracking-wider text-muted-foreground">
        {label}
      </p>
      <p className="mt-1 text-base font-semibold tabular-nums">{value}</p>
    </div>
  );
}

function LockContentionPanel({
  result,
}: {
  result: LockContentionResult;
}): JSX.Element {
  const { t } = useI18n();
  const summary = result.summary;
  const rows = result.tables.locks ?? [];
  const deadlocks = result.tables.deadlock_chains ?? [];
  const rankingBars = useMemo<BarDatum[]>(
    () =>
      rows
        .filter((row) => row.waiter_count > 0)
        .slice(0, 10)
        .map((row) => ({
          id: row.lock_id,
          label: row.lock_class || row.lock_id.slice(-10),
          value: row.waiter_count,
        })),
    [rows],
  );

  return (
    <>
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">
            {t("lockContentionRanking")}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-2 gap-3 sm:grid-cols-5">
          <SummaryStat label="contended" value={summary.contended_locks} />
          <SummaryStat
            label={t("deadlockDetected")}
            value={summary.deadlocks_detected}
            emphasized={summary.deadlocks_detected > 0}
          />
          <SummaryStat label="with locks" value={summary.threads_with_locks} />
          <SummaryStat label="waiting" value={summary.threads_waiting_on_lock} />
          <SummaryStat label="unique" value={summary.unique_locks} />
        </CardContent>
      </Card>

      {deadlocks.length > 0 && (
        <Card className="border-destructive/40 bg-destructive/5">
          <CardHeader className="pb-3">
            <CardTitle className="text-sm text-destructive">
              {t("deadlockDetected")} ({deadlocks.length})
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {deadlocks.map((chain, index) => (
              <div
                key={index}
                className="rounded-md border border-destructive/30 bg-background/40 px-3 py-2 text-xs"
              >
                <p className="font-mono font-semibold">
                  {t("deadlockCycle")}:{" "}
                  {[...chain.threads, chain.threads[0]].join(" → ")}
                </p>
                <ul className="mt-1 space-y-0.5 text-[11px] text-muted-foreground">
                  {chain.edges.map((edge, edgeIndex) => (
                    <li key={edgeIndex} className="font-mono">
                      {edge.from_thread} → {edge.to_thread} (
                      {edge.lock_id ?? "?"}
                      {edge.lock_class ? ` · ${edge.lock_class}` : ""})
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </CardContent>
        </Card>
      )}

      {rankingBars.length > 0 && (
        <D3BarChart
          title={t("lockContentionRanking")}
          exportName="lock-contention-ranking"
          data={rankingBars}
          orientation="horizontal"
          sort
          height={Math.max(180, rankingBars.length * 28 + 40)}
        />
      )}

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-sm">{t("tabLockContention")}</CardTitle>
        </CardHeader>
        <CardContent className="overflow-x-auto p-0">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
                <th className="px-3 py-2 text-left font-medium">
                  {t("lockIdLabel")}
                </th>
                <th className="px-3 py-2 text-left font-medium">
                  {t("lockClassLabel")}
                </th>
                <th className="px-3 py-2 text-left font-medium">
                  {t("lockOwnerThread")}
                </th>
                <th className="px-3 py-2 text-right font-medium">
                  {t("lockWaiterCount")}
                </th>
                <th className="px-3 py-2 text-left font-medium">top waiters</th>
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 ? (
                <tr>
                  <td className="px-3 py-3 text-muted-foreground" colSpan={5}>
                    —
                  </td>
                </tr>
              ) : (
                rows.map((row) => (
                  <tr
                    key={row.lock_id}
                    className="border-b border-border last:border-0"
                  >
                    <td className="px-3 py-2 font-mono text-[11px]">
                      {row.lock_id}
                    </td>
                    <td className="px-3 py-2 font-mono text-[11px]">
                      {row.lock_class ?? "—"}
                    </td>
                    <td className="px-3 py-2 text-xs">
                      {row.owner_thread ?? "—"}
                    </td>
                    <td className="px-3 py-2 text-right tabular-nums">
                      {row.waiter_count}
                    </td>
                    <td className="px-3 py-2 text-[11px] text-muted-foreground">
                      {row.top_waiters.join(", ") || "—"}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </CardContent>
      </Card>
    </>
  );
}


function FindingTable<Row>({
  title,
  rows,
  columns,
  renderCell,
}: {
  title: string;
  rows: Row[];
  columns: string[];
  renderCell: (row: Row, column: string) => JSX.Element;
}): JSX.Element {
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="text-sm">{title}</CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border bg-muted/40 text-xs text-muted-foreground">
              {columns.map((column) => (
                <th
                  key={column}
                  className="px-3 py-2 text-left font-medium uppercase tracking-wide"
                >
                  {column.replace(/_/g, " ")}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {rows.map((row, index) => (
              <tr
                key={index}
                className="border-b border-border last:border-0"
              >
                {columns.map((column) => (
                  <td key={column} className="px-3 py-2 align-top">
                    {renderCell(row, column)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}
