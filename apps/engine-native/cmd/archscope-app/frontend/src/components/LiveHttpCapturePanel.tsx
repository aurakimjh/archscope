import { useHttpCommands } from "@/hooks/useHttpCommands";
import { LiveHttpAnalyzer } from "./LiveHttpAnalyzer";
import { Dialogs, Events } from "@wailsio/runtime";
import {
  AlertTriangle,
  CircleStop,
  Loader2,
  Play,
  ShieldCheck,
  ShieldOff,
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useState,
} from "react";

import type { HttpCaptureAnalysisResult } from "@/bridge/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/i18n/I18nProvider";
import type { MessageKey } from "@/i18n/messages";
import {
  activeBodyCapturePolicy,
  activeUnattributedPolicy,
  bodyGateReasonLabel,
  buildLiveCoverageDisclosure,
  initialLiveCaptureState,
  isLiveSessionActive,
  liveHttpCaptureReducer,
  resolveLiveCAState,
  resolveLiveSessionState,
  resolveLiveCaptureContract,
  type LiveCAStateToken,
  type LiveCaptureSession,
  type LiveCaptureProcess,
  type LiveCaptureStats,
  type LiveCaptureTransaction,
  type LiveProgressEvent,
  type LiveSessionStateToken,
  type LiveTransactionsEvent,
} from "@/state/liveHttpCapture";
import { formatBytes } from "@/state/httpCapture";
import { formatNumber } from "@/utils/formatters";
import * as CaptureService from "../../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app/captureservice";

type Mode = {
  name: string;
  available: boolean;
  reason?: string;
  requiredPrivilege: string;
  fidelity: string;
};

type CAStatus = {
  state: string;
  fingerprint?: string;
  stores: string[];
  expiresAt?: string | null;
  error?: string;
};

type RecoverySummary = {
  sessions: number;
  records: number;
  discardedBytes: number;
};

type LiveHttpCapturePanelProps = {
  onStarted: () => void;
  onFinalized: (result: HttpCaptureAnalysisResult, sessionId: string) => void;
  showAnalyzer?: boolean;
};

const SESSION_STATE_LABEL_KEYS: Record<LiveSessionStateToken, MessageKey> = {
  created: "liveCaptureSessionStateCreated",
  starting: "liveCaptureSessionStateStarting",
  running: "liveCaptureSessionStateRunning",
  stopping: "liveCaptureSessionStateStopping",
  finalized: "liveCaptureSessionStateFinalized",
  failed: "liveCaptureSessionStateFailed",
  recoverable: "liveCaptureSessionStateRecoverable",
  unknown: "liveCaptureSessionStateUnknown",
};

const CA_STATE_LABEL_KEYS: Record<LiveCAStateToken, MessageKey> = {
  loading: "liveCaptureCAStateLoading",
  absent: "liveCaptureCAStateAbsent",
  installing: "liveCaptureCAStateInstalling",
  trusted: "liveCaptureCAStateTrusted",
  partial: "liveCaptureCAStatePartial",
  failed: "liveCaptureCAStateFailed",
  expired: "liveCaptureCAStateExpired",
  unknown: "liveCaptureCAStateUnknown",
};


function eventData<T>(event: unknown): T | null {
  if (!event || typeof event !== "object") return null;
  const wrapped = event as { data?: unknown };
  return (wrapped.data ?? event) as T;
}

