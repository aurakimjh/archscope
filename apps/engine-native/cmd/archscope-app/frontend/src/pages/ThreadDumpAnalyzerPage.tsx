// ─────────────────────────────────────────────────────────────────────
// [한글] pages/ThreadDumpAnalyzerPage.tsx — 스레드 덤프 통합 분석기 페이지.
//
// 책임/목적:
//   - 단일/다중 스레드 덤프(Java jstack, jcmd JSON, Python py-spy,
//     Node.js report, Go pprof, .NET clrstack) 입력을 받아 세 가지
//     분석 모드로 호출:
//       (1) engine.analyzeThreadDump   → ThreadDumpSingleResult
//                                         (sync, type "thread_dump")
//       (2) engine.analyzeMultiThread  → ThreadDumpMultiResult
//                                         (async, type "thread_dump_multi")
//       (3) engine.analyzeLockContention → LockContentionResult
//                                         (async, type "lock_contention")
//
//   - 비동기 모드는 EngineService 가 emit 하는 Wails 이벤트
//     (engine:done / engine:error / engine:cancelled) 를 listen 하여
//     결과/에러/취소를 받아 처리. cancelEngineTask(taskId) 로 중단 가능.
//
// 다루는 결과 형식:
//   - 단일 덤프: ThreadDumpSingleResult — state/category/signature 분포 +
//     thread 목록.
//   - 다중 덤프: ThreadDumpMultiResult — long-running 스택, persistent
//     blocked, virtual thread carrier pinning, SMR unresolved,
//     class histogram 등 멀티 덤프 상관 분석.
//   - Lock contention: LockContentionResult — lock 별 owner/waiter +
//     deadlock chains.
//
// 데이터 흐름 (async path):
//   engine.analyzeMultiThread(req) → { taskId } 즉시 반환
//   Events.On("engine:done")  → 결과 setResult.
//   Events.On("engine:error") → setError.
//   Events.On("engine:cancelled") → idle 상태로 리셋.
//   Cancel button → engine.cancelEngineTask(taskId).
//
// 의존성 주의: @wailsio/runtime Events 구독은 컴포넌트 unmount 시
// off() 처리 필요(메모리 누수 방지). taskId 는 useRef 로 보관해 stale
// closure 문제를 회피.
// ─────────────────────────────────────────────────────────────────────
import {
  AlertTriangle,
  FileText,
  Layers,
  Loader2,
  Play,
  Square,
  Upload,
  X,
} from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Dialogs, Events } from "@wailsio/runtime";

import { engine } from "@/bridge/engine";
import type {
  BridgeError,
  CarrierPinningRow,
  ClassHistogramRow,
  DeadlockChain,
  DumpRow,
  EngineCancelledEvent,
  EngineDoneEvent,
  EngineErrorEvent,
  LockContentionRankingRow,
  LockContentionResult,
  LockContentionRow,
  LongRunningStackRow,
  NativeMethodRow,
  PersistentBlockedRow,
  SmrUnresolvedRow,
  StatePerDumpRow,
  ThreadDumpCategoryRow,
  ThreadDumpFinding,
  ThreadDumpMultiResult,
  ThreadDumpMultiSummary,
  ThreadDumpSignatureRow,
  ThreadDumpSingleResult,
  ThreadDumpSingleSummary,
  ThreadDumpStateRow,
  ThreadDumpThreadRow,
  ThreadPersistenceRow,
} from "@/bridge/types";
import {
  EngineMessagesPanel,
  ErrorPanel,
} from "@/components/AnalyzerFeedback";
import { AnalyzerOptionsDock } from "@/components/AnalyzerOptionsDock";
import { HelpTip, HelpedLabel } from "@/components/HelpTip";
import { MetricCard } from "@/components/MetricCard";
import {
  WailsFileDock,
  type FileDockSelection,
} from "@/components/WailsFileDock";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
} from "@/components/ui/tabs";
import { getHelpText } from "@/help/helpCatalog";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import { cn } from "@/lib/utils";
import { addWorkspaceResult } from "@/state/analysisWorkspace";
import { formatNumber } from "@/utils/formatters";

// Wails port of apps/frontend/src/pages/ThreadDumpAnalyzerPage.tsx.
//
// Three modes share a single page:
//   1. "single" — engine.analyzeThreadDump (sync, one Java jstack file).
//   2. "multi"  — engine.analyzeMultiThread (async, N files, N-way correlation).
//   3. "locks"  — engine.analyzeLockContention (async, N files).
//
// The async modes use the engine:done / engine:error / engine:cancelled
// event family emitted by engineservice.go. We wire one Events.On
// listener per event type at mount time (not per-call, otherwise we'd
// race against the Go side that emits as soon as the goroutine finishes,
// possibly before our handler is registered) and gate every callback on
// `taskRef.current` matching the inbound taskId.
//
// Multi-file selection uses Wails Dialogs.OpenFile with
// AllowsMultipleSelection: true, which returns a string[]. The web
// FileDock would issue an HTTP multipart upload per file; here we just
// hand the engine the absolute paths.
//
// Behavioural deltas vs. the web original:
//   - The web page persists every successful run to a Dashboard panel.
//     Deferred to a later phase (web-only chartFactory dependency).
//   - The web page kicks off lock-contention analysis in parallel with
//     multi-dump analysis from a single button. Here we keep them under
//     separate mode buttons so the engine task lifecycle is explicit
//     (one taskId at a time per page).
//   - D3BarChart is replaced by inline tables; charts will land alongside
//     the AccessLog ChartPanel ports.

type AnalyzerState = "idle" | "ready" | "running" | "success" | "error";
type AnalysisMode = "single" | "multi" | "locks";

type AnyResult =
  | ThreadDumpSingleResult
  | ThreadDumpMultiResult
  | LockContentionResult;

const SEVERITY_COLORS: Record<string, string> = {
  critical: "bg-destructive/10 text-destructive border-destructive/30",
  warning:
    "bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/30",
  info:
    "bg-sky-500/10 text-sky-700 dark:text-sky-400 border-sky-500/30",
};

const FILE_FILTERS = [
  { displayName: "Thread dumps", pattern: "*.txt;*.log;*.dump;*.json" },
  { displayName: "All files", pattern: "*.*" },
];

