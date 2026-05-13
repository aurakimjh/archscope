// ─────────────────────────────────────────────────────────────────────
// [한글] bridge/engine.ts — Wails v3 EngineService 바인딩 래퍼.
//
// 책임/목적:
//   - Go 측 `main.EngineService` 메서드를 TypeScript 함수로
//     1:1 매핑. Wails CLI 의 `task generate:bindings` 가 자동 생성하는
//     모듈 대용으로, 모든 호출은 `Call.ByName("main.EngineService.XXX",
//     ...)` 와이어 형식을 직접 사용 — 빌드 환경에 Wails CLI 가 없어도
//     렌더러가 즉시 EngineService 를 호출 가능합니다.
//   - 동기 분석기(AccessLog/Gc/Jfr/Exception/Runtime/Otel/Jennifer/
//     ThreadDump 등) 는 결과(AnalysisResult) 를 직접 반환.
//   - 비동기 분석기(MultiThread, LockContention) 는 `{ taskId }` 만
//     반환하고, 호출자는 Go side 가 emit 하는
//     `engine:done` / `engine:error` / `engine:cancelled` Wails 이벤트를
//     구독해 결과를 수신합니다 (long-running, cancelable).
//
// 데이터 흐름 (Go EngineService ↔ React):
//   React 컴포넌트 → engine.* 함수 호출 → Call.ByName 으로 Wails IPC
//   → Go EngineService 메서드 실행 → JSON marshalled AnalysisResult
//   반환 → CancellablePromise resolve. 중간에 사용자가 cancel 하면
//   CancellablePromise.cancel() 호출로 Go side 의 context 가 취소됩니다.
//
// 의존성 주의: `@wailsio/runtime` 의 Call.ByName 은 string 기반이라
// typo 가 컴파일 타임에 잡히지 않습니다 — NS 상수 + method 문자열
// 일치를 PR 리뷰 시 반드시 확인하세요.
// ─────────────────────────────────────────────────────────────────────
// Manual Wails-3 binding for `EngineService` (Go side defined in
// apps/engine-native/cmd/archscope-app/engineservice.go).
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
// EngineService methods are wrapped with their Go-side request
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
  JfrAnalysisResult,
  JfrRequest,
  LockContentionRequest,
  NativeMemoryAnalysisResult,
  MultiThreadRequest,
  OtelRequest,
  RuntimeRequest,
  ThreadDumpRequest,
  TraceImportAnalysisResult,
  TraceImportRequest,
} from "./types";

// `main.EngineService` is the qualified name the Go binding generator
// emits — `main` is the package, `EngineService` is the receiver type.
//
// [한글] NS: Wails 가 Go binding 을 식별할 때 사용하는 정식 이름.
// "main.EngineService.<MethodName>" 형식으로 IPC 디스패치됩니다.
const NS = "main.EngineService.";

// [한글] call: NS + method 문자열로 Wails Call.ByName 을 감싸는
// 작은 헬퍼. 반환 타입을 제네릭 T 로 고정해 호출부에서 결과 타입을
// 선언적으로 명시할 수 있게 합니다(런타임 검증은 없음).
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
): CancellablePromise<JfrAnalysisResult> {
  return call<JfrAnalysisResult>("AnalyzeJfr", req);
}

export function analyzeNativeMemory(
  req: JfrRequest,
): CancellablePromise<NativeMemoryAnalysisResult> {
  return call<NativeMemoryAnalysisResult>("AnalyzeNativeMemory", req);
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

export function analyzeTraceImport(
  req: TraceImportRequest,
): CancellablePromise<TraceImportAnalysisResult> {
  return call<TraceImportAnalysisResult>("AnalyzeTraceImport", req);
}

// JenniferProfileRequest matches the Go-side struct; either Path
// (single) or Paths (batch) is required. FallbackCorrelationToTxid
// mirrors the CLI flag of the same name.
export type JenniferProfileRequest = {
  path?: string;
  paths?: string[];
  fallbackCorrelationToTxid?: boolean;
  headerBodyToleranceMs?: number;
  networkPrepPatterns?: string[];
  eventCategoryPatterns?: Record<string, string[]>;
  customAnalysisRules?: Array<{
    id?: string;
    label: string;
    group?: string;
    source: string;
    patterns: string[];
  }>;
};

export function analyzeJenniferProfile(
  req: JenniferProfileRequest,
): CancellablePromise<AnalysisResult> {
  return call<AnalysisResult>("AnalyzeJenniferProfile", req);
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
// [한글] 비동기 분석기 그룹: 멀티 스레드 덤프 / Lock contention 처럼
// N 개 파일을 처리해 수 초~수 분 소요되는 작업은 task 등록 후
// taskId 만 반환. 이후 Wails event channel 로 결과/에러/취소가
// emit 되며, 호출자는 cancelEngineTask(taskId) 로 도중에 중단 가능합니다.

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
  analyzeTraceImport,
  analyzeJenniferProfile,
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
