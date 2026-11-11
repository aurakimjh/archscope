import { useSyncExternalStore } from "react";

const MAX_ENTRIES = 80;

export type WorkspaceAnalysisResult = {
  type: string;
  source_files?: string[];
  created_at?: string;
  summary?: Record<string, unknown>;
  series?: Record<string, unknown>;
  tables?: Record<string, unknown>;
  charts?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
};

export type AnalysisWorkspaceEntry = {
  backend_result_handle?: string;
  saved_case?: SavedCaseReference;
  persistence_error?: string;
  id: string;
  title: string;
  result_type: string;
  source_files: string[];
  created_at: string;
  recorded_at: string;
  summary_preview: Array<{ key: string; value: string }>;
  result: WorkspaceAnalysisResult;
};

export type AnalysisWorkspaceSnapshot = {
  entries: AnalysisWorkspaceEntry[];
  active_id: string | null;
};

type AddWorkspaceResultInput = {
  displaySummary?: Array<{ key: string; value: string }>;
  backendResultHandle?: string;
  backendLeaseID?: string;
  result: WorkspaceAnalysisResult;
  title?: string;
  sourceLabel?: string;
};

const listeners = new Set<() => void>();
let entries: AnalysisWorkspaceEntry[] = [];
let activeID: string | null = null;
let snapshot: AnalysisWorkspaceSnapshot = {
  entries,
  active_id: activeID,
};

