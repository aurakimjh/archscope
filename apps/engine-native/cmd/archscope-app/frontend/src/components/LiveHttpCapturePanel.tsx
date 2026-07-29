import { Events } from "@wailsio/runtime";
import {
  AlertTriangle,
  CircleStop,
  Loader2,
  Play,
  RotateCcw,
  ShieldCheck,
  ShieldOff,
} from "lucide-react";
import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
} from "react";

import type { HttpCaptureAnalysisResult } from "@/bridge/types";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { useI18n } from "@/i18n/I18nProvider";
import type { MessageKey } from "@/i18n/messages";
import {
  activeUnattributedPolicy,
  buildLiveCoverageDisclosure,
  buildLiveProcessGroups,
  countLiveInFlight,
  initialLiveCaptureState,
  isLiveSessionActive,
  isLiveTransactionInFlight,
  liveFidelityTone,
  liveHttpCaptureReducer,
  resolveLiveAttribution,
  resolveLiveCAState,
  resolveLiveFidelity,
  resolveLiveSessionState,
  resolveLiveCaptureContract,
  resolveLiveTransactionState,
  type LiveAttributionToken,
  type LiveCAStateToken,
  type LiveCaptureSession,
  type LiveCaptureStats,
  type LiveCaptureTransaction,
  type LiveFidelityToken,
  type LiveFidelityTone,
  type LiveProgressEvent,
  type LiveSessionStateToken,
  type LiveTransactionStateToken,
  type LiveTransactionsEvent,
} from "@/state/liveHttpCapture";
import { formatBytes } from "@/state/httpCapture";
import { formatMilliseconds, formatNumber } from "@/utils/formatters";
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
};

/**
 * Every fidelity grade the live table can print. `resolveLiveFidelity` maps any
 * unrecognized engine value to `unknown`, so an unexpected grade degrades to
 * "not yet determined" instead of a reassuring string (H-RG4 L1).
 */
const FIDELITY_LABEL_KEYS: Record<LiveFidelityToken, MessageKey> = {
  pending: "liveCaptureFidelityPending",
  decoded_wire: "liveCaptureFidelityDecodedWire",
  semantic: "liveCaptureFidelitySemantic",
  unsupported: "liveCaptureFidelityUnsupported",
  passthrough: "liveCaptureFidelityPassthrough",
  unknown: "liveCaptureFidelityUnknown",
};

/**
 * A grade that does not assert ArchScope read the exchange must not be styled
 * like ordinary captured traffic, so the caution emphasis is derived from
 * `liveFidelityTone` rather than from the label text (H-RG4 R9).
 */
const FIDELITY_TONE_CLASSES: Record<LiveFidelityTone, string> = {
  decoded: "",
  pending: "text-muted-foreground",
  limited: "text-amber-700 dark:text-amber-400",
};

/**
 * Raw engine enums the panel prints. Rendering the wire token left these cells
 * untranslated in both locales (H-RG4 R11); every token now resolves through a
 * closed map, and anything unrecognized says so instead of leaking the token.
 */
