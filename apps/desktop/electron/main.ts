import { app, BrowserWindow, dialog, ipcMain, session } from "electron";
import { execFile, type ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import { mkdir, readFile, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type {
  AccessLogAnalysisResult,
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
  AnalyzerExecuteRequest,
  AnalyzerExecutionResult,
  AnalyzerResponse,
  AnalysisResult,
  ProfilerCollapsedAnalysisResult,
  SelectFileRequest,
  SelectFileResponse,
} from "../src/api/analyzerContract.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "../../..");
const activeEngineProcesses = new Set<ChildProcess>();

type EngineInvocation = {
  file: string;
  argsPrefix: string[];
  cwd: string;
};

type EngineProcessResponse =
  | { ok: true; messages: string[] }
  | { ok: false; error: { code: string; message: string; detail?: string } };

type AnalysisResultValidator<T extends AnalysisResult> = (value: unknown) => value is T;
type SupportedResultType = "access_log" | "profiler_collapsed" | "jfr_recording";

const PACKAGED_CSP = [
  "default-src 'self'",
  "script-src 'self'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data:",
  "font-src 'self'",
  "connect-src 'self'",
  "object-src 'none'",
  "base-uri 'none'",
].join("; ");

const DEVELOPMENT_CSP = [
  "default-src 'self'",
  "script-src 'self' 'unsafe-eval'",
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data:",
  "font-src 'self'",
  "connect-src 'self' http://127.0.0.1:5173 ws://127.0.0.1:5173",
  "object-src 'none'",
  "base-uri 'none'",
].join("; ");

const SUPPORTED_SCHEMA_VERSIONS: Record<SupportedResultType, string> = {
  access_log: "0.1.0",
  profiler_collapsed: "0.1.0",
  jfr_recording: "0.1.0",
};

function createWindow(): void {
  const mainWindow = new BrowserWindow({
    width: 1440,
    height: 960,
    minWidth: 1120,
    minHeight: 760,
    title: "ArchScope",
    backgroundColor: "#f5f7fb",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
    },
  });

  if (app.isPackaged) {
    mainWindow.loadFile(path.join(__dirname, "../dist/index.html"));
  } else {
    mainWindow.loadURL("http://127.0.0.1:5173");
  }
}

app.whenReady().then(() => {
  registerContentSecurityPolicy();
  registerIpcHandlers();
  createWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createWindow();
    }
  });
});

app.on("window-all-closed", () => {
  if (process.platform !== "darwin") {
    app.quit();
  }
});

app.on("before-quit", terminateActiveEngineProcesses);
process.on("exit", terminateActiveEngineProcesses);

function registerIpcHandlers(): void {
  ipcMain.handle(
    "dialog:select-file",
    async (_event, request?: SelectFileRequest): Promise<SelectFileResponse> => {
      const result = await dialog.showOpenDialog({
        title: request?.title,
        properties: ["openFile"],
        filters: request?.filters,
      });

      if (result.canceled || result.filePaths.length === 0) {
        return { canceled: true };
      }

      return {
        canceled: false,
        filePath: result.filePaths[0],
      };
    },
  );

  ipcMain.handle(
    "analyzer:execute",
    async (
      _event,
      request: AnalyzerExecuteRequest,
    ): Promise<AnalyzerResponse<AnalyzerExecutionResult>> => {
      try {
        return await executeAnalyzer(request);
      } catch (error) {
        return ipcFailed(error);
      }
    },
  );
}

function registerContentSecurityPolicy(): void {
  const policy = app.isPackaged ? PACKAGED_CSP : DEVELOPMENT_CSP;
  session.defaultSession.webRequest.onHeadersReceived((details, callback) => {
    callback({
      responseHeaders: {
        ...details.responseHeaders,
        "Content-Security-Policy": [policy],
      },
    });
  });
}

async function executeAnalyzer(
  request: AnalyzerExecuteRequest,
): Promise<AnalyzerResponse<AnalyzerExecutionResult>> {
  if (!request || typeof request !== "object") {
    return failure("INVALID_OPTION", "Analyzer execution request is required.");
  }

  switch (request.type) {
    case "access_log":
      return analyzeAccessLog(request.params);
    case "profiler_collapsed":
      return analyzeCollapsedProfile(request.params);
    default:
      return failure("INVALID_OPTION", "Unsupported analyzer execution type.");
  }
}

