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
    summary_preview: summaryPreview(input.result.summary),
    result: input.result,
  };
  entries = [entry, ...entries.filter((candidate) => candidate.id !== entry.id)].slice(0, MAX_ENTRIES);
  activeID = entry.id;
  emit();
  return entry;
}

export function selectWorkspaceResult(id: string | null): void {
  activeID = id && entries.some((entry) => entry.id === id) ? id : null;
  emit();
}

export function removeWorkspaceResult(id: string): void {
  entries = entries.filter((entry) => entry.id !== id);
  if (activeID === id) {
    activeID = entries[0]?.id ?? null;
  }
  emit();
}

export function clearWorkspaceResults(): void {
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
