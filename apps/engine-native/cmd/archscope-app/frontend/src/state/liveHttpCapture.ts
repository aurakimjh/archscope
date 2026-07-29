/**
 * Renderer half of the live-capture contract the engine publishes through
 * `CaptureService.GetLiveCaptureContract` (`capture.LiveCaptureContract`).
 * The engine is authoritative: the renderer reads these values at startup and
 * derives its own behaviour from them, so the acceptance fixture's `renderer`
 * block describes what this component actually does rather than a literal that
 * can drift from it (H-RG4 R8).
 */
export type LiveCaptureContract = {
  schemaVersion: number;
  transactionRowCap: number;
  resyncOnEventSkip: boolean;
  restoreCurrentSessionOnPageReentry: boolean;
  finalizedSessionUsesAnalysisResult: boolean;
};

/** The only contract schema this renderer knows how to honour. */
export const LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION = 1;

/**
 * Used until the engine answers, and as the fail-safe when it answers with a
 * schema this build cannot honour. These values must stay equal to
 * `capture.DefaultLiveCaptureContract()`; they are a fallback, not the source
 * of truth.
 */
export const DEFAULT_LIVE_CAPTURE_CONTRACT: LiveCaptureContract = {
  schemaVersion: LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION,
  transactionRowCap: 500,
  resyncOnEventSkip: true,
  restoreCurrentSessionOnPageReentry: true,
  finalizedSessionUsesAnalysisResult: true,
};

/** Fallback row cap. The running cap comes from the engine's contract. */
export const LIVE_TRANSACTION_ROW_CAP =
  DEFAULT_LIVE_CAPTURE_CONTRACT.transactionRowCap;

/**
 * Accepts an engine contract only when it declares the schema this renderer
 * implements and every field is well formed. Anything else falls back to the
 * defaults, because silently honouring half of an unknown contract would make
 * the renderer's behaviour undescribed by either side.
 */
export function resolveLiveCaptureContract(raw: unknown): LiveCaptureContract {
  if (!raw || typeof raw !== "object") return DEFAULT_LIVE_CAPTURE_CONTRACT;
  const candidate = raw as Partial<LiveCaptureContract>;
  if (candidate.schemaVersion !== LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION) {
    return DEFAULT_LIVE_CAPTURE_CONTRACT;
  }
  const rowCap = candidate.transactionRowCap;
  if (
    typeof rowCap !== "number" ||
    !Number.isInteger(rowCap) ||
    rowCap <= 0 ||
    typeof candidate.resyncOnEventSkip !== "boolean" ||
    typeof candidate.restoreCurrentSessionOnPageReentry !== "boolean" ||
    typeof candidate.finalizedSessionUsesAnalysisResult !== "boolean"
  ) {
    return DEFAULT_LIVE_CAPTURE_CONTRACT;
  }
  return {
    schemaVersion: candidate.schemaVersion,
    transactionRowCap: rowCap,
    resyncOnEventSkip: candidate.resyncOnEventSkip,
    restoreCurrentSessionOnPageReentry:
      candidate.restoreCurrentSessionOnPageReentry,
    finalizedSessionUsesAnalysisResult:
      candidate.finalizedSessionUsesAnalysisResult,
  };
}

/**
 * True when the engine offered a contract this renderer had to reject. The
 * panel discloses it, because a rejected contract means the renderer is running
 * on its own defaults rather than on what the engine described.
 */
export function isLiveCaptureContractSupported(raw: unknown): boolean {
  if (!raw || typeof raw !== "object") return false;
  return (
    (raw as Partial<LiveCaptureContract>).schemaVersion ===
    LIVE_CAPTURE_CONTRACT_SCHEMA_VERSION
  );
}

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
  /** Engine-published renderer contract in force (H-RG4 R8). */
  contract: LiveCaptureContract;
  /** True once the engine offered a contract this build cannot honour. */
  contractMismatch: boolean;
};

export const initialLiveCaptureState: LiveCaptureState = {
  session: null,
  stats: null,
  transactions: [],
  follow: true,
  needsResync: false,
  busy: false,
  error: null,
  contract: DEFAULT_LIVE_CAPTURE_CONTRACT,
  contractMismatch: false,
};