async function analyzeAccessLog(
  request: AnalyzeAccessLogRequest,
): Promise<AnalyzerResponse<AccessLogAnalysisResult>> {
  if (!request || typeof request !== "object") {
    return failure("INVALID_OPTION", "Access log analyzer request is required.");
  }

  if (
    typeof request.filePath !== "string" ||
    !request.filePath ||
    typeof request.format !== "string" ||
    !request.format
  ) {
    return failure("INVALID_OPTION", "Access log file and format are required.");
  }

  if (
    request.maxLines !== undefined &&
    (!Number.isInteger(request.maxLines) || request.maxLines <= 0)
  ) {
    return failure("INVALID_OPTION", "Max lines must be a positive integer.");
  }

  const readable = await isReadableFile(request.filePath);
  if (!readable) {
    return failure("FILE_NOT_FOUND", "Selected access log file is not readable.", request.filePath);
  }

  return runAnalyzer<AccessLogAnalysisResult>(
    [
      "access-log",
      "analyze",
      "--file",
      request.filePath,
      "--format",
      request.format,
    ],
    isAccessLogAnalysisResult,
    "Python engine did not produce valid access log AnalysisResult JSON.",
    request.maxLines !== undefined ? ["--max-lines", String(request.maxLines)] : [],
    request.startTime ? ["--start-time", request.startTime] : [],
    request.endTime ? ["--end-time", request.endTime] : [],
  );
}

async function analyzeCollapsedProfile(
  request: AnalyzeCollapsedProfileRequest,
): Promise<AnalyzerResponse<ProfilerCollapsedAnalysisResult>> {
  if (!request || typeof request !== "object") {
    return failure("INVALID_OPTION", "Profiler analyzer request is required.");
  }

  if (
    typeof request.wallPath !== "string" ||
    !request.wallPath ||
    typeof request.wallIntervalMs !== "number" ||
    request.wallIntervalMs <= 0
  ) {
    return failure("INVALID_OPTION", "Wall collapsed file and positive interval are required.");
  }

  const readable = await isReadableFile(request.wallPath);
  if (!readable) {
    return failure("FILE_NOT_FOUND", "Selected wall collapsed file is not readable.", request.wallPath);
  }

  const args = [
    "profiler",
    "analyze-collapsed",
    "--wall",
    request.wallPath,
    "--wall-interval-ms",
    String(request.wallIntervalMs),
  ];

  if (request.elapsedSec !== undefined) {
    args.push("--elapsed-sec", String(request.elapsedSec));
  }

  if (request.topN !== undefined) {
    args.push("--top-n", String(request.topN));
  }

  return runAnalyzer<ProfilerCollapsedAnalysisResult>(
    args,
    isProfilerCollapsedAnalysisResult,
    "Python engine did not produce valid profiler AnalysisResult JSON.",
  );
}

async function runAnalyzer<T extends AnalysisResult>(
  analyzerArgs: string[],
  validateResult: AnalysisResultValidator<T>,
  invalidOutputMessage: string,
  ...extraArgs: string[][]
): Promise<AnalyzerResponse<T>> {
  const tempDir = path.join(tmpdir(), `archscope-${randomUUID()}`);
  const outputPath = path.join(tempDir, "analysis-result.json");

  await mkdir(tempDir, { recursive: true });

  try {
    const engineArgs = [...analyzerArgs, ...extraArgs.flat(), "--out", outputPath];
    const result = await execEngine(engineArgs);

    if (!result.ok) {
      return result;
    }

    const rawJson = await readFile(outputPath, "utf8");
    const parsed = JSON.parse(rawJson) as unknown;

    if (!validateResult(parsed)) {
      return failure(
        "ENGINE_OUTPUT_INVALID",
        invalidOutputMessage,
        rawJson.slice(0, 1000),
      );
    }

    return {
      ok: true,
      result: parsed,
      engine_messages: appendSchemaCompatibilityMessages(result.messages, parsed),
    };
  } catch (error) {
    return failure(
      "ENGINE_OUTPUT_INVALID",
      "Python engine output could not be read or parsed.",
      errorToDetail(error),
    );
  } finally {
    await rm(tempDir, { recursive: true, force: true });
  }
}