function emit(): void {
  snapshot = {
    entries,
    active_id: activeID,
  };
  for (const listener of listeners) listener();
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function getSnapshot(): AnalysisWorkspaceSnapshot {
  return snapshot;
}

export function useAnalysisWorkspace(): AnalysisWorkspaceSnapshot {
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

export function addWorkspaceResult(input: AddWorkspaceResultInput): AnalysisWorkspaceEntry {
  const sourceFiles = Array.isArray(input.result.source_files)
    ? input.result.source_files.filter(Boolean)
    : [];
  const entry: AnalysisWorkspaceEntry = {
    id: `ar-${Date.now()}-${Math.random().toString(16).slice(2)}`,
    title: input.title ?? inferResultTitle(input.result, input.sourceLabel),
    result_type: input.result.type,
    source_files: sourceFiles,
    created_at: input.result.created_at || new Date().toISOString(),
    recorded_at: new Date().toISOString(),
    summary_preview: input.displaySummary?.slice(0, 8).map(item => ({ ...item })) ?? summaryPreview(input.result.summary),
    result: input.result,
    ...(input.backendResultHandle ? { backend_result_handle: input.backendResultHandle } : {}),
  };
  const previous = entries;
  entries = [entry, ...entries.filter((candidate) => candidate.id !== entry.id)].slice(0, MAX_ENTRIES);
  for (const removed of previous) if (!entries.some(e => e.id === removed.id)) releaseWorkspaceEntry(removed.id);
  ownWorkspaceEntry(entry, input.backendLeaseID);
  activeID = entry.id;
  emit();
  return entry;
}

export function selectWorkspaceResult(id: string | null): void {
  activeID = id && entries.some((entry) => entry.id === id) ? id : null;
  emit();
}

export function removeWorkspaceResult(id: string): void {
  releaseWorkspaceEntry(id);
  entries = entries.filter((entry) => entry.id !== id);
  if (activeID === id) {
    activeID = entries[0]?.id ?? null;
  }
  emit();
}

export function clearWorkspaceResults(): void {
  workspaceGeneration++;
  for (const entry of entries) releaseWorkspaceEntry(entry.id);
  entries = [];
  activeID = null;
  emit();
}

export function getWorkspaceEntry(id: string | null | undefined): AnalysisWorkspaceEntry | null {
  if (!id) return null;
  return entries.find((entry) => entry.id === id) ?? null;
}

export function getActiveWorkspaceEntry(): AnalysisWorkspaceEntry | null {
  return getWorkspaceEntry(activeID);
}

function inferResultTitle(result: WorkspaceAnalysisResult, sourceLabel?: string): string {
  const source = sourceLabel || basename(result.source_files?.[0] ?? "");
  const type = result.type || "analysis";
  return source ? `${type}: ${source}` : type;
}

function summaryPreview(summary: object | undefined): Array<{ key: string; value: string }> {
  if (!summary || typeof summary !== "object") return [];
  return Object.entries(summary)
    .filter(([, value]) => typeof value === "number" || typeof value === "string" || typeof value === "boolean")
    .slice(0, 8)
    .map(([key, value]) => ({
      key,
      value: formatSummaryValue(value),
    }));
}

function formatSummaryValue(value: unknown): string {
  if (typeof value === "number") {
    if (!Number.isFinite(value)) return String(value);
    return Math.abs(value) >= 1000 ? value.toLocaleString() : String(value);
  }
  if (typeof value === "boolean") return value ? "true" : "false";
  return String(value);
}

function basename(path: string): string {
  const parts = path.split(/[\\/]/);
  return parts[parts.length - 1] ?? path;
}

// Durable case references contain no renderer result, ephemeral backend handle,
// source pathname, title or payload. The backend snapshot remains authoritative.
export type SavedCaseReference = { case_id: string; result_id: string; result_sha256: string };
export type CaseManifest = {
  id: string; revision: string; state: string;
  results: Array<{ id: string; sha256: string; type: string; stale?: boolean }>;
};
type CasePersistence = {
  create: (title: string) => Promise<CaseManifest>;
  get: (id: string) => Promise<CaseManifest>;
  save: (handle: string, request: string) => Promise<CaseManifest>;
  reopen: (caseID: string, resultID: string) => Promise<{ handle: string; leaseID: string; type: string; summaryJSON: string }>;
  acquire: (handle: string) => Promise<{ id: string }>;
  release: (leaseID: string) => Promise<unknown>;
};
let persistence: CasePersistence | null = null;
const owners = new Map<string, string>();
let workspaceGeneration = 0;
export function getWorkspaceGeneration(): number { return workspaceGeneration; }
export function configureCasePersistence(api: CasePersistence): void { persistence = api; }
function ownWorkspaceEntry(entry: AnalysisWorkspaceEntry, ownedLease?: string): void {
  const api = persistence;
  if (!api || !entry.backend_result_handle) return;
  const generation = workspaceGeneration;
  const acquired = ownedLease ? Promise.resolve({ id: ownedLease }) : api.acquire(entry.backend_result_handle);
  void acquired.then(async (lease) => {
    if (generation !== workspaceGeneration || !entries.some(e => e.id === entry.id)) { await api.release(lease.id); return; }
    owners.set(entry.id, lease.id);
  }).catch((e: unknown) => { entry.persistence_error = String(e); emit(); });
}
function releaseWorkspaceEntry(entryID: string): void {
  const lease = owners.get(entryID); owners.delete(entryID);
  if (lease && persistence) void persistence.release(lease).catch((e: unknown) => console.error("Workspace lease release failed", e));
}
export async function saveWorkspaceCase(entryID: string): Promise<SavedCaseReference> {
  if (!persistence) throw new Error("Case backend unavailable");
  const entry = getWorkspaceEntry(entryID);
  if (!entry?.backend_result_handle) throw new Error("A backend-owned result is required to preserve exact saved values");
  // Do not save result projections or JS-parsed unknown metadata.
  const manifest = entry.saved_case ? await persistence.get(entry.saved_case.case_id) : await persistence.create(entry.title);
  if (entry.saved_case && ["active", "restored"].includes(manifest.state) && manifest.results.some(result => result.id === entry.saved_case!.result_id && result.sha256 === entry.saved_case!.result_sha256 && result.stale !== true)) return entry.saved_case;
  const saved = await persistence.save(entry.backend_result_handle, JSON.stringify({
    case_id: manifest.id, expected_revision: manifest.revision, source_ids: [], selection: null,
  }));
  const result = saved.results[saved.results.length - 1];
  if (!result) throw new Error("Saved case contains no result reference");
  const reference = { case_id: saved.id, result_id: result.id, result_sha256: result.sha256 };
  entry.saved_case = reference; emit();
  return reference;
}
export async function reopenWorkspaceCase(caseID: string, resultID: string): Promise<AnalysisWorkspaceEntry> {
  if (!persistence) throw new Error("Case backend unavailable");
  const generation = workspaceGeneration;
  const manifest = await persistence.get(caseID);
  const result = manifest.results.find(r => r.id === resultID);
  if (!result) throw new Error("Saved case result missing");
  const reopened = await persistence.reopen(caseID, resultID);
  if (generation !== workspaceGeneration) { await persistence.release(reopened.leaseID); throw new Error("Workspace changed while reopening case"); }
  const entry = addWorkspaceResult({ backendResultHandle: reopened.handle, backendLeaseID: reopened.leaseID,
    result: { type: reopened.type, summary: {} }, title: `${reopened.type} · ${caseID.slice(-8)}` });
  // Only a bounded display summary may be parsed; exact data stays in the backend.
  try { entry.summary_preview = summaryPreview(JSON.parse(reopened.summaryJSON)); } catch { entry.summary_preview = []; }
  entry.saved_case = { case_id: caseID, result_id: resultID, result_sha256: result.sha256 }; emit();
  return entry;
}