export type LiveCaptureAction =
  | { type: "contract"; contract: unknown }
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
    case "contract": {
      const contract = resolveLiveCaptureContract(action.contract);
      return {
        ...state,
        contract,
        contractMismatch: !isLiveCaptureContractSupported(action.contract),
        // A late contract must not leave a longer table than it allows.
        transactions: boundedDistinct(
          state.transactions,
          contract.transactionRowCap,
        ),
      };
    }
    case "hydrate":
      return {
        ...state,
        session: action.session,
        stats: action.stats,
        transactions: boundedDistinct(
          action.transactions,
          state.contract.transactionRowCap,
        ),
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
        // The contract describes the renderer, not the session, so a new
        // session must not silently revert it to the built-in defaults.
        contract: state.contract,
        contractMismatch: state.contractMismatch,
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
      // Resync-on-skip is a contract term, so a contract that turns it off
      // turns off the renderer's recovery path with it (H-RG4 R8).
      const skippedAdvanced =
        state.contract.resyncOnEventSkip &&
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
        transactions: boundedDistinct(
          [...state.transactions, ...items],
          state.contract.transactionRowCap,
        ),
      };
    }
    case "transactions":
      if (!matchesSession(state, action.event.sessionId)) return state;
      return {
        ...state,
        transactions: boundedDistinct(
          [...state.transactions, ...action.event.items],
          state.contract.transactionRowCap,
        ),
      };
    case "resynced":
      if (!matchesSession(state, action.stats.sessionId)) return state;
      return {
        ...state,
        stats: action.stats,
        transactions: boundedDistinct(
          action.transactions,
          state.contract.transactionRowCap,
        ),
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
 * The row cap comes from the engine's renderer contract and keeps the newest
 * rows, as before.
 */
function boundedDistinct(
  items: LiveCaptureTransaction[],
  rowCap: number,
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
  if (ordered.length <= rowCap) return ordered;
  return ordered.slice(ordered.length - rowCap);
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

/**
 * How the live table must present a grade. `limited` covers every grade that
 * says ArchScope did **not** read the exchange, so those rows carry a caution
 * emphasis instead of reading like ordinary captured traffic; `pending` is the
 * neutral "not yet determined" case. This is where `isDecodedLiveFidelity`
 * gates the positive presentation in the product (H-RG4 R9).
 */
export type LiveFidelityTone = "decoded" | "pending" | "limited";

export function liveFidelityTone(token: LiveFidelityToken): LiveFidelityTone {
  if (isDecodedLiveFidelity(token)) return "decoded";
  return token === "pending" || token === "unknown" ? "pending" : "limited";
}

/**
 * Closed label token sets for the raw engine enums the panel prints. Every one
 * of these was rendered as its wire token before, which broke the project's
 * English/Korean parity guardrail exactly on the columns added to explain
 * unresolved rows to the user (H-RG4 R11). Unrecognized values resolve to
 * `unknown` for the same reason the fidelity map does.
 */
export type LiveTransactionStateToken =
  | "request_sent"
  | "receiving"
  | "complete"
  | "failed"
  | "aborted"
  | "unknown";

const LIVE_TRANSACTION_STATE_TOKENS = new Set<LiveTransactionStateToken>([
  "request_sent",
  "receiving",
  "complete",
  "failed",
  "aborted",
]);

export function resolveLiveTransactionState(
  state: string,
): LiveTransactionStateToken {
  return LIVE_TRANSACTION_STATE_TOKENS.has(state as LiveTransactionStateToken)
    ? (state as LiveTransactionStateToken)
    : "unknown";
}

export type LiveSessionStateToken =
  | "created"
  | "starting"
  | "running"
  | "stopping"
  | "finalized"
  | "failed"
  | "recoverable"
  | "unknown";

const LIVE_SESSION_STATE_TOKENS = new Set<LiveSessionStateToken>([
  "created",
  "starting",
  "running",
  "stopping",
  "finalized",
  "failed",
  "recoverable",
]);

export function resolveLiveSessionState(state: string): LiveSessionStateToken {
  return LIVE_SESSION_STATE_TOKENS.has(state as LiveSessionStateToken)
    ? (state as LiveSessionStateToken)
    : "unknown";
}

export type LiveCAStateToken =
  | "loading"
  | "absent"
  | "installing"
  | "trusted"
  | "partial"
  | "failed"
  | "expired"
  | "unknown";

const LIVE_CA_STATE_TOKENS = new Set<LiveCAStateToken>([
  "loading",
  "absent",
  "installing",
  "trusted",
  "partial",
  "failed",
  "expired",
]);

export function resolveLiveCAState(state: string): LiveCAStateToken {
  return LIVE_CA_STATE_TOKENS.has(state as LiveCAStateToken)
    ? (state as LiveCAStateToken)
    : "unknown";
}

export type LiveAttributionToken = "confirmed" | "inferred" | "unknown";

export function resolveLiveAttribution(
  attribution: string,
): LiveAttributionToken {
  return attribution === "confirmed" || attribution === "inferred"
    ? attribution
    : "unknown";
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