function execEngine(args: string[]): Promise<EngineProcessResponse> {
  const invocation = resolveEngineInvocation();

  return new Promise((resolve) => {
    const child = execFile(
      invocation.file,
      [...invocation.argsPrefix, ...args],
      {
        cwd: invocation.cwd,
        env: process.env,
        timeout: 60_000,
        maxBuffer: 1024 * 1024 * 4,
      },
      (error, stdout, stderr) => {
        if (error) {
          resolve({
            ok: false,
            error: {
              code: "ENGINE_EXITED",
              message: "Python engine failed while analyzing the selected file.",
              detail: compactDetail([errorToDetail(error), stderr, stdout]),
            },
          });
          return;
        }

        resolve({ ok: true, messages: splitEngineMessages([stderr, stdout]) });
      },
    );

    activeEngineProcesses.add(child);
    child.once("exit", () => {
      activeEngineProcesses.delete(child);
    });
  });
}

function terminateActiveEngineProcesses(): void {
  for (const child of activeEngineProcesses) {
    if (child.exitCode === null && !child.killed) {
      child.kill();
    }
    activeEngineProcesses.delete(child);
  }
}

function resolveEngineInvocation(): EngineInvocation {
  const configuredCommand = process.env.ARCHSCOPE_ENGINE_COMMAND;

  if (configuredCommand) {
    return {
      file: configuredCommand,
      argsPrefix: [],
      cwd: repoRoot,
    };
  }

  if (process.platform === "win32") {
    return {
      file: "powershell.exe",
      argsPrefix: [
        "-NoProfile",
        "-ExecutionPolicy",
        "Bypass",
        "-File",
        path.join(repoRoot, "scripts", "run-engine.ps1"),
      ],
      cwd: repoRoot,
    };
  }

  return {
    file: path.join(repoRoot, "scripts", "run-engine.sh"),
    argsPrefix: [],
    cwd: repoRoot,
  };
}

async function isReadableFile(filePath: string): Promise<boolean> {
  try {
    const fileStat = await stat(filePath);
    return fileStat.isFile();
  } catch {
    return false;
  }
}

function isAnalysisResult(value: unknown): value is AnalysisResult {
  if (!value || typeof value !== "object") {
    return false;
  }

  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.type === "string" &&
    typeof candidate.created_at === "string" &&
    Array.isArray(candidate.source_files) &&
    candidate.source_files.every((sourceFile) => typeof sourceFile === "string") &&
    isPlainObject(candidate.summary) &&
    isPlainObject(candidate.series) &&
    isPlainObject(candidate.tables) &&
    isPlainObject(candidate.charts) &&
    isPlainObject(candidate.metadata)
  );
}

function appendSchemaCompatibilityMessages(
  messages: string[],
  result: AnalysisResult,
): string[] | undefined {
  const schemaMessages = schemaCompatibilityMessages(result);
  const combined = [...messages, ...schemaMessages];
  return combined.length > 0 ? combined : undefined;
}

function schemaCompatibilityMessages(result: AnalysisResult): string[] {
  const messages: string[] = [];
  const type = result.type as SupportedResultType;
  const expectedVersion = SUPPORTED_SCHEMA_VERSIONS[type];
  if (!expectedVersion) {
    messages.push(
      `Warning: unsupported AnalysisResult type '${result.type}' returned by engine.`,
    );
    return messages;
  }

  const schemaVersion = result.metadata.schema_version;
  if (typeof schemaVersion !== "string") {
    messages.push(
      `Warning: ${result.type} result is missing metadata.schema_version.`,
    );
  } else if (schemaVersion !== expectedVersion) {
    messages.push(
      `Warning: ${result.type} schema_version ${schemaVersion} differs from supported ${expectedVersion}.`,
    );
  }

  const aiInterpretation = result.metadata.ai_interpretation;
  if (aiInterpretation !== undefined && !isInterpretationResultPayload(aiInterpretation)) {
    messages.push(
      "Warning: metadata.ai_interpretation is present but does not match the supported AI interpretation contract.",
    );
  }

  return messages;
}

