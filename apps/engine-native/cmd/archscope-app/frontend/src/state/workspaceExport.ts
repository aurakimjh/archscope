import type { AnalysisWorkspaceEntry } from "./analysisWorkspace.js";

export type WorkspaceExportMode = "json" | "html" | "pptx" | "csv" | "csv_dir";
type ExportRequest = { path: string; result: unknown };
type ExportPorts = {
  backend: (handle: string, path: string, format: string) => PromiseLike<void>;
  legacy: Record<WorkspaceExportMode, (request: ExportRequest) => PromiseLike<void>>;
};

export function backendWorkspaceExportSupported(mode: WorkspaceExportMode): boolean {
  return mode === "json" || mode === "html" || mode === "csv";
}

// Handle-backed entries contain only display projections in JavaScript. Never
// serialize those placeholders as a successful report or saved result.
export async function exportWorkspaceResult(
  entry: Pick<AnalysisWorkspaceEntry, "backend_result_handle" | "result">,
  mode: WorkspaceExportMode,
  path: string,
  ports: ExportPorts,
): Promise<void> {
  if (entry.backend_result_handle) {
    if (!backendWorkspaceExportSupported(mode)) throw new Error("RESULT_EXPORT_FORMAT_UNSUPPORTED");
    // Original JSON remains available even for unknown/future read-only types.
    await ports.backend(entry.backend_result_handle, path, mode === "json" ? "raw-json" : mode);
    return;
  }
  await ports.legacy[mode]({ path, result: entry.result });
}
