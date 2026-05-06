// Manual Wails-3 binding for `EngineService` (Go side defined in
// apps/profiler-native/cmd/archscope-profiler-app/engineservice.go).
//
// Wails 3's runtime exposes calls through `Call.ByName("package.Struct.Method", args)`.
// The CLI normally generates a typed module under
// `frontend/bindings/.../engineservice.ts`, but the Wails CLI is not
// installed in every developer environment (`task generate:bindings`
// runs the generator). This file ships the same wire shape as the
// generator would so the renderer can call EngineService today; once the
// generator runs locally, drop this file and import the generated module
// instead.
//
// All 14 EngineService methods are wrapped with their Go-side request
// shape mirrored under `bridge/types.ts`. Async analyzers
// (`analyzeMultiThread`, `analyzeLockContention`) return `{ taskId }` and
// the caller subscribes to the `engine:done` / `engine:error` /
// `engine:cancelled` events emitted by the Go side.

import { Call, type CancellablePromise } from "@wailsio/runtime";

import type {
  AccessLogAnalysisResult,
  AccessLogRequest,
  AnalysisResult,
  ClassifyRequest,
  ClassifyResult,
  CollapsedRequest,
  CollapsedResult,
  EngineAsyncResponse,
  EngineDiffRequest,
  ExceptionRequest,
  ExceptionStackAnalysisResult,
  ExportCSVRequest,
  ExportHTMLRequest,
  ExportJSONRequest,
  ExportPPTXRequest,
  GcLogRequest,
  JfrRequest,
  LockContentionRequest,
  MultiThreadRequest,
  OtelRequest,
  RuntimeRequest,
  ThreadDumpRequest,
} from "./types";

// `main.EngineService` is the qualified name the Go binding generator
// emits — `main` is the package, `EngineService` is the receiver type.
const NS = "main.EngineService.";

function call<T>(method: string, ...args: unknown[]): CancellablePromise<T> {
  return Call.ByName(NS + method, ...args) as CancellablePromise<T>;
}

// ──────────────────────────────────────────────────────────────────
// Sync analyzers — single-file inputs.
// ──────────────────────────────────────────────────────────────────

export function analyzeAccessLog(
  req: AccessLogRequest,
): CancellablePromise<AccessLogAnalysisResult> {
  return call<AccessLogAnalysisResult>("AnalyzeAccessLog", req);
}

export function analyzeGcLog(
  req: GcLogRequest,
): CancellablePromise<AnalysisResult> {
  return call<AnalysisResult>("AnalyzeGcLog", req);
}

export function analyzeJfr(
  req: JfrRequest,
): CancellablePromise<AnalysisResult> {
  return call<AnalysisResult>("AnalyzeJfr", req);
}

export function analyzeNativeMemory(
  req: JfrRequest,
): CancellablePromise<AnalysisResult> {
  return call<AnalysisResult>("AnalyzeNativeMemory", req);
}

export function analyzeException(
  req: ExceptionRequest,
): CancellablePromise<ExceptionStackAnalysisResult> {
  return call<ExceptionStackAnalysisResult>("AnalyzeException", req);
}

export function analyzeRuntimeStack(
  req: RuntimeRequest,
): CancellablePromise<AnalysisResult> {
  return call<AnalysisResult>("AnalyzeRuntimeStack", req);
}

export function analyzeOtel(
  req: OtelRequest,
): CancellablePromise<AnalysisResult> {
  return call<AnalysisResult>("AnalyzeOtel", req);
}

export function analyzeThreadDump(
  req: ThreadDumpRequest,
): CancellablePromise<AnalysisResult> {
  return call<AnalysisResult>("AnalyzeThreadDump", req);
}

export function classifyStack(
  req: ClassifyRequest,
): CancellablePromise<ClassifyResult> {
  return call<ClassifyResult>("ClassifyStack", req);
}

// ──────────────────────────────────────────────────────────────────
// Async analyzers — return a TaskID; the renderer subscribes to
// `engine:done` / `engine:error` / `engine:cancelled` events.
// ──────────────────────────────────────────────────────────────────

export function analyzeMultiThread(
  req: MultiThreadRequest,
): CancellablePromise<EngineAsyncResponse> {
  return call<EngineAsyncResponse>("AnalyzeMultiThread", req);
}

export function analyzeLockContention(
  req: LockContentionRequest,
): CancellablePromise<EngineAsyncResponse> {
  return call<EngineAsyncResponse>("AnalyzeLockContention", req);
}

export function cancelEngineTask(taskID: string): CancellablePromise<boolean> {
  return call<boolean>("CancelEngineTask", taskID);
}

// ──────────────────────────────────────────────────────────────────
// Conversions
// ──────────────────────────────────────────────────────────────────

export function convertToCollapsed(
  req: CollapsedRequest,
): CancellablePromise<CollapsedResult> {
  return call<CollapsedResult>("ConvertToCollapsed", req);
}

// ──────────────────────────────────────────────────────────────────
// Exporters
// ──────────────────────────────────────────────────────────────────

export function exportJSON(req: ExportJSONRequest): CancellablePromise<void> {
  return call<void>("ExportJSON", req);
}

export function exportHTML(req: ExportHTMLRequest): CancellablePromise<void> {
  return call<void>("ExportHTML", req);
}

export function exportPPTX(req: ExportPPTXRequest): CancellablePromise<void> {
  return call<void>("ExportPPTX", req);
}

export function exportCSV(req: ExportCSVRequest): CancellablePromise<void> {
  return call<void>("ExportCSV", req);
}

export function exportCSVDir(req: ExportCSVRequest): CancellablePromise<void> {
  return call<void>("ExportCSVDir", req);
}

export function diffReports(
  req: EngineDiffRequest,
): CancellablePromise<Record<string, unknown>> {
  return call<Record<string, unknown>>("DiffReports", req);
}

// Convenience namespace export so callers can write
// `engine.analyzeAccessLog(...)` instead of importing 14 names.
export const engine = {
  analyzeAccessLog,
  analyzeGcLog,
  analyzeJfr,
  analyzeNativeMemory,
  analyzeException,
  analyzeRuntimeStack,
  analyzeOtel,
  analyzeThreadDump,
  analyzeMultiThread,
  analyzeLockContention,
  classifyStack,
  cancelEngineTask,
  convertToCollapsed,
  exportJSON,
  exportHTML,
  exportPPTX,
  exportCSV,
  exportCSVDir,
  diffReports,
};