function isAccessLogAnalysisResult(value: unknown): value is AccessLogAnalysisResult {
  if (!isAnalysisResult(value) || value.type !== "access_log") {
    return false;
  }

  return (
    hasNumber(value.summary, "total_requests") &&
    hasNumber(value.summary, "avg_response_ms") &&
    hasNumber(value.summary, "p95_response_ms") &&
    hasNumber(value.summary, "p99_response_ms") &&
    hasNumber(value.summary, "error_rate") &&
    Array.isArray(value.series.requests_per_minute) &&
    Array.isArray(value.series.avg_response_time_per_minute) &&
    Array.isArray(value.series.p95_response_time_per_minute) &&
    Array.isArray(value.series.status_code_distribution) &&
    Array.isArray(value.series.top_urls_by_count) &&
    Array.isArray(value.series.top_urls_by_avg_response_time) &&
    Array.isArray(value.tables.sample_records) &&
    isPlainObject(value.metadata.diagnostics)
  );
}

function isProfilerCollapsedAnalysisResult(
  value: unknown,
): value is ProfilerCollapsedAnalysisResult {
  if (!isAnalysisResult(value) || value.type !== "profiler_collapsed") {
    return false;
  }

  return (
    typeof value.summary.profile_kind === "string" &&
    hasNumber(value.summary, "total_samples") &&
    hasNumber(value.summary, "interval_ms") &&
    hasNumber(value.summary, "estimated_seconds") &&
    (typeof value.summary.elapsed_seconds === "number" ||
      value.summary.elapsed_seconds === null) &&
    Array.isArray(value.series.top_stacks) &&
    Array.isArray(value.series.component_breakdown) &&
    Array.isArray(value.tables.top_stacks) &&
    isPlainObject(value.metadata.diagnostics)
  );
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function hasNumber(value: Record<string, unknown>, key: string): boolean {
  return typeof value[key] === "number" && Number.isFinite(value[key]);
}

function isInterpretationResultPayload(value: unknown): boolean {
  if (!isPlainObject(value)) {
    return false;
  }

  return (
    value.schema_version === "0.1.0" &&
    typeof value.provider === "string" &&
    typeof value.model === "string" &&
    typeof value.prompt_version === "string" &&
    typeof value.source_result_type === "string" &&
    typeof value.source_schema_version === "string" &&
    typeof value.generated_at === "string" &&
    typeof value.disabled === "boolean" &&
    Array.isArray(value.findings) &&
    value.findings.every(isAiFindingPayload)
  );
}

function isAiFindingPayload(value: unknown): boolean {
  if (!isPlainObject(value)) {
    return false;
  }

  return (
    typeof value.id === "string" &&
    typeof value.label === "string" &&
    ["info", "warning", "critical"].includes(String(value.severity)) &&
    value.generated_by === "ai" &&
    typeof value.model === "string" &&
    typeof value.summary === "string" &&
    typeof value.reasoning === "string" &&
    Array.isArray(value.evidence_refs) &&
    value.evidence_refs.length > 0 &&
    value.evidence_refs.every((ref) => typeof ref === "string" && ref.trim()) &&
    typeof value.confidence === "number" &&
    Number.isFinite(value.confidence) &&
    value.confidence >= 0 &&
    value.confidence <= 1 &&
    Array.isArray(value.limitations) &&
    value.limitations.every((item) => typeof item === "string") &&
    isOptionalStringRecord(value.evidence_quotes)
  );
}

function isOptionalStringRecord(value: unknown): boolean {
  if (value === undefined) {
    return true;
  }
  if (!isPlainObject(value)) {
    return false;
  }
  return Object.values(value).every((item) => typeof item === "string");
}

function failure<T extends AnalysisResult>(
  code: string,
  message: string,
  detail?: string,
): AnalyzerResponse<T> {
  return {
    ok: false,
    error: {
      code,
      message,
      detail,
    },
  };
}

function ipcFailed<T extends AnalysisResult>(error: unknown): AnalyzerResponse<T> {
  return failure("IPC_FAILED", "Analyzer IPC request failed.", errorToDetail(error));
}

function errorToDetail(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }

  return String(error);
}

function compactDetail(parts: Array<string | undefined>): string | undefined {
  const detail = parts
    .map((part) => part?.trim())
    .filter((part): part is string => Boolean(part))
    .join("\n");

  return detail || undefined;
}

function splitEngineMessages(parts: Array<string | undefined>): string[] {
  return parts
    .flatMap((part) => part?.split(/\r?\n/) ?? [])
    .map((line) => line.trim())
    .filter((line) => line.length > 0)
    .slice(-20);
}
