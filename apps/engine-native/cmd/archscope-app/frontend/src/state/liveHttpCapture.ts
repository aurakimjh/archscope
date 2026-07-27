export const LIVE_TRANSACTION_ROW_CAP = 500;

export type LiveCaptureSession = {
  sessionId: string;
  state: string;
  listenAddress: string;
  storePath: string;
  startedAt: string | null;
  endedAt?: string | null;
  error?: string;
};

export type LiveCaptureStats = {
  sessionId: string;
  state: string;
  captured: number;
  persisted: number;
  bodyOmitted: number;
  eventSkipped: number;
  kernelDropped: number;
  parseFailed: number;
  unsupported: number;
  passthrough: number;
  unattributed: number;
  dropped: number;
  backpressured: boolean;
  snapshotVersion: number;
  sequence: number;
  storeBytes: number;
};

export type LiveCaptureProcess = {
  key: {
    pid: number;
    startTime: string;
  };
  name: string;
  execPath?: string;
  parentPid?: number;
  attribution: string;
};

export type LiveCaptureTransaction = {
  id: string;
  sequence: number;
  method: string;
  url: string;
  host: string;
  path: string;
  statusCode: number;
  statusText?: string;
  httpVersion: string;
  state: string;
  totalMs: number;
  captureMode: string;
  coverage: string;
  fidelity: string;
  process?: LiveCaptureProcess | null;
  error?: string;
};

export type LiveTransactionsEvent = {
  sessionId: string;
  sequence: number;
  snapshotVersion: number;
  items: LiveCaptureTransaction[];
};

export type LiveProgressEvent = {
  sessionId: string;
  transaction: LiveCaptureTransaction;
};

export type LiveCaptureState = {
  session: LiveCaptureSession | null;
  stats: LiveCaptureStats | null;
  transactions: LiveCaptureTransaction[];
  follow: boolean;
  needsResync: boolean;
  busy: boolean;
  error: string | null;
};

export const initialLiveCaptureState: LiveCaptureState = {
  session: null,
  stats: null,
  transactions: [],
  follow: true,
  needsResync: false,
  busy: false,
  error: null,
};

export type LiveCaptureAction =
  | {
      type: "hydrate";
      session: LiveCaptureSession;
      stats: LiveCaptureStats | null;
      transactions: LiveCaptureTransaction[];
    }
  | { type: "started"; session: LiveCaptureSession }
  | { type: "stopped"; session: LiveCaptureSession }
  | { type: "stats"; stats: LiveCaptureStats }
  | { type: "progress"; event: LiveProgressEvent }
  | { type: "transactions"; event: LiveTransactionsEvent }
  | { type: "resynced"; stats: LiveCaptureStats; transactions: LiveCaptureTransaction[] }
  | { type: "follow"; follow: boolean }
  | { type: "busy"; busy: boolean }
  | { type: "error"; message: string }
  | { type: "clearError" };

export function liveHttpCaptureReducer(
  state: LiveCaptureState,
  action: LiveCaptureAction,
): LiveCaptureState {
  switch (action.type) {
    case "hydrate":
      return {
        ...state,
        session: action.session,
        stats: action.stats,
        transactions: boundedDistinct(action.transactions),
        needsResync: false,
        busy: false,
        error: action.session.error || null,
      };
    case "started":
      return {
        ...initialLiveCaptureState,
        session: action.session,
      };
    case "stopped":
      if (!matchesSession(state, action.session.sessionId)) return state;
      return {
        ...state,
        session: action.session,
        busy: false,
        error: action.session.error || state.error,
      };
    case "stats": {
      if (!matchesSession(state, action.stats.sessionId)) return state;
      const skippedAdvanced =
        action.stats.eventSkipped > (state.stats?.eventSkipped ?? 0);
      return {
        ...state,
        stats: action.stats,
        needsResync: state.needsResync || skippedAdvanced,
      };
    }
    case "progress":
      if (!matchesSession(state, action.event.sessionId)) return state;
      return {
        ...state,
        transactions: boundedDistinct([
          ...state.transactions,
          action.event.transaction,
        ]),
      };
    case "transactions":
      if (!matchesSession(state, action.event.sessionId)) return state;
      return {
        ...state,
        transactions: boundedDistinct([
          ...state.transactions,
          ...action.event.items,
        ]),
      };
    case "resynced":
      if (!matchesSession(state, action.stats.sessionId)) return state;
      return {
        ...state,
        stats: action.stats,
        transactions: boundedDistinct(action.transactions),
        needsResync: false,
      };
    case "follow":
      return { ...state, follow: action.follow };
    case "busy":
      return { ...state, busy: action.busy };
    case "error":
      return { ...state, busy: false, error: action.message };
    case "clearError":
      return { ...state, error: null };
  }
}

function matchesSession(state: LiveCaptureState, sessionId: string): boolean {
  return Boolean(sessionId) && state.session?.sessionId === sessionId;
}

function boundedDistinct(
  items: LiveCaptureTransaction[],
): LiveCaptureTransaction[] {
  const seen = new Set<string>();
  const reversed: LiveCaptureTransaction[] = [];
  for (let index = items.length - 1; index >= 0; index -= 1) {
    const item = items[index];
    if (!item || seen.has(item.id)) continue;
    seen.add(item.id);
    reversed.push(item);
    if (reversed.length === LIVE_TRANSACTION_ROW_CAP) break;
  }
  reversed.reverse();
  return reversed;
}

export type LiveProcessGroup = {
  key: string;
  label: string;
  attribution: string;
  count: number;
  errors: number;
};

export function buildLiveProcessGroups(
  transactions: LiveCaptureTransaction[],
): LiveProcessGroup[] {
  const groups = new Map<string, LiveProcessGroup>();
  for (const transaction of transactions) {
    const process = transaction.process;
    const key = process
      ? `${process.key.pid}:${process.key.startTime || "unknown"}`
      : "unattributed";
    const current = groups.get(key) ?? {
      key,
      label: process?.name || (process ? `PID ${process.key.pid}` : "Unattributed"),
      attribution: process?.attribution || "unknown",
      count: 0,
      errors: 0,
    };
    current.count += 1;
    if (
      transaction.statusCode >= 400 ||
      transaction.state === "failed" ||
      transaction.state === "aborted"
    ) {
      current.errors += 1;
    }
    groups.set(key, current);
  }
  return [...groups.values()].sort(
    (left, right) =>
      right.count - left.count || left.label.localeCompare(right.label),
  );
}

export function isLiveSessionActive(
  session: LiveCaptureSession | null,
): boolean {
  return (
    session?.state === "starting" ||
    session?.state === "running" ||
    session?.state === "stopping"
  );
}