export function ThreadDumpAnalyzerPage(): JSX.Element {
  const { locale, t } = useI18n();
  const [mode, setMode] = useState<AnalysisMode>("single");
  const [files, setFiles] = useState<FileDockSelection[]>([]);
  const [singleFile, setSingleFile] = useState<FileDockSelection | null>(
    null,
  );
  const [threshold, setThreshold] = useState(3);
  const [formatOverride, setFormatOverride] = useState("");
  const [topN, setTopN] = useState(20);
  const [state, setState] = useState<AnalyzerState>("idle");
  const [result, setResult] = useState<AnyResult | null>(null);
  const [error, setError] = useState<BridgeError | null>(null);
  const [engineMessages, setEngineMessages] = useState<string[]>([]);
  // Tracks the inflight async taskId (multi/locks). For sync mode we
  // use a Symbol token instead, exactly like AccessLog does.
  const taskRef = useRef<string | null>(null);
  const inflightSyncRef = useRef<symbol | null>(null);

  // ────────────────────────────────────────────────────────────────
  // Async event subscription. The Go side emits engine:done /
  // engine:error / engine:cancelled with a taskId payload; we filter
  // on `taskRef.current` so a stale event from a previously-cancelled
  // run never updates this component's state.
  // ────────────────────────────────────────────────────────────────
  useEffect(() => {
    const offDone = Events.On("engine:done", (event: unknown) => {
      const data = extractData<EngineDoneEvent>(event);
      if (!data || data.taskId !== taskRef.current) return;
      taskRef.current = null;
      setResult(data.result as unknown as AnyResult);
      addWorkspaceResult({
        result: data.result,
        title: `${data.result.type}: ${data.result.source_files?.[0] ?? ""}`,
        sourceLabel: data.result.source_files?.[0],
      });
      setState("success");
    });
    const offError = Events.On("engine:error", (event: unknown) => {
      const data = extractData<EngineErrorEvent>(event);
      if (!data || data.taskId !== taskRef.current) return;
      taskRef.current = null;
      setError({ code: "ENGINE_FAILED", message: data.message });
      setState("error");
    });
    const offCancelled = Events.On(
      "engine:cancelled",
      (event: unknown) => {
        const data = extractData<EngineCancelledEvent>(event);
        if (!data || data.taskId !== taskRef.current) return;
        taskRef.current = null;
        setState("ready");
        setEngineMessages([t("analysisCanceled")]);
      },
    );
    return () => {
      offDone?.();
      offError?.();
      offCancelled?.();
    };
  }, [t]);

  // Resetting common state when mode flips so the single-file picker
  // doesn't bleed into the multi-file list and vice versa.
  const switchMode = useCallback(
    (next: AnalysisMode) => {
      if (next === mode) return;
      setMode(next);
      setResult(null);
      setError(null);
      setEngineMessages([]);
      setState("idle");
    },
    [mode],
  );

  const canAnalyze =
    state !== "running" &&
    (mode === "single" ? Boolean(singleFile?.filePath) : files.length > 0);

  // ────────────────────────────────────────────────────────────────
  // File selection — single + multi pickers.
  // ────────────────────────────────────────────────────────────────

  const handleSingleFile = useCallback((file: FileDockSelection) => {
    setSingleFile(file);
    setState("ready");
    setError(null);
    setEngineMessages([]);
  }, []);

  const handleClearSingle = useCallback(() => {
    setSingleFile(null);
    setState("idle");
  }, []);

  const pickMultipleFiles = useCallback(async () => {
    try {
      const result = await Dialogs.OpenFile({
        Title: t("threadDumpSelectFiles"),
        AllowsMultipleSelection: true,
        Filters: FILE_FILTERS.map((f) => ({
          DisplayName: f.displayName,
          Pattern: f.pattern,
        })),
      });
      const paths = Array.isArray(result) ? result : [];
      if (paths.length === 0) return;
      setFiles((prev) => {
        const seen = new Set(prev.map((entry) => entry.filePath));
        const additions: FileDockSelection[] = [];
        for (const path of paths) {
          if (!path || seen.has(path)) continue;
          seen.add(path);
          additions.push({
            filePath: path,
            originalName: basenameOf(path),
            size: 0,
          });
        }
        const next = additions.length === 0 ? prev : [...prev, ...additions];
        if (next.length > 0 && state === "idle") setState("ready");
        return next;
      });
      setError(null);
      setEngineMessages([]);
    } catch (caught) {
      setError({
        code: "DIALOG_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
    }
  }, [state, t]);

  const removeFile = useCallback((filePath: string) => {
    setFiles((prev) => prev.filter((file) => file.filePath !== filePath));
  }, []);

  const clearFiles = useCallback(() => {
    setFiles([]);
    setState("idle");
  }, []);

  // ────────────────────────────────────────────────────────────────
  // Run / cancel.
  // ────────────────────────────────────────────────────────────────

  const analyze = useCallback(async () => {
    if (!canAnalyze) return;
    setError(null);
    setEngineMessages([]);
    setState("running");

    if (mode === "single" && singleFile) {
      // Sync: the engine returns the result directly. We follow the
      // AccessLog pattern of guarding with a Symbol token so a stale
      // late-arriving promise can't overwrite a fresh selection.
      const token = Symbol("thread-dump-single");
      inflightSyncRef.current = token;
      try {
        const response = await engine.analyzeThreadDump({
          path: singleFile.filePath,
          topN,
        });
        if (inflightSyncRef.current !== token) return;
        inflightSyncRef.current = null;
        const singleResult = response as unknown as ThreadDumpSingleResult;
        setResult(singleResult);
        addWorkspaceResult({
          result: singleResult,
          title: `thread_dump: ${singleFile.originalName}`,
          sourceLabel: singleFile.originalName,
        });
        setState("success");
      } catch (caught) {
        if (inflightSyncRef.current !== token) return;
        inflightSyncRef.current = null;
        setError({
          code: "ENGINE_FAILED",
          message: caught instanceof Error ? caught.message : String(caught),
        });
        setState("error");
      }
      return;
    }

    // Async modes — return a taskId; the result lands via engine:done.
    try {
      const paths = files.map((file) => file.filePath);
      const fmt = formatOverride.trim() || undefined;
      const response =
        mode === "multi"
          ? await engine.analyzeMultiThread({
              paths,
              formatOverride: fmt,
              threshold,
              topN,
            })
          : await engine.analyzeLockContention({
              paths,
              formatOverride: fmt,
              topN,
            });
      taskRef.current = response.taskId;
    } catch (caught) {
      setError({
        code: "ENGINE_FAILED",
        message: caught instanceof Error ? caught.message : String(caught),
      });
      setState("error");
    }
  }, [canAnalyze, files, formatOverride, mode, singleFile, threshold, topN]);

  const cancelAnalysis = useCallback(() => {
    if (mode === "single") {
      // Sync analyzer doesn't expose a cancel hook — discard the token
      // so the late-arriving promise becomes a no-op.
      inflightSyncRef.current = null;
      setState("ready");
      setEngineMessages([t("analysisCanceled")]);
      return;
    }
    const taskId = taskRef.current;
    if (!taskId) return;
    void engine.cancelEngineTask(taskId).catch(() => {
      // Best-effort cancel: even if the registry rejects we already
      // told the renderer to stop spinning. The done/error event for
      // the original taskId is filtered out anyway by taskRef gating.
    });
    taskRef.current = null;
    setState("ready");
    setEngineMessages([t("analysisCanceled")]);
  }, [mode, t]);

  // ────────────────────────────────────────────────────────────────
  // Per-mode result projections (typed cast happens once per mode).
  // ────────────────────────────────────────────────────────────────

  const singleResult =
    mode === "single" && result?.type === "thread_dump"
      ? (result as ThreadDumpSingleResult)
      : null;
  const multiResult =
    mode === "multi" && result?.type === "thread_dump_multi"
      ? (result as ThreadDumpMultiResult)
      : null;
  const lockResult =
    mode === "locks" && (result?.type === "thread_dump_locks" || result?.type === "lock_contention")
      ? (result as LockContentionResult)
      : null;

  return (
    <main className="flex flex-col gap-5 p-5">
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="inline-flex items-center gap-2 text-sm">
            {t("threadDumpModeLabel")}
            <HelpTip text={getHelpText(locale, "sectionThreadMode")} />
          </CardTitle>
          <CardDescription>
            {mode === "single"
              ? t("threadDumpModeSingleHint")
              : mode === "multi"
                ? t("threadDumpModeMultiHint")
                : t("threadDumpModeLocksHint")}
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-wrap gap-2">
          {(["single", "multi", "locks"] as AnalysisMode[]).map((opt) => (
            <Button
              key={opt}
              type="button"
              size="sm"
              variant={opt === mode ? "default" : "outline"}
              onClick={() => switchMode(opt)}
            >
              {opt === "single"
                ? t("threadDumpModeSingle")
                : opt === "multi"
                  ? t("threadDumpModeMulti")
                  : t("threadDumpModeLocks")}
            </Button>
          ))}
        </CardContent>
      </Card>

      <div className="flex items-stretch gap-3">
        <div className="min-w-0 flex-1">
          {mode === "single" ? (
            <WailsFileDock
              className="min-w-0"
              label={t("threadDumpSelectFile")}
              helpText={getHelpText(locale, "pageThreadDump")}
              description={t("threadDumpDropOrBrowse")}
              accept=".txt,.log,.dump,.json"
              selected={singleFile}
              onSelect={handleSingleFile}
              onClear={handleClearSingle}
              browseLabel={t("browseFile")}
              dropHereLabel={t("dropHere")}
              errorLabel={t("error")}
              fileFilters={FILE_FILTERS}
              rightSlot={
                <RunControls
                  t={t}
                  state={state}
                  canAnalyze={canAnalyze}
                  onRun={() => void analyze()}
                  onCancel={() => cancelAnalysis()}
                  runLabel={t("threadDumpRunSingle")}
                />
              }
            />
          ) : (
            <MultiFileDock
              t={t}
              files={files}
              mode={mode}
              state={state}
              canAnalyze={canAnalyze}
              onPick={() => void pickMultipleFiles()}
              onRemove={removeFile}
              onClear={clearFiles}
              onRun={() => void analyze()}
              onCancel={() => cancelAnalysis()}
            />
          )}
        </div>

        <AnalyzerOptionsDock
          title={t("analyzerOptions")}
          footer={
            <div className="flex justify-end">
              <RunControls
                t={t}
                state={state}
                canAnalyze={canAnalyze}
                onRun={() => void analyze()}
                onCancel={() => cancelAnalysis()}
                runLabel={
                  mode === "single"
                    ? t("threadDumpRunSingle")
                    : mode === "multi"
                      ? t("threadDumpRunMulti")
                      : t("threadDumpRunLocks")
                }
              />
            </div>
          }
        >
          <div className="grid grid-cols-1 gap-3">
          <label className="flex flex-col gap-1.5 text-xs">
            <HelpedLabel help={getHelpText(locale, "optionTopN")} className="font-medium text-foreground/80">
              {t("topN")}
            </HelpedLabel>
            <Input
              type="number"
              min={1}
              value={topN}
              onChange={(event) =>
                setTopN(Math.max(1, Number(event.target.value) || 20))
              }
            />
          </label>
          {mode === "multi" && (
            <label className="flex flex-col gap-1.5 text-xs">
              <HelpedLabel help={getHelpText(locale, "optionThreadThreshold")} className="font-medium text-foreground/80">
                {t("threadDumpThreshold")}
              </HelpedLabel>
              <Input
                type="number"
                min={1}
                value={threshold}
                onChange={(event) =>
                  setThreshold(Math.max(1, Number(event.target.value) || 1))
                }
              />
            </label>
          )}
          {mode !== "single" && (
            <label className="flex flex-col gap-1.5 text-xs">
              <HelpedLabel help={getHelpText(locale, "optionThreadFormat")} className="font-medium text-foreground/80">
                {t("threadDumpFormatOverride")}
              </HelpedLabel>
              <Input
                type="text"
                placeholder={t("threadDumpFormatOverridePlaceholder")}
                value={formatOverride}
                onChange={(event) => setFormatOverride(event.target.value)}
              />
            </label>
          )}
          </div>
        </AnalyzerOptionsDock>
      </div>

      <ErrorPanel
        error={error}
        labels={{ title: t("analysisError"), code: t("errorCode") }}
      />
      <EngineMessagesPanel
        messages={engineMessages}
        title={t("engineMessages")}
      />

      {state === "running" && (
        <Card className="border-primary/40 bg-primary/5">
          <CardContent className="flex items-center gap-3 px-4 py-3 text-sm">
            <Loader2 className="h-4 w-4 animate-spin text-primary" />
            <span>{t("threadDumpAwaitingResult")}</span>
          </CardContent>
        </Card>
      )}

      {singleResult && (
        <SingleDumpResult t={t} result={singleResult} />
      )}
      {multiResult && (
        <MultiDumpResult t={t} result={multiResult} />
      )}
      {lockResult && (
        <LockContentionView t={t} result={lockResult} />
      )}
    </main>
  );
}

// ────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────

function extractData<T>(event: unknown): T | null {
  if (!event) return null;
  if (typeof event === "object" && event !== null && "data" in event) {
    const data = (event as { data?: unknown }).data;
    if (data) return data as T;
  }
  return event as T;
}

function basenameOf(path: string): string {
  const segments = path.split(/[\\/]/);
  return segments[segments.length - 1] ?? path;
}

// ────────────────────────────────────────────────────────────────
// Run + Cancel buttons.
// ────────────────────────────────────────────────────────────────

function RunControls({
  t,
  state,
  canAnalyze,
  onRun,
  onCancel,
  runLabel,
}: {
  t: (key: MessageKey) => string;
  state: AnalyzerState;
  canAnalyze: boolean;
  onRun: () => void;
  onCancel: () => void;
  runLabel: string;
}): JSX.Element {
  return (
    <div className="flex items-center gap-2">
      <Button
        type="button"
        size="sm"
        disabled={!canAnalyze}
        onClick={onRun}
      >
        {state === "running" ? (
          <>
            <Loader2 className="h-3.5 w-3.5 animate-spin" />
            {t("analyzing")}
          </>
        ) : (
          <>
            <Play className="h-3.5 w-3.5" />
            {runLabel}
          </>
        )}
      </Button>
      {state === "running" && (
        <Button
          type="button"
          size="sm"
          variant="outline"
          onClick={onCancel}
        >
          <Square className="h-3.5 w-3.5" />
          {t("cancelAnalysis")}
        </Button>
      )}
    </div>
  );
}

// ────────────────────────────────────────────────────────────────
// Multi-file selector card.
// ────────────────────────────────────────────────────────────────

type MultiFileDockProps = {
  t: (key: MessageKey) => string;
  files: FileDockSelection[];
  mode: Exclude<AnalysisMode, "single">;
  state: AnalyzerState;
  canAnalyze: boolean;
  onPick: () => void;
  onRemove: (filePath: string) => void;
  onClear: () => void;
  onRun: () => void;
  onCancel: () => void;
};

function MultiFileDock({
  t,
  files,
  mode,
  state,
  canAnalyze,
  onPick,
  onRemove,
  onClear,
  onRun,
  onCancel,
}: MultiFileDockProps): JSX.Element {
  const { locale } = useI18n();
  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-3 space-y-0 pb-3">
        <div className="min-w-0">
          <CardTitle className="inline-flex items-center gap-2 text-sm">
            {t("threadDumpsSelected")} ({files.length})
            <HelpTip text={getHelpText(locale, "sectionThreadSelected")} />
          </CardTitle>
          <CardDescription>
            {t("threadDumpDropOrBrowseMulti")}
          </CardDescription>
        </div>
        <div className="flex items-center gap-2">
          <Button type="button" size="sm" variant="secondary" onClick={onPick}>
            <Upload className="h-3.5 w-3.5" />
            {t("threadDumpAdd")}
          </Button>
          {files.length > 0 && (
            <Button type="button" size="sm" variant="ghost" onClick={onClear}>
              {t("threadDumpClear")}
            </Button>
          )}
          <RunControls
            t={t}
            state={state}
            canAnalyze={canAnalyze}
            onRun={onRun}
            onCancel={onCancel}
            runLabel={
              mode === "multi"
                ? t("threadDumpRunMulti")
                : t("threadDumpRunLocks")
            }
          />
        </div>
      </CardHeader>
      <CardContent className="space-y-3 p-4 pt-0">
        {files.length === 0 ? (
          <p className="rounded-md border border-dashed border-border bg-muted/30 px-3 py-3 text-xs text-muted-foreground">
            {t("threadDumpEmpty")}
          </p>
        ) : (
          <ul className="divide-y divide-border rounded-md border border-border">
            {files.map((file, index) => (
              <li
                key={file.filePath}
                className="flex items-center gap-3 px-3 py-2 text-sm"
              >
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-muted text-[10px] font-semibold tabular-nums text-muted-foreground">
                  {index}
                </span>
                <FileText className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                <span
                  className="flex-1 truncate font-mono text-xs"
                  title={file.filePath}
                >
                  {file.originalName}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  aria-label={t("threadDumpRemove")}
                  onClick={() => onRemove(file.filePath)}
                >
                  <X className="h-3 w-3" />
                </Button>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  );
}

// ────────────────────────────────────────────────────────────────
// Single-dump result rendering.
// ────────────────────────────────────────────────────────────────

function SingleDumpResult({
  t,
  result,
}: {
  t: (key: MessageKey) => string;
  result: ThreadDumpSingleResult;
}): JSX.Element {
  const { locale } = useI18n();
  const summary: ThreadDumpSingleSummary = result.summary;
  const stateRows: ThreadDumpStateRow[] = result.series.state_distribution ?? [];
  const categoryRows: ThreadDumpCategoryRow[] =
    result.series.category_distribution ?? [];
  const signatures: ThreadDumpSignatureRow[] =
    result.series.top_stack_signatures ?? [];
  const threads: ThreadDumpThreadRow[] = result.tables.threads ?? [];
  const findings: ThreadDumpFinding[] = result.metadata.findings ?? [];

  return (
    <Tabs defaultValue="summary" className="w-full">
      <TabsList>
        <TabsTrigger value="summary">{t("threadDumpTabSummary")}</TabsTrigger>
        <TabsTrigger value="findings">
          {t("threadDumpTabFindings")} ({findings.length})
        </TabsTrigger>
        <TabsTrigger value="states">{t("threadDumpTabStates")}</TabsTrigger>
        <TabsTrigger value="signatures">
          {t("threadDumpTabSignatures")}
        </TabsTrigger>
        <TabsTrigger value="threads">
          {t("threadDumpTabThreads")} ({threads.length})
        </TabsTrigger>
      </TabsList>

      <TabsContent value="summary" className="mt-4">
        <section className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
          <MetricCard
            label={t("threadDumpTotalThreads")}
            value={formatNumber(summary.total_threads)}
          />
          <MetricCard
            label={t("threadDumpRunnable")}
            value={formatNumber(summary.runnable_threads)}
          />
          <MetricCard
            label={t("threadDumpBlockedState")}
            value={formatNumber(summary.blocked_threads)}
          />
          <MetricCard
            label={t("threadDumpWaiting")}
            value={formatNumber(summary.waiting_threads)}
          />
          <MetricCard
            label={t("threadDumpWithLocks")}
            value={formatNumber(summary.threads_with_locks)}
          />
        </section>
      </TabsContent>

      <TabsContent value="findings" className="mt-4">
        <FindingsList t={t} findings={findings} />
      </TabsContent>

      <TabsContent value="states" className="mt-4">
        <div className="grid gap-4 md:grid-cols-2">
          <SimpleCountTable
            title={t("threadDumpTabStates")}
            keyHeader={t("threadDumpColState")}
            rows={stateRows.map((row) => ({
              key: row.state,
              count: row.count,
            }))}
          />
          <SimpleCountTable
            title={t("threadDumpColCategory")}
            keyHeader={t("threadDumpColCategory")}
            rows={categoryRows.map((row) => ({
              key: row.category,
              count: row.count,
            }))}
          />
        </div>
      </TabsContent>

      <TabsContent value="signatures" className="mt-4">
        {signatures.length === 0 ? (
          <EmptyCard message={t("threadDumpSignaturesEmpty")} />
        ) : (
          <Card>
            <CardHeader className="pb-3">
              <CardTitle className="inline-flex items-center gap-2 text-sm">
                {t("threadDumpTabSignatures")}
                <HelpTip text={getHelpText(locale, "sectionThreadSignatures")} />
              </CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColSignature")}
                    </th>
                    <th className="w-[100px] px-3 py-2 text-right font-medium">
                      {t("threadDumpColCount")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {signatures.map((row, idx) => (
                    <tr
                      key={`${row.signature}-${idx}`}
                      className="border-b border-border last:border-0"
                    >
                      <td className="max-w-[600px] truncate px-3 py-1.5 font-mono">
                        {row.signature}
                      </td>
                      <td className="px-3 py-1.5 text-right tabular-nums">
                        {row.count.toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        )}
      </TabsContent>

      <TabsContent value="threads" className="mt-4">
        {threads.length === 0 ? (
          <EmptyCard message={t("threadDumpThreadsEmpty")} />
        ) : (
          <Card>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColThread")}
                    </th>
                    <th className="w-[110px] px-3 py-2 text-left font-medium">
                      {t("threadDumpColState")}
                    </th>
                    <th className="w-[110px] px-3 py-2 text-left font-medium">
                      {t("threadDumpColCategory")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColTopFrame")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColLock")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {threads.slice(0, 200).map((row, idx) => (
                    <tr
                      key={`${row.name ?? row.thread_name ?? idx}-${idx}`}
                      className="border-b border-border last:border-0"
                    >
                      <td
                        className="max-w-[260px] truncate px-3 py-1.5 font-mono"
                        title={row.name ?? row.thread_name ?? ""}
                      >
                        {row.name ?? row.thread_name ?? "—"}
                      </td>
                      <td className="px-3 py-1.5">{row.state ?? "—"}</td>
                      <td className="px-3 py-1.5">{row.category ?? "—"}</td>
                      <td
                        className="max-w-[420px] truncate px-3 py-1.5 font-mono"
                        title={row.top_frame ?? ""}
                      >
                        {row.top_frame ?? "—"}
                      </td>
                      <td
                        className="max-w-[260px] truncate px-3 py-1.5 font-mono"
                        title={row.lock_info ?? ""}
                      >
                        {row.lock_info ?? "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        )}
      </TabsContent>
    </Tabs>
  );
}

// ────────────────────────────────────────────────────────────────
// Multi-dump result rendering.
// ────────────────────────────────────────────────────────────────

function MultiDumpResult({
  t,
  result,
}: {
  t: (key: MessageKey) => string;
  result: ThreadDumpMultiResult;
}): JSX.Element {
  const { locale } = useI18n();
  const summary: ThreadDumpMultiSummary = result.summary;
  const longRunning: LongRunningStackRow[] =
    result.tables.long_running_stacks ?? [];
  const persistentBlocked: PersistentBlockedRow[] =
    result.tables.persistent_blocked_threads ?? [];
  const dumps: DumpRow[] = result.tables.dumps ?? [];
  const statePerDump: StatePerDumpRow[] =
    result.series.state_distribution_per_dump ?? [];
  const persistence: ThreadPersistenceRow[] =
    result.series.thread_persistence ?? [];
  const findings: ThreadDumpFinding[] = result.metadata.findings ?? [];
  const carrierPinning: CarrierPinningRow[] =
    result.tables.virtual_thread_carrier_pinning ?? [];
  const smrUnresolved: SmrUnresolvedRow[] =
    result.tables.smr_unresolved_threads ?? [];
  const nativeMethodThreads: NativeMethodRow[] =
    result.tables.native_method_threads ?? [];
  const classHistogram: ClassHistogramRow[] =
    result.tables.class_histogram_top_classes ?? [];

  return (
    <Tabs defaultValue="summary" className="w-full">
      <TabsList>
        <TabsTrigger value="summary">{t("threadDumpTabSummary")}</TabsTrigger>
        <TabsTrigger value="findings">
          {t("threadDumpTabFindings")} ({findings.length})
        </TabsTrigger>
        <TabsTrigger value="long">
          {t("threadDumpTabLongRunning")} ({longRunning.length})
        </TabsTrigger>
        <TabsTrigger value="blocked">
          {t("threadDumpTabBlocked")} ({persistentBlocked.length})
        </TabsTrigger>
        <TabsTrigger value="per-dump">
          {t("threadDumpTabPerDump")}
        </TabsTrigger>
        <TabsTrigger value="threads">
          {t("threadDumpTabThreads")} ({persistence.length})
        </TabsTrigger>
        <TabsTrigger value="dumps">
          {t("threadDumpTabDumps")} ({dumps.length})
        </TabsTrigger>
        <TabsTrigger value="jvm">JVM signals</TabsTrigger>
      </TabsList>

      <TabsContent value="summary" className="mt-4 space-y-4">
        <section className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-6">
          <MetricCard
            label={t("threadDumpTotalDumps")}
            value={formatNumber(summary.total_dumps)}
          />
          <MetricCard
            label={t("threadDumpUniqueThreads")}
            value={formatNumber(summary.unique_threads)}
          />
          <MetricCard
            label={t("threadDumpLongRunning")}
            value={formatNumber(summary.long_running_threads)}
          />
          <MetricCard
            label={t("threadDumpPersistBlocked")}
            value={formatNumber(summary.persistent_blocked_threads)}
          />
          <MetricCard
            label={t("threadDumpVirtualPinning")}
            value={formatNumber(summary.virtual_thread_carrier_pinning ?? 0)}
          />
          <MetricCard
            label={t("threadDumpSmrUnresolved")}
            value={formatNumber(summary.smr_unresolved_threads ?? 0)}
          />
          <MetricCard
            label={t("threadDumpNativeMethods")}
            value={formatNumber(summary.native_method_threads ?? 0)}
          />
          <MetricCard
            label={t("threadDumpHistogramClasses")}
            value={formatNumber(summary.class_histogram_classes ?? 0)}
          />
          <MetricCard
            label={t("threadDumpThresholdMet")}
            value={String(summary.consecutive_dump_threshold)}
          />
          <MetricCard
            label={t("threadDumpLanguages")}
            value={(summary.languages_detected ?? []).join(", ") || "—"}
          />
          <MetricCard
            label={t("threadDumpFormats")}
            value={(summary.source_formats ?? []).join(", ") || "—"}
          />
        </section>
      </TabsContent>

      <TabsContent value="findings" className="mt-4">
        <FindingsList t={t} findings={findings} />
      </TabsContent>

      <TabsContent value="long" className="mt-4">
        {longRunning.length === 0 ? (
          <EmptyCard message={t("threadDumpLongRunningEmpty")} />
        ) : (
          <Card>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColThread")}
                    </th>
                    <th className="w-[80px] px-3 py-2 text-right font-medium">
                      {t("threadDumpColDumps")}
                    </th>
                    <th className="w-[100px] px-3 py-2 text-right font-medium">
                      {t("threadDumpColFirstSeen")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColSignature")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {longRunning.map((row, idx) => (
                    <tr
                      key={`${row.thread_name}-${idx}`}
                      className="border-b border-border last:border-0"
                    >
                      <td
                        className="max-w-[260px] truncate px-3 py-1.5 font-mono"
                        title={row.thread_name}
                      >
                        {row.thread_name}
                      </td>
                      <td className="px-3 py-1.5 text-right tabular-nums">
                        {row.dumps}
                      </td>
                      <td className="px-3 py-1.5 text-right tabular-nums">
                        {row.first_dump_index}
                      </td>
                      <td
                        className="max-w-[480px] truncate px-3 py-1.5 font-mono"
                        title={row.stack_signature}
                      >
                        {row.stack_signature}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        )}
      </TabsContent>

      <TabsContent value="blocked" className="mt-4">
        {persistentBlocked.length === 0 ? (
          <EmptyCard message={t("threadDumpBlockedEmpty")} />
        ) : (
          <Card>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColThread")}
                    </th>
                    <th className="w-[80px] px-3 py-2 text-right font-medium">
                      {t("threadDumpColDumps")}
                    </th>
                    <th className="w-[100px] px-3 py-2 text-right font-medium">
                      {t("threadDumpColFirstSeen")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColSignature")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {persistentBlocked.map((row, idx) => (
                    <tr
                      key={`${row.thread_name}-${idx}`}
                      className="border-b border-border last:border-0"
                    >
                      <td
                        className="max-w-[260px] truncate px-3 py-1.5 font-mono"
                        title={row.thread_name}
                      >
                        {row.thread_name}
                      </td>
                      <td className="px-3 py-1.5 text-right tabular-nums">
                        {row.dumps}
                      </td>
                      <td className="px-3 py-1.5 text-right tabular-nums">
                        {row.first_dump_index}
                      </td>
                      <td className="max-w-[480px] truncate px-3 py-1.5 font-mono">
                        {(row.stack_signatures ?? []).join(" | ")}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        )}
      </TabsContent>

      <TabsContent value="per-dump" className="mt-4">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="inline-flex items-center gap-2 text-sm">
              {t("threadDumpTabPerDump")}
              <HelpTip text={getHelpText(locale, "sectionThreadPerDump")} />
            </CardTitle>
          </CardHeader>
          <CardContent className="overflow-x-auto p-0">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                  <th className="w-[60px] px-3 py-2 text-left font-medium">
                    {t("threadDumpColDumpIndex")}
                  </th>
                  <th className="px-3 py-2 text-left font-medium">
                    {t("threadDumpColDumpLabel")}
                  </th>
                  <th className="px-3 py-2 text-left font-medium">
                    {t("threadDumpColState")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {statePerDump.map((row) => (
                  <tr
                    key={row.dump_index}
                    className="border-b border-border last:border-0"
                  >
                    <td className="px-3 py-1.5 tabular-nums">
                      {row.dump_index}
                    </td>
                    <td className="px-3 py-1.5 font-mono text-[11px]">
                      {row.dump_label ?? "—"}
                    </td>
                    <td className="px-3 py-1.5 text-[11px]">
                      {Object.entries(row.counts ?? {})
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

      <TabsContent value="threads" className="mt-4">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="inline-flex items-center gap-2 text-sm">
              {t("threadDumpTabThreads")}
              <HelpTip text={getHelpText(locale, "sectionThreadRows")} />
            </CardTitle>
          </CardHeader>
          <CardContent className="overflow-x-auto p-0">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                  <th className="px-3 py-2 text-left font-medium">
                    {t("threadDumpColThread")}
                  </th>
                  <th className="w-[140px] px-3 py-2 text-right font-medium">
                    {t("threadDumpColDumps")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {persistence.slice(0, 200).map((row) => (
                  <tr
                    key={row.thread_name}
                    className="border-b border-border last:border-0"
                  >
                    <td
                      className="max-w-[440px] truncate px-3 py-1.5 font-mono"
                      title={row.thread_name}
                    >
                      {row.thread_name}
                    </td>
                    <td className="px-3 py-1.5 text-right tabular-nums">
                      {row.observed_in_dumps}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="dumps" className="mt-4">
        <Card>
          <CardContent className="overflow-x-auto p-0">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                  <th className="w-[60px] px-3 py-2 text-left font-medium">
                    {t("threadDumpColDumpIndex")}
                  </th>
                  <th className="px-3 py-2 text-left font-medium">
                    {t("threadDumpColDumpLabel")}
                  </th>
                  <th className="px-3 py-2 text-left font-medium">
                    {t("threadDumpColSourceFile")}
                  </th>
                  <th className="px-3 py-2 text-left font-medium">
                    {t("threadDumpColLanguage")}
                  </th>
                  <th className="px-3 py-2 text-left font-medium">
                    {t("threadDumpColFormat")}
                  </th>
                  <th className="w-[100px] px-3 py-2 text-right font-medium">
                    {t("threadDumpColThreadCount")}
                  </th>
                </tr>
              </thead>
              <tbody>
                {dumps.map((row) => (
                  <tr
                    key={row.dump_index}
                    className="border-b border-border last:border-0"
                  >
                    <td className="px-3 py-1.5 tabular-nums">
                      {row.dump_index}
                    </td>
                    <td className="px-3 py-1.5 font-mono text-[11px]">
                      {row.dump_label ?? "—"}
                    </td>
                    <td
                      className="max-w-[320px] truncate px-3 py-1.5 font-mono"
                      title={row.source_file}
                    >
                      {row.source_file}
                    </td>
                    <td className="px-3 py-1.5">{row.language}</td>
                    <td className="px-3 py-1.5">{row.source_format}</td>
                    <td className="px-3 py-1.5 text-right tabular-nums">
                      {row.thread_count}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </CardContent>
        </Card>
      </TabsContent>

      <TabsContent value="jvm" className="mt-4">
        <JvmSignals
          t={t}
          carrierPinning={carrierPinning}
          smrUnresolved={smrUnresolved}
          nativeMethodThreads={nativeMethodThreads}
          classHistogram={classHistogram}
        />
      </TabsContent>
    </Tabs>
  );
}

// ────────────────────────────────────────────────────────────────
// JVM signals (carrier pinning / SMR / native methods / histogram).
// ────────────────────────────────────────────────────────────────

function JvmSignals({
  t,
  carrierPinning,
  smrUnresolved,
  nativeMethodThreads,
  classHistogram,
}: {
  t: (key: MessageKey) => string;
  carrierPinning: CarrierPinningRow[];
  smrUnresolved: SmrUnresolvedRow[];
  nativeMethodThreads: NativeMethodRow[];
  classHistogram: ClassHistogramRow[];
}): JSX.Element {
  return (
    <Tabs defaultValue="pinning">
      <TabsList>
        <TabsTrigger value="pinning">
          {t("threadDumpVirtualPinning")} ({carrierPinning.length})
        </TabsTrigger>
        <TabsTrigger value="smr">
          {t("threadDumpSmrUnresolved")} ({smrUnresolved.length})
        </TabsTrigger>
        <TabsTrigger value="native">
          {t("threadDumpNativeMethods")} ({nativeMethodThreads.length})
        </TabsTrigger>
        <TabsTrigger value="histogram">
          {t("threadDumpHistogramClasses")} ({classHistogram.length})
        </TabsTrigger>
      </TabsList>

      <TabsContent value="pinning" className="mt-4">
        {carrierPinning.length === 0 ? (
          <EmptyCard message="—" />
        ) : (
          <Card>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="w-[60px] px-3 py-2 text-left font-medium">
                      {t("threadDumpColDumpIndex")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColThread")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColTopFrame")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {carrierPinning.slice(0, 200).map((row, idx) => (
                    <tr
                      key={`${row.thread_name}-${row.dump_index}-${idx}`}
                      className="border-b border-border last:border-0"
                    >
                      <td className="px-3 py-1.5 tabular-nums">
                        {row.dump_index}
                      </td>
                      <td
                        className="max-w-[260px] truncate px-3 py-1.5 font-mono"
                        title={row.thread_name}
                      >
                        {row.thread_name}
                      </td>
                      <td
                        className="max-w-[440px] truncate px-3 py-1.5 font-mono"
                        title={row.top_frame ?? ""}
                      >
                        {row.top_frame ?? "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        )}
      </TabsContent>

      <TabsContent value="smr" className="mt-4">
        {smrUnresolved.length === 0 ? (
          <EmptyCard message="—" />
        ) : (
          <Card>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="w-[60px] px-3 py-2 text-left font-medium">
                      {t("threadDumpColDumpIndex")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">Line</th>
                  </tr>
                </thead>
                <tbody>
                  {smrUnresolved.slice(0, 200).map((row, idx) => (
                    <tr
                      key={`${row.dump_index}-${idx}`}
                      className="border-b border-border last:border-0"
                    >
                      <td className="px-3 py-1.5 tabular-nums">
                        {row.dump_index}
                      </td>
                      <td
                        className="px-3 py-1.5 font-mono"
                        title={row.line}
                      >
                        {row.line ?? "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        )}
      </TabsContent>

      <TabsContent value="native" className="mt-4">
        {nativeMethodThreads.length === 0 ? (
          <EmptyCard message="—" />
        ) : (
          <Card>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="w-[60px] px-3 py-2 text-left font-medium">
                      {t("threadDumpColDumpIndex")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColThread")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpColTopFrame")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {nativeMethodThreads.slice(0, 200).map((row, idx) => (
                    <tr
                      key={`${row.thread_name}-${row.dump_index}-${idx}`}
                      className="border-b border-border last:border-0"
                    >
                      <td className="px-3 py-1.5 tabular-nums">
                        {row.dump_index}
                      </td>
                      <td
                        className="max-w-[260px] truncate px-3 py-1.5 font-mono"
                        title={row.thread_name}
                      >
                        {row.thread_name}
                      </td>
                      <td
                        className="max-w-[440px] truncate px-3 py-1.5 font-mono"
                        title={row.native_method ?? ""}
                      >
                        {row.native_method ?? "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        )}
      </TabsContent>

      <TabsContent value="histogram" className="mt-4">
        {classHistogram.length === 0 ? (
          <EmptyCard message="—" />
        ) : (
          <Card>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">Class</th>
                    <th className="w-[120px] px-3 py-2 text-right font-medium">
                      Instances
                    </th>
                    <th className="w-[140px] px-3 py-2 text-right font-medium">
                      Bytes
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {classHistogram.slice(0, 50).map((row, idx) => (
                    <tr
                      key={`${row.class_name}-${idx}`}
                      className="border-b border-border last:border-0"
                    >
                      <td
                        className="max-w-[480px] truncate px-3 py-1.5 font-mono"
                        title={row.class_name}
                      >
                        {row.class_name}
                      </td>
                      <td className="px-3 py-1.5 text-right tabular-nums">
                        {(row.instances ?? 0).toLocaleString()}
                      </td>
                      <td className="px-3 py-1.5 text-right tabular-nums">
                        {(row.bytes ?? 0).toLocaleString()}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        )}
      </TabsContent>
    </Tabs>
  );
}

// ────────────────────────────────────────────────────────────────
// Lock contention rendering.
// ────────────────────────────────────────────────────────────────

function LockContentionView({
  t,
  result,
}: {
  t: (key: MessageKey) => string;
  result: LockContentionResult;
}): JSX.Element {
  const { locale } = useI18n();
  const summary = result.summary;
  const locks: LockContentionRow[] = result.tables.locks ?? [];
  const deadlocks: DeadlockChain[] = result.tables.deadlock_chains ?? [];
  const ranking: LockContentionRankingRow[] =
    result.series.contention_ranking ?? [];
  const findings: ThreadDumpFinding[] = result.metadata.findings ?? [];

  return (
    <Tabs defaultValue="summary" className="w-full">
      <TabsList>
        <TabsTrigger value="summary">{t("threadDumpTabSummary")}</TabsTrigger>
        <TabsTrigger value="findings">
          {t("threadDumpTabFindings")} ({findings.length})
        </TabsTrigger>
        <TabsTrigger value="locks">
          {t("threadDumpTabLocks")} ({locks.length})
        </TabsTrigger>
        <TabsTrigger value="deadlocks">
          {t("threadDumpTabDeadlocks")} ({deadlocks.length})
        </TabsTrigger>
        <TabsTrigger value="ranking">Ranking ({ranking.length})</TabsTrigger>
      </TabsList>

      <TabsContent value="summary" className="mt-4 space-y-4">
        <section className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
          <MetricCard
            label={t("threadDumpContendedLocks")}
            value={formatNumber(summary.contended_locks)}
          />
          <MetricCard
            label={t("threadDumpDeadlocks")}
            value={formatNumber(summary.deadlocks_detected)}
            className={
              summary.deadlocks_detected > 0
                ? "border-destructive/40 bg-destructive/10"
                : undefined
            }
          />
          <MetricCard
            label={t("threadDumpThreadsWithLocks")}
            value={formatNumber(summary.threads_with_locks)}
          />
          <MetricCard
            label={t("threadDumpThreadsWaiting")}
            value={formatNumber(summary.threads_waiting_on_lock)}
          />
          <MetricCard
            label={t("threadDumpUniqueLocks")}
            value={formatNumber(summary.unique_locks)}
          />
        </section>
      </TabsContent>

      <TabsContent value="findings" className="mt-4">
        <FindingsList t={t} findings={findings} />
      </TabsContent>

      <TabsContent value="locks" className="mt-4">
        {locks.length === 0 ? (
          <EmptyCard message={t("threadDumpLocksEmpty")} />
        ) : (
          <Card>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpLockId")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpLockClass")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpLockOwner")}
                    </th>
                    <th className="w-[100px] px-3 py-2 text-right font-medium">
                      {t("threadDumpLockWaiters")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpLockTopWaiters")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {locks.map((row) => (
                    <tr
                      key={row.lock_id}
                      className="border-b border-border last:border-0"
                    >
                      <td
                        className="max-w-[200px] truncate px-3 py-1.5 font-mono text-[11px]"
                        title={row.lock_id}
                      >
                        {row.lock_id}
                      </td>
                      <td
                        className="max-w-[260px] truncate px-3 py-1.5 font-mono text-[11px]"
                        title={row.lock_class ?? ""}
                      >
                        {row.lock_class ?? "—"}
                      </td>
                      <td className="max-w-[200px] truncate px-3 py-1.5">
                        {row.owner_thread ?? "—"}
                      </td>
                      <td className="px-3 py-1.5 text-right tabular-nums">
                        {row.waiter_count}
                      </td>
                      <td className="px-3 py-1.5 text-[11px] text-muted-foreground">
                        {row.top_waiters?.join(", ") || "—"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        )}
      </TabsContent>

      <TabsContent value="deadlocks" className="mt-4">
        {deadlocks.length === 0 ? (
          <EmptyCard message={t("threadDumpDeadlocksEmpty")} />
        ) : (
          <div className="space-y-3">
            {deadlocks.map((chain, idx) => (
              <Card
                key={idx}
                className="border-destructive/40 bg-destructive/5"
              >
                <CardHeader className="pb-2">
                  <CardTitle className="inline-flex items-center gap-2 text-sm text-destructive">
                    {t("threadDumpDeadlockCycle")} #{idx + 1}
                    <HelpTip text={getHelpText(locale, "sectionThreadDeadlock")} variant="warning" />
                  </CardTitle>
                </CardHeader>
                <CardContent className="space-y-2 text-xs">
                  <p className="font-mono font-semibold">
                    {[...chain.threads, chain.threads[0]].join(" → ")}
                  </p>
                  <ul className="space-y-0.5 text-[11px] text-muted-foreground">
                    {chain.edges.map((edge, edgeIndex) => (
                      <li key={edgeIndex} className="font-mono">
                        {edge.from_thread} → {edge.to_thread} (
                        {edge.lock_id ?? "?"}
                        {edge.lock_class ? ` · ${edge.lock_class}` : ""})
                      </li>
                    ))}
                  </ul>
                </CardContent>
              </Card>
            ))}
          </div>
        )}
      </TabsContent>

      <TabsContent value="ranking" className="mt-4">
        {ranking.length === 0 ? (
          <EmptyCard message="—" />
        ) : (
          <Card>
            <CardContent className="overflow-x-auto p-0">
              <table className="w-full text-xs">
                <thead>
                  <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpLockId")}
                    </th>
                    <th className="px-3 py-2 text-left font-medium">
                      {t("threadDumpLockClass")}
                    </th>
                    <th className="w-[100px] px-3 py-2 text-right font-medium">
                      {t("threadDumpLockWaiters")}
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {ranking.map((row) => (
                    <tr
                      key={row.lock_id}
                      className="border-b border-border last:border-0"
                    >
                      <td
                        className="max-w-[260px] truncate px-3 py-1.5 font-mono text-[11px]"
                        title={row.lock_id}
                      >
                        {row.lock_id}
                      </td>
                      <td
                        className="max-w-[260px] truncate px-3 py-1.5 font-mono text-[11px]"
                        title={row.lock_class ?? ""}
                      >
                        {row.lock_class ?? "—"}
                      </td>
                      <td className="px-3 py-1.5 text-right tabular-nums">
                        {row.waiter_count}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>
        )}
      </TabsContent>
    </Tabs>
  );
}

// ────────────────────────────────────────────────────────────────
// Reusable bits.
// ────────────────────────────────────────────────────────────────

function FindingsList({
  t,
  findings,
}: {
  t: (key: MessageKey) => string;
  findings: ThreadDumpFinding[];
}): JSX.Element {
  const { locale } = useI18n();
  if (findings.length === 0) {
    return (
      <Card>
        <CardContent className="flex items-center gap-2 px-4 py-3 text-sm text-muted-foreground">
          <Layers className="h-4 w-4" />
          {t("threadDumpFindingsEmpty")}
        </CardContent>
      </Card>
    );
  }
  return (
    <Card>
      <CardHeader className="pb-0">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          {t("threadDumpTabFindings")}
          <HelpTip text={getHelpText(locale, "sectionFindings")} />
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-2 p-4">
        {findings.map((finding, idx) => (
          <div
            key={`${finding.code}-${idx}`}
            className={cn(
              "rounded-md border px-3 py-2 text-xs",
              SEVERITY_COLORS[finding.severity] ?? SEVERITY_COLORS.info,
            )}
          >
            <div className="flex items-center gap-2 font-semibold">
              <AlertTriangle className="h-3.5 w-3.5" />
              <span className="font-mono uppercase">{finding.code}</span>
              <span className="ml-auto text-[10px] uppercase tracking-wide opacity-70">
                {finding.severity}
              </span>
            </div>
            <p className="mt-1 leading-relaxed">{finding.message}</p>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}

function SimpleCountTable({
  title,
  keyHeader,
  rows,
}: {
  title: string;
  keyHeader: string;
  rows: { key: string; count: number }[];
}): JSX.Element {
  const { locale } = useI18n();
  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="inline-flex items-center gap-2 text-sm">
          {title}
          <HelpTip text={getHelpText(locale, "genericTable")} />
        </CardTitle>
      </CardHeader>
      <CardContent className="overflow-x-auto p-0">
        <table className="w-full text-xs">
          <thead>
            <tr className="border-b border-border bg-muted/40 text-[10px] uppercase tracking-wider text-muted-foreground">
              <th className="px-3 py-2 text-left font-medium">{keyHeader}</th>
              <th className="w-[100px] px-3 py-2 text-right font-medium">
                Count
              </th>
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td className="px-3 py-3 text-muted-foreground" colSpan={2}>
                  —
                </td>
              </tr>
            ) : (
              rows.map((row) => (
                <tr
                  key={row.key}
                  className="border-b border-border last:border-0"
                >
                  <td className="px-3 py-1.5 font-mono">{row.key}</td>
                  <td className="px-3 py-1.5 text-right tabular-nums">
                    {row.count.toLocaleString()}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </CardContent>
    </Card>
  );
}

function EmptyCard({ message }: { message: string }): JSX.Element {
  return (
    <Card>
      <CardContent className="px-4 py-6 text-center text-sm text-muted-foreground">
        {message}
      </CardContent>
    </Card>
  );
}