const TX_STATE_LABEL_KEYS: Record<LiveTransactionStateToken, MessageKey> = {
  request_sent: "liveCaptureTxStateRequestSent",
  receiving: "liveCaptureTxStateReceiving",
  complete: "liveCaptureTxStateComplete",
  failed: "liveCaptureTxStateFailed",
  aborted: "liveCaptureTxStateAborted",
  unknown: "liveCaptureTxStateUnknown",
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

const ATTRIBUTION_LABEL_KEYS: Record<LiveAttributionToken, MessageKey> = {
  confirmed: "liveCaptureAttributionConfirmed",
  inferred: "liveCaptureAttributionInferred",
  unknown: "liveCaptureAttributionUnknown",
};

function eventData<T>(event: unknown): T | null {
  if (!event || typeof event !== "object") return null;
  const wrapped = event as { data?: unknown };
  return (wrapped.data ?? event) as T;
}

export function LiveHttpCapturePanel({
  onStarted,
  onFinalized,
}: LiveHttpCapturePanelProps): React.JSX.Element {
  const { t } = useI18n();
  const [state, dispatch] = useReducer(
    liveHttpCaptureReducer,
    initialLiveCaptureState,
  );
  const [modes, setModes] = useState<Mode[]>([]);
  const [caStatus, setCAStatus] = useState<CAStatus>({
    state: "loading",
    stores: [],
  });
  const [firstUseAccepted, setFirstUseAccepted] = useState(false);
  const [retainUnattributed, setRetainUnattributed] = useState(false);
  const [recovery, setRecovery] = useState<RecoverySummary | null>(null);
  const [analysisBusy, setAnalysisBusy] = useState(false);
  const listRef = useRef<HTMLDivElement | null>(null);
  const active = isLiveSessionActive(state.session);

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
        const [contract, nextModes, nextCA, current, reports] =
          await Promise.all([
            CaptureService.GetLiveCaptureContract(),
            CaptureService.ListCaptureModes(),
            CaptureService.GetCaptureCAStatus(),
            CaptureService.GetCurrentCaptureSession(),
            CaptureService.RecoverCaptureSessions(),
          ]);
        if (disposed) return;
        // The contract governs the row cap, the resync path, and the two
        // handoffs below, so it is adopted before any of them runs (H-RG4 R8).
        dispatch({ type: "contract", contract });
        const renderer = resolveLiveCaptureContract(contract);
        setModes(nextModes as unknown as Mode[]);
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

  useEffect(() => {
    if (!state.follow) return;
    const element = listRef.current;
    if (element) element.scrollTop = element.scrollHeight;
  }, [state.follow, state.transactions]);

  const start = useCallback(async () => {
    if (!firstUseAccepted || state.busy || active) return;
    dispatch({ type: "busy", busy: true });
    dispatch({ type: "clearError" });
    try {
      const session = await CaptureService.StartCapture({
        listenAddress: "127.0.0.1:0",
        liveWindow: 5000,
        retainUnattributedMetadata: retainUnattributed,
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
    firstUseAccepted,
    onStarted,
    retainUnattributed,
    state.busy,
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

  const processGroups = useMemo(
    () => buildLiveProcessGroups(state.transactions),
    [state.transactions],
  );
  const inFlightCount = useMemo(
    () => countLiveInFlight(state.transactions),
    [state.transactions],
  );
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
  const mode = modes[0] ?? null;
  const caTrusted = caStatus.state === "trusted";

  const onListScroll = useCallback(() => {
    const element = listRef.current;
    if (!element) return;
    const distance =
      element.scrollHeight - element.scrollTop - element.clientHeight;
    const follow = distance < 32;
    if (follow !== state.follow) dispatch({ type: "follow", follow });
  }, [state.follow]);

  return (
    <Card>
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
      <CardContent className="flex flex-col gap-4">
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
              {t("liveCaptureBodyDisabled")}
            </p>
          </div>
        </div>

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

        <div className="flex flex-wrap items-center gap-2">
          <Button
            type="button"
            disabled={
              state.busy ||
              active ||
              !firstUseAccepted ||
              (mode !== null && !mode.available)
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

        {state.session && (
          <div className="grid gap-3 lg:grid-cols-[minmax(0,0.65fr)_minmax(0,1.35fr)]">
            <div className="rounded-md border border-border p-3">
              <p className="mb-2 text-sm font-medium">
                {t("liveCaptureProcessTree")}
              </p>
              {processGroups.length === 0 ? (
                <p className="text-xs text-muted-foreground">
                  {t("liveCaptureNoRows")}
                </p>
              ) : (
                <ul className="space-y-1">
                  {processGroups.map((group) => (
                    <li
                      key={group.key}
                      className="flex items-center gap-2 rounded bg-muted/40 px-2 py-1 text-xs"
                    >
                      <span className="min-w-0 flex-1 truncate">
                        {group.label}
                      </span>
                      <span className="text-[10px] text-muted-foreground">
                        {t(
                          ATTRIBUTION_LABEL_KEYS[
                            resolveLiveAttribution(group.attribution)
                          ],
                        )}
                      </span>
                      <span className="tabular-nums">
                        {group.count}
                        {group.errors > 0 && (
                          <span className="ml-1 text-red-600">
                            ({group.errors})
                          </span>
                        )}
                      </span>
                    </li>
                  ))}
                </ul>
              )}
            </div>

            <div className="rounded-md border border-border p-3">
              <div className="mb-2 flex items-center justify-between gap-2">
                <p className="text-sm font-medium">
                  {t("liveCaptureRows")}
                  {inFlightCount > 0 && (
                    <span className="ml-2 rounded-full bg-muted px-2 py-0.5 text-[10px] font-normal text-muted-foreground">
                      {formatNumber(inFlightCount)}{" "}
                      {t("liveCaptureInFlightCount")}
                    </span>
                  )}
                </p>
                <Button
                  type="button"
                  size="sm"
                  variant={state.follow ? "secondary" : "outline"}
                  onClick={() =>
                    dispatch({ type: "follow", follow: !state.follow })
                  }
                >
                  <RotateCcw className="h-3.5 w-3.5" />
                  {state.follow
                    ? t("liveCaptureFollowing")
                    : t("liveCaptureResumeFollow")}
                </Button>
              </div>
              <div
                ref={listRef}
                className="max-h-72 overflow-auto"
                onScroll={onListScroll}
              >
                {state.transactions.length === 0 ? (
                  <p className="py-4 text-center text-xs text-muted-foreground">
                    {t("liveCaptureNoRows")}
                  </p>
                ) : (
                  <table className="w-full text-left text-xs">
                    <thead className="sticky top-0 bg-background">
                      <tr className="border-b border-border text-muted-foreground">
                        <th className="px-2 py-1">{t("httpCaptureColMethod")}</th>
                        <th className="px-2 py-1">{t("httpCaptureColUrl")}</th>
                        <th className="px-2 py-1">{t("httpCaptureColStatus")}</th>
                        <th className="px-2 py-1">{t("liveCaptureColState")}</th>
                        <th className="px-2 py-1">{t("httpCaptureColDuration")}</th>
                        <th className="px-2 py-1">{t("httpCaptureFidelityLabel")}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {state.transactions.map((transaction) => {
                        const inFlight =
                          isLiveTransactionInFlight(transaction);
                        const fidelity = resolveLiveFidelity(
                          transaction.fidelity,
                        );
                        const txState = resolveLiveTransactionState(
                          transaction.state,
                        );
                        return (
                          <tr
                            key={transaction.id}
                            className={`border-b border-border/60 ${
                              inFlight ? "text-muted-foreground" : ""
                            }`}
                          >
                            <td className="px-2 py-1 font-mono">
                              {transaction.method}
                            </td>
                            <td
                              className="max-w-80 truncate px-2 py-1"
                              title={transaction.url}
                            >
                              {transaction.host}
                              {transaction.path}
                            </td>
                            <td className="px-2 py-1 font-mono">
                              {inFlight ? "—" : transaction.statusCode || "—"}
                            </td>
                            <td className="px-2 py-1" title={transaction.error}>
                              {t(TX_STATE_LABEL_KEYS[txState])}
                              {inFlight && ` · ${t("liveCaptureInFlight")}`}
                            </td>
                            <td className="px-2 py-1 tabular-nums">
                              {inFlight
                                ? "—"
                                : formatMilliseconds(
                                    Math.round(transaction.totalMs),
                                  )}
                            </td>
                            <td
                              className={`px-2 py-1 ${
                                FIDELITY_TONE_CLASSES[liveFidelityTone(fidelity)]
                              }`}
                              title={transaction.fidelity}
                            >
                              {t(FIDELITY_LABEL_KEYS[fidelity])}
                            </td>
                          </tr>
                        );
                      })}
                    </tbody>
                  </table>
                )}
              </div>
              <p className="mt-2 text-[10px] text-muted-foreground">
                {t("liveCaptureRowCapPrefix")}{" "}
                {formatNumber(state.contract.transactionRowCap)}{" "}
                {t("liveCaptureRowCapSuffix")} {t("liveCaptureInFlightHint")}
              </p>
            </div>
          </div>
        )}
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
