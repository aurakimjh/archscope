import { useSyncExternalStore } from "react";
const MAX_ENTRIES = 80;
const listeners = new Set();
// No independent strong cache: rows live exactly as long as retained entries.
const httpRowFingerprints = new WeakMap();
let entries = [];
let activeID = null;
let snapshot = {
    entries,
    active_id: activeID,
};
function emit() {
    snapshot = {
        entries,
        active_id: activeID,
    };
    for (const listener of listeners)
        listener();
}
function subscribe(listener) {
    listeners.add(listener);
    return () => listeners.delete(listener);
}
function getSnapshot() {
    return snapshot;
}
export function useAnalysisWorkspace() {
    return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}
export function addWorkspaceResult(input) {
    const sourceFiles = Array.isArray(input.result.source_files)
        ? input.result.source_files.filter(Boolean)
        : [];
    const entry = {
        id: `ar-${Date.now()}-${Math.random().toString(16).slice(2)}`,
        title: input.title ?? inferResultTitle(input.result, input.sourceLabel),
        result_type: input.result.type,
        source_files: sourceFiles,
        created_at: input.result.created_at || new Date().toISOString(),
        recorded_at: new Date().toISOString(),
        summary_preview: input.displaySummary?.slice(0, 8).map(item => ({ ...item })) ?? summaryPreview(input.result.summary),
        result: sharedHttpImportRows(input),
        ...(input.backendResultHandle ? { backend_result_handle: input.backendResultHandle } : {}),
    };
    if (input.httpTransactionRowsFingerprint)
        httpRowFingerprints.set(entry, input.httpTransactionRowsFingerprint);
    const previous = entries;
    entries = [entry, ...entries.filter((candidate) => candidate.id !== entry.id)].slice(0, MAX_ENTRIES);
    for (const removed of previous)
        if (!entries.some(e => e.id === removed.id))
            releaseWorkspaceEntry(removed.id);
    ownWorkspaceEntry(entry, input.backendLeaseID);
    activeID = entry.id;
    emit();
    return entry;
}
function sharedHttpImportRows(input) {
    const fingerprint = input.httpTransactionRowsFingerprint;
    const rows = input.result.tables?.transactions;
    if (input.result.type !== "http_capture" || !fingerprint || !/^v1:[a-f0-9]{64}$/.test(fingerprint) || !Array.isArray(rows))
        return input.result;
    let canonical;
    for (const entry of entries) {
        if (entry.result_type !== "http_capture" || httpRowFingerprints.get(entry) !== fingerprint)
            continue;
        const previous = entry.result.tables?.transactions;
        if (Array.isArray(previous) && previous.length === rows.length) {
            canonical = previous;
            break;
        }
    }
    if (!canonical) {
        // Never freeze the caller's current view. History owns an immutable copy.
        canonical = structuredClone(rows);
        const pending = [canonical];
        const seen = new WeakSet();
        while (pending.length) {
            const value = pending.pop();
            if (seen.has(value))
                continue;
            seen.add(value);
            for (const child of Object.values(value))
                if (child !== null && typeof child === "object")
                    pending.push(child);
            Object.freeze(value);
        }
    }
    return { ...input.result, tables: { ...input.result.tables, transactions: canonical } };
}
export function selectWorkspaceResult(id) {
    activeID = id && entries.some((entry) => entry.id === id) ? id : null;
    emit();
}
export function removeWorkspaceResult(id) {
    releaseWorkspaceEntry(id);
    entries = entries.filter((entry) => entry.id !== id);
    if (activeID === id) {
        activeID = entries[0]?.id ?? null;
    }
    emit();
}
export function clearWorkspaceResults() {
    workspaceGeneration++;
    for (const entry of entries)
        releaseWorkspaceEntry(entry.id);
    entries = [];
    activeID = null;
    emit();
}
export function getWorkspaceEntry(id) {
    if (!id)
        return null;
    return entries.find((entry) => entry.id === id) ?? null;
}
export function getActiveWorkspaceEntry() {
    return getWorkspaceEntry(activeID);
}
function inferResultTitle(result, sourceLabel) {
    const source = sourceLabel || basename(result.source_files?.[0] ?? "");
    const type = result.type || "analysis";
    return source ? `${type}: ${source}` : type;
}
function summaryPreview(summary) {
    if (!summary || typeof summary !== "object")
        return [];
    return Object.entries(summary)
        .filter(([, value]) => typeof value === "number" || typeof value === "string" || typeof value === "boolean")
        .slice(0, 8)
        .map(([key, value]) => ({
        key,
        value: formatSummaryValue(value),
    }));
}
function formatSummaryValue(value) {
    if (typeof value === "number") {
        if (!Number.isFinite(value))
            return String(value);
        return Math.abs(value) >= 1000 ? value.toLocaleString() : String(value);
    }
    if (typeof value === "boolean")
        return value ? "true" : "false";
    return String(value);
}
function basename(path) {
    const parts = path.split(/[\\/]/);
    return parts[parts.length - 1] ?? path;
}
let persistence = null;
const owners = new Map();
let workspaceGeneration = 0;
export function getWorkspaceGeneration() { return workspaceGeneration; }
export function configureCasePersistence(api) { persistence = api; }
function ownWorkspaceEntry(entry, ownedLease) {
    const api = persistence;
    if (!api || !entry.backend_result_handle)
        return;
    const generation = workspaceGeneration;
    const acquired = ownedLease ? Promise.resolve({ id: ownedLease }) : api.acquire(entry.backend_result_handle);
    void acquired.then(async (lease) => {
        if (generation !== workspaceGeneration || !entries.some(e => e.id === entry.id)) {
            await api.release(lease.id);
            return;
        }
        owners.set(entry.id, lease.id);
    }).catch((e) => { entry.persistence_error = String(e); emit(); });
}
function releaseWorkspaceEntry(entryID) {
    const lease = owners.get(entryID);
    owners.delete(entryID);
    if (lease && persistence)
        void persistence.release(lease).catch((e) => console.error("Workspace lease release failed", e));
}
export async function saveWorkspaceCase(entryID) {
    if (!persistence)
        throw new Error("Case backend unavailable");
    const entry = getWorkspaceEntry(entryID);
    if (!entry?.backend_result_handle)
        throw new Error("A backend-owned result is required to preserve exact saved values");
    // Do not save result projections or JS-parsed unknown metadata.
    const manifest = entry.saved_case ? await persistence.get(entry.saved_case.case_id) : await persistence.create(entry.title);
    if (entry.saved_case && ["active", "restored"].includes(manifest.state) && manifest.results.some(result => result.id === entry.saved_case.result_id && result.sha256 === entry.saved_case.result_sha256 && result.stale !== true))
        return entry.saved_case;
    const saved = await persistence.save(entry.backend_result_handle, JSON.stringify({
        case_id: manifest.id, expected_revision: manifest.revision, source_ids: [], selection: null,
    }));
    const result = saved.results[saved.results.length - 1];
    if (!result)
        throw new Error("Saved case contains no result reference");
    const reference = { case_id: saved.id, result_id: result.id, result_sha256: result.sha256 };
    entry.saved_case = reference;
    emit();
    return reference;
}
export async function reopenWorkspaceCase(caseID, resultID) {
    if (!persistence)
        throw new Error("Case backend unavailable");
    const generation = workspaceGeneration;
    const manifest = await persistence.get(caseID);
    const result = manifest.results.find(r => r.id === resultID);
    if (!result)
        throw new Error("Saved case result missing");
    const reopened = await persistence.reopen(caseID, resultID);
    if (generation !== workspaceGeneration) {
        await persistence.release(reopened.leaseID);
        throw new Error("Workspace changed while reopening case");
    }
    const entry = addWorkspaceResult({ backendResultHandle: reopened.handle, backendLeaseID: reopened.leaseID,
        result: { type: reopened.type, summary: {} }, title: `${reopened.type} · ${caseID.slice(-8)}` });
    // Only a bounded display summary may be parsed; exact data stays in the backend.
    try {
        entry.summary_preview = summaryPreview(JSON.parse(reopened.summaryJSON));
    }
    catch {
        entry.summary_preview = [];
    }
    entry.saved_case = { case_id: caseID, result_id: resultID, result_sha256: result.sha256 };
    emit();
    return entry;
}
// Explicit diagnostic bundle only. Return counts, never payloads or paths, and
// never keep a second owner/cache or force collection. Ordinary Vite builds
// remove this branch; the layoutaudit observer is excluded from production Go.
if (import.meta.env && import.meta.env.VITE_MEMORY_DIAGNOSTICS === "1" && typeof window !== "undefined") {
    window.__archscopeWorkspaceRetention = () => {
        const http = entries.filter(entry => entry.result_type === "http_capture");
        const arrays = http.map(entry => entry.result.tables?.transactions).filter(Array.isArray);
        const unique = [...new Set(arrays)];
        const memory = performance.memory;
        return { entries: entries.length, httpEntries: http.length,
            fingerprintedHttpEntries: http.filter(entry => httpRowFingerprints.has(entry)).length,
            uniqueTransactionArrays: unique.length, logicalRows: arrays.reduce((sum, array) => sum + array.length, 0),
            uniqueRetainedRows: unique.reduce((sum, array) => sum + array.length, 0),
            immutableArrays: unique.filter(Object.isFrozen).length,
            activeRows: getActiveWorkspaceEntry()?.result.tables?.transactions?.length ?? 0,
            jsHeap: memory ? { used: memory.usedJSHeapSize, total: memory.totalJSHeapSize } : null };
    };
}
