// Only bounded backend display DTOs enter this module. Stored result JSON stays
// opaque; exact numbers, deltas, units and provenance travel as text.
export type DisplayCell = { text: string; null: boolean; truncated: boolean; availability?: string; reason?: string; unit?: string; population?: string };
export type ResultCapability = { id: string; version: string; kind: string; payload_path: string; schema: string; availability: string; reason?: string };
export type ResultCapabilities = { handle: string; result_sha256: string; type: string; version: string; compatibility: { allowed: boolean; reason?: string }; identity_basis: string; providers: ResultCapability[]; sources: DisplayCell[]; sources_truncated: boolean; truncated: boolean };
export type ProviderPage = { handle: string; result_sha256: string; provider: string; columns: string[]; rows: DisplayCell[][]; offset: number; next: number; total: number; truncated: boolean };
export type ProviderComparison = { allowed: boolean; code?: string; reason?: string; delta?: { value: string | null; percent: string | null; reason?: string; comparison?: { identity_basis: string; baseline: { handle: string; result_sha256: string }; target: { handle: string; result_sha256: string } } } };
export type ImportJob = { packs?: Array<{ id: string; version: string; sha256: string; origin: string; scope: string }>; pack_state_revision?: string; id: string; generation: number; state: string; resultHandle?: string; cleanupPending?: boolean; problem?: { code: string; message: string }; detection: { candidates: Array<{ importerId: string; reason: string }> } };
export type Cancelable<T> = Promise<T> & { cancel?: () => void };

// One scope per mounted load/query surface. Invalidating before cancellation
// prevents late completions and rejections from overwriting a newer selection.
export class ResultRequestScope {
  private revision = 0;
  private pending = new Set<Cancelable<unknown>>();
  begin(): number { this.cancel(); return this.revision; }
  current(revision: number): boolean { return this.revision === revision; }
  async wait<T>(revision: number, promise: Cancelable<T>): Promise<T> {
    if (!this.current(revision)) { promise.cancel?.(); void promise.catch(() => {}); throw new Error("request_superseded"); }
    this.pending.add(promise);
    try { const value = await promise; if (!this.current(revision)) throw new Error("request_superseded"); return value; }
    finally { this.pending.delete(promise); }
  }
  cancel(): void { this.revision++; for (const request of this.pending) request.cancel?.(); this.pending.clear(); }
}
export function providerUsable(capability: ResultCapability): boolean {
  return capability.availability === "available" && capability.version === "1.0.0" &&
    ((["table", "detail"].includes(capability.kind) && capability.schema === "object-rows-v1") ||
      (["summary", "diagnostics"].includes(capability.kind) && capability.schema === "key-value-v1") ||
      (capability.kind === "timeline" && capability.schema === "timeline-v1") ||
      (capability.kind === "sli" && capability.schema === "sli-v1") ||
      (capability.kind === "compare" && capability.schema === "measurement-delta-v1"));
}
export function validateProviderPage(page: ProviderPage, caps: ResultCapabilities, id: string): ProviderPage {
  if (page.handle !== caps.handle || page.result_sha256 !== caps.result_sha256 || page.provider !== id || page.columns.length > 32 || page.rows.length > 100 || page.rows.some(row => row.length !== page.columns.length)) throw new Error("provider_revision_mismatch");
  return page;
}
export function resultProblem(error: unknown): { code: string; message: string; stale: boolean } {
  const text = String(error);
  const code = error && typeof error === "object" && "cause" in error && error.cause && typeof error.cause === "object" && "code" in error.cause ? String(error.cause.code) : (text.match(/(?:IMPORT|RESULT)_[A-Z_]+/)?.[0] ?? "request_failed");
  return { code, message: text, stale: code === "IMPORT_RESULT_STALE" || code === "IMPORT_RESULT_CORRUPT" };
}

// Job creation is not a cancelable read: retain a late ID long enough to cancel
// the backend worker, even if its requesting UI has already disappeared.
export async function startScopedImport(scope: ResultRequestScope, revision: number, start: Promise<ImportJob>, cancel: (id: string, generation: number) => Promise<unknown>): Promise<ImportJob | null> {
  const job = await start;
  if (!scope.current(revision)) { await cancel(job.id, job.generation); return null; }
  return job;
}
