export const LIVE_TRANSACTION_ROW_CAP = 500;

export type LiveCaptureSession = {
  sessionId: string;
  state: string;
  listenAddress: string;
  storePath: string;
  startedAt: string | null;
  endedAt?: string | null;
  error?: string;
  /**
   * Authoritative SEC-17 opt-in for the running session. The renderer checkbox
   * is a request; this field is what the engine actually enforces, so the panel
   * must display this value whenever a session exists (H-RG4 L6).
   */
  retainUnattributedMetadata?: boolean;
};

export type LiveCaptureStats = {
  sessionId: string;
  state: string;
  /**
   * Everything the proxy saw, before any privacy drop. This is the honest
   * denominator for `dropped`/`unattributed` disclosure (H-RG4 L7); `captured`
   * already excludes deliberately dropped records.
   */
  observed: number;
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

/**
 * Progress arrives in engine-batched groups so one page load cannot produce one
 * IPC message and one O(cap) rebuild per request (H-RG4 L4).
 */
export type LiveProgressEvent = {
  sessionId: string;
  items: LiveCaptureTransaction[];
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
      // The panel dispatches "started" twice for one session: once from the
      // StartCapture promise and once from the capture:started event. Resetting
      // on the second dispatch would discard progress rows that landed in
      // between, so only a genuinely new session clears the table. The follow
      // preference is a user setting and survives every start (H-RG4 L12).
      if (matchesSession(state, action.session.sessionId)) {
        return { ...state, session: action.session, busy: false };
      }
      return {
        ...initialLiveCaptureState,
        follow: state.follow,
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
    case "progress": {
      if (!matchesSession(state, action.event.sessionId)) return state;
      const items = action.event.items ?? [];
      if (items.length === 0) return state;
      return {
        ...state,
        transactions: boundedDistinct([...state.transactions, ...items]),
      };
    }
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

/**
 * Deduplicates by transaction id, keeping the newest payload for each id **at
 * the position where that id first appeared**. A finalized row therefore
 * replaces its in-flight row in place instead of jumping to the tail, so
 * completing requests never leapfrog still-pending neighbours (H-RG4 L8).
 * The row cap keeps the newest rows, as before.
 */
function boundedDistinct(
  items: LiveCaptureTransaction[],
): LiveCaptureTransaction[] {
  const positions = new Map<string, number>();
  const ordered: LiveCaptureTransaction[] = [];
  for (const item of items) {
    if (!item) continue;
    const position = positions.get(item.id);
    if (position === undefined) {
      positions.set(item.id, ordered.length);
      ordered.push(item);
      continue;
    }
    ordered[position] = item;
  }
  if (ordered.length <= LIVE_TRANSACTION_ROW_CAP) return ordered;
  return ordered.slice(ordered.length - LIVE_TRANSACTION_ROW_CAP);
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

/**
 * Closed set of fidelity grades the live view is allowed to present. Anything
 * the engine emits that is not in this set resolves to `unknown`, which renders
 * as "not yet determined" — never as a reassuring grade (H-RG4 L1).
 */
export type LiveFidelityToken =
  | "pending"
  | "decoded_wire"
  | "semantic"
  | "unsupported"
  | "passthrough"
  | "unknown";

const LIVE_FIDELITY_TOKENS: Record<string, LiveFidelityToken> = {
  pending: "pending",
  decoded_wire: "decoded_wire",
  semantic: "semantic",
  unsupported: "unsupported",
  passthrough: "passthrough",
  proxy_passthrough: "passthrough",
};

export function resolveLiveFidelity(fidelity: string): LiveFidelityToken {
  return LIVE_FIDELITY_TOKENS[fidelity] ?? "unknown";
}

/**
 * True only for grades that assert ArchScope actually read the exchange. Used
 * to keep in-flight, passthrough, and unrecognized rows out of any presentation
 * that reads as successful semantic capture.
 */
export function isDecodedLiveFidelity(token: LiveFidelityToken): boolean {
  return token === "decoded_wire" || token === "semantic";
}

const LIVE_TERMINAL_TX_STATES = new Set(["complete", "failed", "aborted"]);

/**
 * A row is in flight until the engine gives it a terminal state. In-flight rows
 * have no meaningful duration or status yet, so the panel must not render their
 * `0 ms` / `0` as measured values (H-RG4 L5).
 */
export function isLiveTransactionInFlight(
  transaction: LiveCaptureTransaction,
): boolean {
  return !LIVE_TERMINAL_TX_STATES.has(transaction.state);
}

export function countLiveInFlight(
  transactions: LiveCaptureTransaction[],
): number {
  return transactions.reduce(
    (total, transaction) =>
      total + (isLiveTransactionInFlight(transaction) ? 1 : 0),
    0,
  );
}

export type LiveCoverageDisclosure = {
  observed: number;
  captured: number;
  dropped: number;
  unattributed: number;
  /** Share of observed traffic ArchScope deliberately discarded, 0-100. */
  droppedPercent: number | null;
  hasDrops: boolean;
  hasUnattributed: boolean;
};

/**
 * Derives the honest coverage picture from the stats snapshot. `captured`
 * excludes deliberately dropped records while `unattributed` counts them, so
 * `unattributed > captured` is a normal state; `observed` is the only
 * denominator that makes the tiles mutually consistent (H-RG4 L7).
 */
export function buildLiveCoverageDisclosure(
  stats: LiveCaptureStats | null,
): LiveCoverageDisclosure | null {
  if (!stats) return null;
  const observed = stats.observed ?? 0;
  const dropped = stats.dropped ?? 0;
  return {
    observed,
    captured: stats.captured ?? 0,
    dropped,
    unattributed: stats.unattributed ?? 0,
    droppedPercent: observed > 0 ? (dropped / observed) * 100 : null,
    hasDrops: dropped > 0,
    hasUnattributed: (stats.unattributed ?? 0) > 0,
  };
}

/**
 * The SEC-17 policy actually in force. While a session is active the engine's
 * value is authoritative and the renderer must display it — on page re-entry
 * the local checkbox is back to its default and would otherwise state the
 * opposite of the running privacy policy (H-RG4 L6). The policy is fixed at
 * start, so once the session ends the local choice seeds the next start again.
 */
export function activeUnattributedPolicy(
  session: LiveCaptureSession | null,
  pendingChoice: boolean,
): boolean {
  if (isLiveSessionActive(session)) {
    return session?.retainUnattributedMetadata === true;
  }
  return pendingChoice;
}