export function LiveHttpCapturePanel({
  onStarted,
  onFinalized,
  showAnalyzer = true,
}: LiveHttpCapturePanelProps): React.JSX.Element {
  const { t, locale } = useI18n();
  const [state, dispatch] = useReducer(
    liveHttpCaptureReducer,
    initialLiveCaptureState,
  );
  const [modes, setModes] = useState<Mode[]>([]);
  const [backends,setBackends]=useState<{name:string;available:boolean;reason?:string;transparent:boolean}[]>([]);
  const [backend,setBackend]=useState("explicit_proxy");
  const [caStatus, setCAStatus] = useState<CAStatus>({
    state: "loading",
    stores: [],
  });
  const [firstUseAccepted, setFirstUseAccepted] = useState(false);
  const [retainUnattributed, setRetainUnattributed] = useState(false);
  // Seeds the per-session choice. The control renders only when the engine
  // publishes the tier as available, which requires a verified SEC-10
  // preflight; on any other result the fail-closed contract hides it and this
  // value stays false.
  const [captureBodies, setCaptureBodies] = useState(false);
  const [processes, setProcesses] = useState<LiveCaptureProcess[]>([]);
  const [selectedProcesses, setSelectedProcesses] = useState<LiveCaptureProcess[]>([]);
  const [processScope, setProcessScope] = useState("all_sessions");
  const [launchPath, setLaunchPath] = useState("");
  const [launchArgs, setLaunchArgs] = useState("[]");
  const chooseExecutable = async () => {
    const selected=await Dialogs.OpenFile({Title:t("httpCaptureLaunch"),Filters:[{DisplayName:"Executable",Pattern:"*.exe"}]});
    if (typeof selected==="string") setLaunchPath(selected);
  };
  const loadProcesses = async () => {
    try {
      const next = await CaptureService.ListCaptureProcesses();
      setProcesses(next as unknown as LiveCaptureProcess[]);
    } catch (caught) {
      dispatch({ type:"error", message:String(caught) });
    }
  };
  const [recovery, setRecovery] = useState<RecoverySummary | null>(null);
  const [analysisBusy, setAnalysisBusy] = useState(false);

  const active = isLiveSessionActive(state.session);

  useEffect(() => {
    if (!state.session) return;
    const keys = state.session.allowedProcesses ?? [];
    setProcessScope(state.session.processScope ?? (keys.length ? "selected" : "all_sessions"));
    setSelectedProcesses(keys.map((key) => ({key, name:`PID ${key.pid}`, attribution:"confirmed"})));
  }, [state.session?.sessionId]);

  const hydrate = useCallback(async (session: LiveCaptureSession) => {
    let stats: LiveCaptureStats | null = null;
    let transactions: LiveCaptureTransaction[] = [];
    try {
      [stats, transactions] = await Promise.all([
        CaptureService.GetCaptureStats(session.sessionId) as Promise<LiveCaptureStats>,
        CaptureService.GetCaptureLiveWindow(session.sessionId) as Promise<
          LiveCaptureTransaction[]
        >,
      ]);
    } catch {
      // A finalized/recovered session may no longer have an in-memory live
      // window. Its analysis remains available through AnalyzeCaptureSession.
    }
    dispatch({ type: "hydrate", session, stats, transactions });
  }, []);

  useEffect(() => {
    let disposed = false;
    const initialize = async () => {
      try {
        const [contract, nextModes, nextCA, current, reports,nextBackends] =
          await Promise.all([
            CaptureService.GetLiveCaptureContract(),
            CaptureService.ListCaptureModes(),
            CaptureService.GetCaptureCAStatus(),
            CaptureService.GetCurrentCaptureSession(),
            CaptureService.RecoverCaptureSessions(),
            CaptureService.ListCaptureBackends(),
          ]);
        if (disposed) return;
        // The contract governs the row cap, the resync path, and the two
        // handoffs below, so it is adopted before any of them runs (H-RG4 R8).
        dispatch({ type: "contract", contract });
        const renderer = resolveLiveCaptureContract(contract);
        setModes(nextModes as unknown as Mode[]);
        setBackends(nextBackends);
        setCAStatus(nextCA as unknown as CAStatus);
        const recoveryReports = reports as unknown as Array<{
          recovered: boolean;
          records: number;
          discardedBytes: number;
        }>;
        const recovered = recoveryReports.filter((report) => report.recovered);
        if (recovered.length > 0) {
          setRecovery({
            sessions: recovered.length,
            records: recovered.reduce((sum, report) => sum + report.records, 0),
            discardedBytes: recovered.reduce(
              (sum, report) => sum + report.discardedBytes,
              0,
            ),
          });
        }
        const session = current as unknown as LiveCaptureSession;
        if (renderer.restoreCurrentSessionOnPageReentry && session.sessionId) {
          await hydrate(session);
        }
      } catch (caught) {
        if (!disposed) {
          dispatch({
            type: "error",
            message: caught instanceof Error ? caught.message : String(caught),
          });
        }
      }
    };

    const offStarted = Events.On("capture:started", (event: unknown) => {
      const data = eventData<LiveCaptureSession>(event);
      if (data) dispatch({ type: "started", session: data });
    });
    const offTransactions = Events.On(
      "capture:transactions",
      (event: unknown) => {
        const data = eventData<LiveTransactionsEvent>(event);
        if (data) dispatch({ type: "transactions", event: data });
      },
    );
    const offProgress = Events.On("capture:progress", (event: unknown) => {
      const data = eventData<LiveProgressEvent>(event);
      if (data) dispatch({ type: "progress", event: data });
    });
    const offStats = Events.On("capture:stats", (event: unknown) => {
      const data = eventData<LiveCaptureStats>(event);
      if (data) dispatch({ type: "stats", stats: data });
    });
    const offStopped = Events.On("capture:stopped", (event: unknown) => {
      const data = eventData<LiveCaptureSession>(event);
      if (data) dispatch({ type: "stopped", session: data });
    });
    const offError = Events.On("capture:error", (event: unknown) => {
      const data = eventData<{ sessionId: string; message: string }>(event);
      if (data) dispatch({ type: "error", message: data.message });
    });

    void initialize();
    return () => {
      disposed = true;
      offStarted?.();
      offProgress?.();
      offTransactions?.();
      offStats?.();
      offStopped?.();
      offError?.();
    };
  }, [hydrate]);

  useEffect(() => {
    if (!state.needsResync || !state.session?.sessionId) return;
    let disposed = false;
    const resync = async () => {
      try {
        const [stats, transactions] = await Promise.all([
          CaptureService.GetCaptureStats(state.session!.sessionId),
          CaptureService.GetCaptureLiveWindow(state.session!.sessionId),
        ]);
        if (!disposed) {
          dispatch({
            type: "resynced",
            stats: stats as unknown as LiveCaptureStats,
            transactions: transactions as unknown as LiveCaptureTransaction[],
          });
        }
      } catch (caught) {
        if (!disposed) {
          dispatch({
            type: "error",
            message: caught instanceof Error ? caught.message : String(caught),
          });
        }
      }
    };
    void resync();
    return () => {
      disposed = true;
    };
  }, [state.needsResync, state.session?.sessionId]);

  const start = useCallback(async () => {
    if (!firstUseAccepted || state.busy || active || (processScope === "selected" && selectedProcesses.length === 0)) return;
    dispatch({ type: "busy", busy: true });
    dispatch({ type: "clearError" });
    try {
      const session = await CaptureService.StartCapture({
        backend,
        listenAddress: "127.0.0.1:0",
        liveWindow: 5000,
        processScope,
        allowedProcesses: processScope === "selected" ? selectedProcesses.map((p) => p.key) : [],
        launchPath,
        launchArgs: processScope === "launch" ? (() => {
          const args: unknown = JSON.parse(launchArgs);
          if (!Array.isArray(args) || args.some(arg => typeof arg !== "string")) throw new Error(t("httpCaptureLaunchArgsError"));
          return args as string[];
        })() : [],
        retainUnattributedMetadata: retainUnattributed,
        captureBodies,
        maxBodyBytes: state.contract.maxBodyBytes,
      });
      onStarted();
      dispatch({
        type: "started",
        session: session as unknown as LiveCaptureSession,
      });
    } catch (caught) {
      dispatch({
        type: "error",
        message: caught instanceof Error ? caught.message : String(caught),
      });
    }
  }, [
    active,
    captureBodies,
    firstUseAccepted,
    processScope,
    backend,
    launchPath,
    launchArgs,
    selectedProcesses,
    onStarted,
    retainUnattributed,
    state.busy,
    state.contract.maxBodyBytes,
  ]);

  const stop = useCallback(async () => {
    if (!state.session?.sessionId || state.busy) return;
    dispatch({ type: "busy", busy: true });
    try {
      const session = await CaptureService.StopCapture(state.session.sessionId);
      dispatch({
        type: "stopped",
        session: session as unknown as LiveCaptureSession,
      });
      setCAStatus(
        (await CaptureService.GetCaptureCAStatus()) as unknown as CAStatus,
      );
    } catch (caught) {
      dispatch({
        type: "error",
        message: caught instanceof Error ? caught.message : String(caught),
      });
    }
  }, [state.busy, state.session?.sessionId]);

  const pause = useCallback(async () => {
    if (!state.session || !active || state.busy) return;
    dispatch({type:"busy",busy:true});
    try {
      const next=await CaptureService.SetCapturePaused(state.session.sessionId, !state.session.paused);
      dispatch({type:"session",session:next as unknown as LiveCaptureSession});
    } catch(caught) { dispatch({type:"error",message:String(caught)}); }
  },[state.session,active,state.busy]);

  const clear = useCallback(async () => {
    if (!state.session || state.busy) return;
    if (!window.confirm(t("httpCaptureClearConfirm").replace("{count}",String(state.stats?.persisted ?? state.transactions.length)))) return;
    dispatch({type:"busy",busy:true});
    try {
      const next=await CaptureService.ClearCapture(state.session.sessionId);
      onStarted();
      dispatch({type:"session",session:next as unknown as LiveCaptureSession});
    } catch(caught) { dispatch({type:"error",message:String(caught)}); }
  },[state.session,state.busy,state.stats?.persisted,state.transactions.length,onStarted,t]);

  useHttpCommands({
    start: () => void start(), stop: () => void stop(), pause: () => void pause(),
    clear: () => void clear(), follow: () => dispatch({type:"follow",follow:!state.follow}),
  });

  const installCA = useCallback(async () => {
    dispatch({ type: "busy", busy: true });
    try {
      setCAStatus(
        (await CaptureService.InstallCaptureCA()) as unknown as CAStatus,
      );
      dispatch({ type: "busy", busy: false });
    } catch (caught) {
      dispatch({
        type: "error",
        message: caught instanceof Error ? caught.message : String(caught),
      });
    }
  }, []);

  const removeCA = useCallback(async () => {
    dispatch({ type: "busy", busy: true });
    try {
      setCAStatus(
        (await CaptureService.RemoveCaptureCA()) as unknown as CAStatus,
      );
      dispatch({ type: "busy", busy: false });
    } catch (caught) {
      dispatch({
        type: "error",
        message: caught instanceof Error ? caught.message : String(caught),
      });
    }
  }, []);

  const loadFinalized = useCallback(async () => {
    if (
      !state.session?.sessionId ||
      active ||
      analysisBusy ||
      !state.contract.finalizedSessionUsesAnalysisResult
    ) {
      return;
    }
    setAnalysisBusy(true);
    try {
      const result = await CaptureService.AnalyzeCaptureSession({
        sessionId: state.session.sessionId,
      });
      onFinalized(
        result as unknown as HttpCaptureAnalysisResult,
        state.session.sessionId,
      );
    } catch (caught) {
      dispatch({
        type: "error",
        message: caught instanceof Error ? caught.message : String(caught),
      });
    } finally {
      setAnalysisBusy(false);
    }
  }, [
    active,
    analysisBusy,
    onFinalized,
    state.contract.finalizedSessionUsesAnalysisResult,
    state.session?.sessionId,
  ]);

  const coverage = useMemo(
    () => buildLiveCoverageDisclosure(state.stats),
    [state.stats],
  );
  // While a session runs the engine's value is the policy in force; the local
  // checkbox only seeds the next start (H-RG4 L6).
  const effectiveRetainUnattributed = activeUnattributedPolicy(
    state.session,
    retainUnattributed,
  );
  const effectiveCaptureBodies = activeBodyCapturePolicy(state.session, captureBodies);
  const mode = modes[0] ?? null;
  const caTrusted = caStatus.state === "trusted";


  return (
    <Card className="min-w-0">
      <CardHeader className="flex-row items-start justify-between gap-3">
        <div>
          <CardTitle>{t("liveCaptureTitle")}</CardTitle>
          <p className="mt-1 text-xs text-muted-foreground">
            {t("liveCaptureDescription")}
          </p>
        </div>
        <span
          className={`rounded-full px-2 py-1 text-xs font-medium ${
            active
              ? "bg-emerald-500/15 text-emerald-700 dark:text-emerald-400"
              : "bg-muted text-muted-foreground"
          }`}
        >
          {state.session
            ? t(SESSION_STATE_LABEL_KEYS[resolveLiveSessionState(state.session.state)])
            : t("liveCaptureIdle")}
        </span>
      </CardHeader>
      <CardContent className="flex min-w-0 flex-col gap-2">
        <details className="rounded border p-2">
          <summary className="cursor-pointer text-xs">{t("httpCaptureSectionSource")}</summary>
          <div className="flex flex-col gap-2 pt-2">
          <label className="text-xs">{t("httpBackendTitle")} <select aria-label={t("httpBackendTitle")} value={backend} disabled={active} onChange={event=>setBackend(event.target.value)}>{backends.map(item=><option key={item.name} value={item.name} disabled={!item.available}>{t(item.transparent?"httpBackendTransparent":"httpBackendExplicit")}{item.available?"":` · ${t("httpBackendUnavailable")}`}</option>)}</select></label>
          <p className="text-xs text-muted-foreground">{t("httpBackendCoverage")}</p>
        <div className="rounded-md border border-amber-500/40 bg-amber-500/10 p-3">
          <div className="flex items-start gap-2">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" />
            <div className="space-y-2 text-xs">
              <p className="font-medium">{t("liveCaptureFirstUseTitle")}</p>
              <p className="text-muted-foreground">
                {t("liveCaptureFirstUseBody")}
              </p>
              <label className="inline-flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={firstUseAccepted}
                  disabled={active}
                  onChange={(event) =>
                    setFirstUseAccepted(event.target.checked)
                  }
                />
                {t("liveCaptureFirstUseAccept")}
              </label>
            </div>
          </div>
        </div>

        <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
          <div className="rounded-md border border-border p-3">
            <div className="mb-2 flex items-center justify-between gap-2">
              <div>
                <p className="text-sm font-medium">{t("liveCaptureCATitle")}</p>
                <p className="text-xs text-muted-foreground">
                  {t(CA_STATE_LABEL_KEYS[resolveLiveCAState(caStatus.state)])}
                  {caStatus.stores.length > 0
                    ? ` · ${caStatus.stores.join(", ")}`
                    : ""}
                </p>
              </div>
              {caTrusted ? (
                <ShieldCheck className="h-5 w-5 text-emerald-500" />
              ) : (
                <ShieldOff className="h-5 w-5 text-muted-foreground" />
              )}
            </div>
            {caStatus.error && (
              <p className="mb-2 text-xs text-red-600">{caStatus.error}</p>
            )}
            <div className="flex gap-2">
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={
                  state.busy || active || caTrusted || !firstUseAccepted
                }
                onClick={() => void installCA()}
              >
                {t("liveCaptureCAInstall")}
              </Button>
              <Button
                type="button"
                size="sm"
                variant="outline"
                disabled={state.busy || active || !caTrusted}
                onClick={() => void removeCA()}
              >
                {t("liveCaptureCARemove")}
              </Button>
            </div>
          </div>

          <div className="rounded-md border border-border p-3 text-xs">
            <p className="text-sm font-medium">{t("liveCaptureModeTitle")}</p>
            <p className="mt-1 font-mono">{mode?.name ?? "-"}</p>
            <p className="mt-1 text-muted-foreground">
              {mode?.fidelity || t("liveCaptureModeUnavailable")}
            </p>
            {mode && !mode.available && (
              <p className="mt-1 text-amber-700 dark:text-amber-400">
                {mode.reason || t("liveCaptureModeUnavailable")}
              </p>
            )}
            <p className="mt-2 text-muted-foreground">
              {effectiveCaptureBodies
                ? t("liveCaptureBodyOptInOn")
                : t("liveCaptureBodyOptInOff")}
            </p>
          </div>
        </div>

        {state.contract.bodyCaptureAvailable && (
          <label className="flex items-start gap-2 rounded-md border border-amber-500/40 bg-amber-500/5 p-3 text-xs">
            <input
              type="checkbox"
              className="mt-0.5"
              checked={effectiveCaptureBodies}
              disabled={active}
              onChange={(event) => setCaptureBodies(event.target.checked)}
            />
            <span>
              <span className="block font-medium">{t("liveCaptureBodyOptIn")}</span>
              <span className="text-muted-foreground">{t("liveCaptureBodyOptInHint")}</span>
              {/* This branch is reachable only when the engine contract makes
                  the tier available. A pending SEC-10 contract is fail-closed
                  and does not render the control. */}
              <span className="mt-1 block text-amber-700 dark:text-amber-400">
                {t("liveCaptureBodyGateWarning")}
              </span>
              {/* Name the gate that permitted this, so an operator enabling
                  plaintext retention can see what was actually established
                  rather than only that a checkbox exists. */}
              <span className="mt-1 block text-muted-foreground">
                {t("liveCaptureBodyGateVerified")}
              </span>
              {active && (
                <span className="mt-1 block font-medium text-amber-700 dark:text-amber-400">
                  {effectiveCaptureBodies
                    ? t("liveCaptureBodyOptInOn")
                    : t("liveCaptureBodyOptInOff")}{" "}
                  {t("liveCaptureUnknownOptInLocked")}
                </span>
              )}
            </span>
          </label>
        )}

        {/* An absent checkbox is not self-explanatory. When the preflight did
            not verify, say so and say why — otherwise the operator reads a
            missing capability as a missing feature. */}
        {!state.contract.bodyCaptureAvailable && (
          <div className="rounded-md border border-border bg-muted/30 p-3 text-xs">
            <span className="block font-medium">{t("liveCaptureBodyUnavailable")}</span>
            <span className="mt-1 block text-muted-foreground">
              {bodyGateReasonLabel(state.contract.bodyCaptureGateReason, locale)}
            </span>
          </div>
        )}

        <fieldset disabled={active} className="rounded border p-2 text-xs">
          <legend>{t("httpCaptureProcessScope")}</legend>
          <select aria-label={t("httpCaptureProcessScope")} value={processScope}
            className="h-8 rounded border px-2"
            onChange={(event) => { setProcessScope(event.target.value); if(event.target.value==="selected") void loadProcesses(); }}>
            <option value="all_sessions">{t("httpCaptureAllSessions")}</option>
            <option value="current_session">{t("httpCaptureCurrentSession")}</option>
            <option value="selected">{t("httpCaptureSelectedProcesses")}</option>
            <option value="launch">{t("httpCaptureLaunch")}</option>
          </select>
          <p className="mt-1 text-muted-foreground">{t("httpCaptureScopeCoverage")}</p>
          {processScope==="launch" && <div className="mt-2 flex flex-col gap-2">
            <Button type="button" size="sm" variant="outline" onClick={() => void chooseExecutable()}>{t("browseFile")}</Button>
            <p className="break-all font-mono">{launchPath}</p>
            <label>{t("httpCaptureLaunchArgs")}<input value={launchArgs} onChange={(event) => setLaunchArgs(event.target.value)} className="ml-2 rounded border px-2 py-1 font-mono" /></label>
          </div>}
          {processScope === "selected" && (
            <div className="mt-2">
              <Button type="button" size="sm" variant="outline" onClick={() => void loadProcesses()}>{t("httpCaptureRefreshProcesses")}</Button>
              <div className="mt-2 max-h-40 overflow-auto">
                {processes.map((process) => (
                  <label key={JSON.stringify(process.key)} className="flex gap-2 py-1">
                    <input type="checkbox" checked={selectedProcesses.some((p) => p.key.pid === process.key.pid && p.key.startTime === process.key.startTime)}
                      onChange={(event) => setSelectedProcesses((current) => event.target.checked
                        ? [...current, process]
                        : current.filter((p) => p.key.pid !== process.key.pid || p.key.startTime !== process.key.startTime))} />
                    {process.name} [{process.key.pid}] · {process.key.startTime}
                  </label>
                ))}
              </div>
              <p>{selectedProcesses.length} {t("httpCaptureSelectedProcesses")}</p>
            </div>
          )}
        </fieldset>
        <label className="flex items-start gap-2 rounded-md border border-border p-3 text-xs">
          <input
            type="checkbox"
            className="mt-0.5"
            checked={effectiveRetainUnattributed}
            disabled={active}
            onChange={(event) => setRetainUnattributed(event.target.checked)}
          />
          <span>
            <span className="block font-medium">
              {t("liveCaptureUnknownOptIn")}
            </span>
            <span className="text-muted-foreground">
              {t("liveCaptureUnknownOptInHint")}
            </span>
            {active && (
              <span className="mt-1 block font-medium text-amber-700 dark:text-amber-400">
                {effectiveRetainUnattributed
                  ? t("liveCaptureUnknownOptInOn")
                  : t("liveCaptureUnknownOptInOff")}{" "}
                {t("liveCaptureUnknownOptInLocked")}
              </span>
            )}
          </span>
        </label>

          </div>
        </details>
        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            disabled={
              state.busy ||
              active ||
              !firstUseAccepted ||
              (!mode?.available) || (processScope === "selected" && selectedProcesses.length === 0) || (processScope === "launch" && !launchPath)
            }
            onClick={() => void start()}
          >
            {state.busy && !active ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Play className="h-4 w-4" />
            )}
            {t("liveCaptureStart")}
          </Button>
          <Button
            type="button"
            variant="destructive"
            disabled={state.busy || !active}
            onClick={() => void stop()}
          >
            {state.busy && active ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <CircleStop className="h-4 w-4" />
            )}
            {t("liveCaptureStop")}
          </Button>
          <Button type="button" size="sm" variant="outline"
            onClick={() => dispatch({ type: "follow", follow: !state.follow })}>
            {t(state.follow ? "liveCaptureFollowing" : "liveCaptureResumeFollow")}
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={!active || state.busy} onClick={() => void pause()}>
            {t(state.session?.paused ? "httpCaptureResume" : "httpCapturePause")}
          </Button>
          <Button type="button" size="sm" variant="outline" disabled={!state.session || state.busy} onClick={() => void clear()}>
            {t("httpCaptureClear")}
          </Button>
          {state.session?.paused && <span role="status" className="text-xs text-amber-700">{t("httpCapturePaused")}</span>}
          {state.session &&
            !active &&
            state.contract.finalizedSessionUsesAnalysisResult && (
              <Button
                type="button"
                variant="outline"
                disabled={analysisBusy}
                onClick={() => void loadFinalized()}
              >
                {analysisBusy && <Loader2 className="h-4 w-4 animate-spin" />}
                {t("liveCaptureLoadFinalized")}
              </Button>
            )}
          {state.session?.listenAddress && (
            <span className="font-mono text-xs text-muted-foreground">
              proxy://{state.session.listenAddress}
            </span>
          )}
        </div>

        {showAnalyzer && <LiveHttpAnalyzer key={`${state.session?.sessionId ?? "idle"}:${state.session?.generation ?? 0}`}
          sessionId={state.session?.sessionId ?? ""} transactions={state.transactions}
          persisted={state.stats?.persisted ?? 0} follow={state.follow} />}
        <footer role="status" className="flex flex-wrap items-center gap-3 border-t pt-1 text-[11px]">
          <span>{state.session ? t(SESSION_STATE_LABEL_KEYS[resolveLiveSessionState(state.session.state)]) : t("liveCaptureIdle")}</span>
          <span>{t("liveCaptureCaptured")}: {state.stats?.captured ?? 0}</span>
          <span>{t("liveCapturePersisted")}: {state.stats?.persisted ?? 0}</span>
          {state.session?.allowedProcesses?.length ? <span>{t("httpCaptureSelectedProcesses")}: {state.session.allowedProcesses.length}</span> : null}
        </footer>
        <details>
          <summary className="cursor-pointer text-xs">{t("httpCaptureSectionSession")}</summary>
        {state.error && (
          <div
            className="rounded-md border border-red-500/40 bg-red-500/10 p-2 text-xs text-red-700 dark:text-red-400"
            role="alert"
          >
            {state.error}
          </div>
        )}
        {state.contractMismatch && (
          <div
            className="rounded-md border border-amber-500/40 bg-amber-500/10 p-2 text-xs text-amber-800 dark:text-amber-300"
            role="status"
          >
            {t("liveCaptureContractMismatch")}
          </div>
        )}
        {recovery && (
          <div
            className="rounded-md border border-sky-500/40 bg-sky-500/10 p-2 text-xs"
            role="status"
          >
            {t("liveCaptureRecovery")}: {formatNumber(recovery.sessions)} ·{" "}
            {formatNumber(recovery.records)} {t("liveCaptureRecords")} ·{" "}
            {formatBytes(recovery.discardedBytes)} {t("liveCaptureDiscarded")}
          </div>
        )}

        {state.stats && (
          <>
            <div className="grid gap-2 sm:grid-cols-3 lg:grid-cols-9">
              <LiveStat label={t("liveCaptureObserved")} value={state.stats.observed} />
              <LiveStat label={t("liveCaptureCaptured")} value={state.stats.captured} />
              <LiveStat label={t("liveCapturePersisted")} value={state.stats.persisted} />
              <LiveStat label={t("liveCaptureDropped")} value={state.stats.dropped} warning={state.stats.dropped > 0} />
              <LiveStat label={t("liveCaptureUnattributed")} value={state.stats.unattributed} warning={state.stats.unattributed > 0} />
              <LiveStat label={t("liveCaptureKernelDropped")} value={state.stats.kernelDropped} warning={state.stats.kernelDropped > 0} />
              <LiveStat label={t("liveCaptureUnsupported")} value={state.stats.unsupported} warning={state.stats.unsupported > 0} />
              <LiveStat label={t("liveCapturePassthrough")} value={state.stats.passthrough} warning={state.stats.passthrough > 0} />
              <LiveStat label={t("liveCaptureEventSkipped")} value={state.stats.eventSkipped} warning={state.stats.eventSkipped > 0} />
            </div>
            <p className="text-[10px] text-muted-foreground">
              {t("liveCaptureCoverageDenominator")}
              {coverage?.droppedPercent !== null &&
                coverage?.droppedPercent !== undefined && (
                  <>
                    {" "}
                    <span className="font-mono">
                      {coverage.droppedPercent.toFixed(1)}%
                    </span>{" "}
                    {t("liveCaptureDroppedShare")}.
                  </>
                )}{" "}
              {t("liveCaptureStoreBytes")}:{" "}
              <span className="font-mono">
                {formatBytes(state.stats.storeBytes)}
              </span>
            </p>
            {(state.stats.backpressured ||
              state.stats.eventSkipped > 0 ||
              state.stats.unsupported > 0 ||
              state.stats.passthrough > 0 ||
              state.stats.kernelDropped > 0 ||
              coverage?.hasDrops ||
              coverage?.hasUnattributed) && (
              <div className="space-y-1 rounded-md border border-amber-500/40 bg-amber-500/10 p-2 text-xs text-amber-800 dark:text-amber-300">
                {state.stats.backpressured && (
                  <p>{t("liveCaptureBackpressureWarning")}</p>
                )}
                {state.stats.eventSkipped > 0 && (
                  <p>{t("liveCaptureEventLossWarning")}</p>
                )}
                {coverage?.hasDrops && <p>{t("liveCaptureDropWarning")}</p>}
                {coverage?.hasUnattributed && (
                  <p>{t("liveCaptureUnattributedWarning")}</p>
                )}
                {state.stats.kernelDropped > 0 && (
                  <p>{t("liveCaptureKernelDroppedWarning")}</p>
                )}
                {(state.stats.unsupported > 0 ||
                  state.stats.passthrough > 0) && (
                  <p>{t("liveCaptureFidelityWarning")}</p>
                )}
              </div>
            )}
          </>
        )}


        </details>
      </CardContent>
    </Card>
  );
}

function LiveStat({
  label,
  value,
  warning = false,
}: {
  label: string;
  value: string | number;
  warning?: boolean;
}): React.JSX.Element {
  return (
    <div className="rounded-md border border-border p-2">
      <p className="truncate text-[10px] uppercase tracking-wide text-muted-foreground">
        {label}
      </p>
      <p
        className={`mt-1 font-mono text-sm font-semibold ${
          warning ? "text-amber-700 dark:text-amber-400" : ""
        }`}
      >
        {typeof value === "number" ? formatNumber(value) : value}
      </p>
    </div>
  );
}
