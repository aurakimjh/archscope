import { app, BrowserWindow, dialog, ipcMain, session, shell } from "electron";
import { execFile, type ChildProcess } from "node:child_process";
import { randomUUID } from "node:crypto";
import { mkdir, readdir, readFile, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type {
  AccessLogAnalysisResult,
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
  AnalyzeFileRequest,
  AnalyzerExecuteRequest,
  AnalyzerExecutionResult,
  AnalyzerResponse,
  AnalysisResult,
  DemoListResponse,
  DemoOutputArtifact,
  DemoReferenceFile,
  DemoRunRequest,
  DemoRunResponse,
  DemoRunScenarioResult,
  DemoScenarioManifest,
  ExceptionStackAnalysisResult,
  ExportExecuteRequest,
  ExportResponse,
  GcLogAnalysisResult,
  ProfilerCollapsedAnalysisResult,
  SelectFileRequest,
  SelectFileResponse,
  ThreadDumpAnalysisResult,
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
type SupportedResultType =
  | "access_log"
  | "profiler_collapsed"
  | "jfr_recording"
  | "gc_log"
  | "thread_dump"
  | "exception_stack";

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
  gc_log: "0.1.0",
  thread_dump: "0.1.0",
  exception_stack: "0.1.0",
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
    "dialog:select-directory",
    async (_event, request?: { title?: string }) => {
      const result = await dialog.showOpenDialog({
        title: request?.title,
        properties: ["openDirectory"],
      });

      if (result.canceled || result.filePaths.length === 0) {
        return { canceled: true };
      }

      return {
        canceled: false,
        directoryPath: result.filePaths[0],
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

  ipcMain.handle(
    "export:execute",
    async (_event, request: ExportExecuteRequest): Promise<ExportResponse> => {
      try {
        return await executeExport(request);
      } catch (error) {
        return exportFailure("IPC_FAILED", "Export IPC request failed.", errorToDetail(error));
      }
    },
  );

  ipcMain.handle(
    "demo:list",
    async (_event, manifestRoot?: string): Promise<DemoListResponse> => {
      try {
        return await listDemoScenarios(manifestRoot || defaultDemoSiteRoot());
      } catch (error) {
        return demoFailure("DEMO_LIST_FAILED", "Demo manifest list failed.", error);
      }
    },
  );

  ipcMain.handle(
    "demo:run",
    async (_event, request: DemoRunRequest): Promise<DemoRunResponse> => {
      try {
        return await runDemoScenario(request);
      } catch (error) {
        return demoFailure("DEMO_RUN_FAILED", "Demo data execution failed.", error);
      }
    },
  );

  ipcMain.handle(
    "file:open",
    async (_event, targetPath: string) => {
      if (typeof targetPath !== "string" || !targetPath) {
        return exportFailure("INVALID_OPTION", "A file path is required.");
      }
      if (!(await isReadablePath(targetPath))) {
        return exportFailure("FILE_NOT_FOUND", "The requested path is not readable.", targetPath);
      }
      const errorMessage = await shell.openPath(targetPath);
      if (errorMessage) {
        return exportFailure("OPEN_FAILED", "Could not open the requested path.", errorMessage);
      }
      return { ok: true };
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
    case "gc_log":
      return analyzeJvmFile(
        request.params,
        ["gc-log", "analyze"],
        isGcLogAnalysisResult,
        "Python engine did not produce valid GC log AnalysisResult JSON.",
      );
    case "thread_dump":
      return analyzeJvmFile(
        request.params,
        ["thread-dump", "analyze"],
        isThreadDumpAnalysisResult,
        "Python engine did not produce valid thread dump AnalysisResult JSON.",
      );
    case "exception_stack":
      return analyzeJvmFile(
        request.params,
        ["exception", "analyze"],
        isExceptionStackAnalysisResult,
        "Python engine did not produce valid exception AnalysisResult JSON.",
      );
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

async function analyzeJvmFile<T extends AnalysisResult>(
  request: AnalyzeFileRequest,
  commandPrefix: string[],
  validateResult: AnalysisResultValidator<T>,
  invalidOutputMessage: string,
): Promise<AnalyzerResponse<T>> {
  if (!request || typeof request !== "object") {
    return failure("INVALID_OPTION", "Analyzer request is required.");
  }

  if (typeof request.filePath !== "string" || !request.filePath) {
    return failure("INVALID_OPTION", "Analyzer file path is required.");
  }

  if (
    request.topN !== undefined &&
    (!Number.isInteger(request.topN) || request.topN <= 0)
  ) {
    return failure("INVALID_OPTION", "Top N must be a positive integer.");
  }

  const readable = await isReadableFile(request.filePath);
  if (!readable) {
    return failure("FILE_NOT_FOUND", "Selected file is not readable.", request.filePath);
  }

  const args = [...commandPrefix, "--file", request.filePath];
  if (request.topN !== undefined) {
    args.push("--top-n", String(request.topN));
  }

  return runAnalyzer<T>(args, validateResult, invalidOutputMessage);
}

async function executeExport(request: ExportExecuteRequest): Promise<ExportResponse> {
  if (!request || typeof request !== "object") {
    return exportFailure("INVALID_OPTION", "Export request is required.");
  }

  switch (request.format) {
    case "html":
      return exportSingleFile(request.inputPath, "html");
    case "pptx":
      return exportSingleFile(request.inputPath, "pptx");
    case "diff":
      return exportDiff(request.beforePath, request.afterPath);
    default:
      return exportFailure("INVALID_OPTION", "Unsupported export format.");
  }
}

async function listDemoScenarios(manifestRoot: string): Promise<DemoListResponse> {
  if (typeof manifestRoot !== "string" || !manifestRoot) {
    return demoFailure("INVALID_OPTION", "Demo manifest root is required.");
  }
  const readable = await isReadablePath(manifestRoot);
  if (!readable) {
    return demoFailure("FILE_NOT_FOUND", "Demo manifest root is not readable.", manifestRoot);
  }
  const manifestPaths = await discoverManifestPaths(manifestRoot);
  const scenarios = await Promise.all(
    manifestPaths.map(async (manifestPath): Promise<DemoScenarioManifest> => {
      const payload = JSON.parse(await readFile(manifestPath, "utf8")) as Record<string, unknown>;
      const files = Array.isArray(payload.files) ? payload.files : [];
      const analyzers = files
        .map((item) => {
          if (!item || typeof item !== "object") {
            return null;
          }
          const analyzerType = (item as Record<string, unknown>).analyzer_type;
          return typeof analyzerType === "string" ? analyzerType : null;
        })
        .filter((item): item is string => Boolean(item));
      return {
        scenario: typeof payload.scenario === "string" ? payload.scenario : path.basename(path.dirname(manifestPath)),
        dataSource: dataSourceForManifest(payload, manifestPath),
        manifestPath,
        description: typeof payload.description === "string" ? payload.description : "",
        analyzers,
      };
    }),
  );
  return {
    ok: true,
    manifestRoot,
    scenarios: scenarios.sort((left, right) =>
      `${left.dataSource}/${left.scenario}`.localeCompare(`${right.dataSource}/${right.scenario}`),
    ),
  };
}

async function runDemoScenario(request: DemoRunRequest): Promise<DemoRunResponse> {
  if (!request || typeof request !== "object") {
    return demoFailure("INVALID_OPTION", "Demo run request is required.");
  }
  if (typeof request.manifestRoot !== "string" || !request.manifestRoot) {
    return demoFailure("INVALID_OPTION", "Demo manifest root is required.");
  }
  const outputRoot =
    typeof request.outputRoot === "string" && request.outputRoot
      ? request.outputRoot
      : path.join(repoRoot, "demo-site-report-bundles");
  const args = ["demo-site", "run", "--manifest-root", request.manifestRoot, "--out", outputRoot];
  if (typeof request.scenario === "string" && request.scenario) {
    args.push("--scenario", request.scenario);
  }
  if (request.dataSource === "real" || request.dataSource === "synthetic") {
    args.push("--data-source", request.dataSource);
  }
  const result = await execEngine(args);
  if (!result.ok) {
    return result;
  }
  const scenarios = await listDemoScenarios(request.manifestRoot);
  const selectedScenarios = scenarios.ok
    ? scenarios.scenarios.filter(
        (scenario) =>
          (!request.scenario || scenario.scenario === request.scenario) &&
          (!request.dataSource || scenario.dataSource === request.dataSource),
      )
    : [];
  const outputPaths = [
    path.join(outputRoot, "index.html"),
    ...selectedScenarios.map((scenario) =>
      path.join(outputRoot, scenario.dataSource, scenario.scenario, "index.html"),
    ),
  ];
  const scenarioResults = (
    await Promise.all(
      selectedScenarios.map((scenario) =>
        readDemoRunScenarioResult(outputRoot, scenario),
      ),
    )
  ).filter((item): item is DemoRunScenarioResult => Boolean(item));
  const exportInputPaths = scenarioResults
    .flatMap((scenario) => scenario.artifacts)
    .filter((artifact) => artifact.exportable)
    .map((artifact) => artifact.path);
  return {
    ok: true,
    outputPaths,
    exportInputPaths,
    scenarios: scenarioResults,
    engine_messages: result.messages.length > 0 ? result.messages : undefined,
  };
}

async function exportSingleFile(
  inputPath: string,
  format: "html" | "pptx",
): Promise<ExportResponse> {
  if (typeof inputPath !== "string" || !inputPath) {
    return exportFailure("INVALID_OPTION", "Input JSON path is required.");
  }
  if (!(await isReadableFile(inputPath))) {
    return exportFailure("FILE_NOT_FOUND", "Selected JSON file is not readable.", inputPath);
  }

  const outputPath = siblingOutputPath(inputPath, format);
  const command =
    format === "html"
      ? ["report", "html", "--input", inputPath, "--out", outputPath]
      : ["report", "pptx", "--input", inputPath, "--out", outputPath];
  const result = await execEngine(command);
  if (!result.ok) {
    return result;
  }
  return {
    ok: true,
    outputPaths: [outputPath],
    engine_messages: result.messages.length > 0 ? result.messages : undefined,
  };
}

async function exportDiff(
  beforePath: string,
  afterPath: string,
): Promise<ExportResponse> {
  if (
    typeof beforePath !== "string" ||
    !beforePath ||
    typeof afterPath !== "string" ||
    !afterPath
  ) {
    return exportFailure("INVALID_OPTION", "Before and after JSON paths are required.");
  }
  if (!(await isReadableFile(beforePath))) {
    return exportFailure("FILE_NOT_FOUND", "Before JSON file is not readable.", beforePath);
  }
  if (!(await isReadableFile(afterPath))) {
    return exportFailure("FILE_NOT_FOUND", "After JSON file is not readable.", afterPath);
  }

  const outputBase = `${baseNameWithoutExt(beforePath)}-vs-${baseNameWithoutExt(afterPath)}`;
  const outputDir = path.dirname(afterPath);
  const jsonOutput = path.join(outputDir, `${outputBase}-diff.json`);
  const htmlOutput = path.join(outputDir, `${outputBase}-diff.html`);
  const result = await execEngine([
    "report",
    "diff",
    "--before",
    beforePath,
    "--after",
    afterPath,
    "--out",
    jsonOutput,
    "--html-out",
    htmlOutput,
  ]);
  if (!result.ok) {
    return result;
  }
  return {
    ok: true,
    outputPaths: [jsonOutput, htmlOutput],
    engine_messages: result.messages.length > 0 ? result.messages : undefined,
  };
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
    const engineArgs = [
      ...analyzerArgs,
      ...extraArgs.flat(),
      "--debug-log-dir",
      resolveDebugLogDir(),
      "--out",
      outputPath,
    ];
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

function resolveDebugLogDir(): string {
  const baseDir = app.isPackaged ? path.dirname(process.execPath) : repoRoot;
  return path.join(baseDir, "archscope-debug");
}

async function isReadableFile(filePath: string): Promise<boolean> {
  try {
    const fileStat = await stat(filePath);
    return fileStat.isFile();
  } catch {
    return false;
  }
}

async function isReadablePath(filePath: string): Promise<boolean> {
  try {
    await stat(filePath);
    return true;
  } catch {
    return false;
  }
}

async function discoverManifestPaths(manifestRoot: string): Promise<string[]> {
  const rootStat = await stat(manifestRoot);
  if (rootStat.isFile()) {
    return [manifestRoot];
  }
  const manifestPaths: string[] = [];
  for (const sourceEntry of await readdir(manifestRoot, { withFileTypes: true })) {
    if (!sourceEntry.isDirectory()) {
      continue;
    }
    const sourceDir = path.join(manifestRoot, sourceEntry.name);
    for (const scenarioEntry of await readdir(sourceDir, { withFileTypes: true })) {
      if (!scenarioEntry.isDirectory()) {
        continue;
      }
      const manifestPath = path.join(sourceDir, scenarioEntry.name, "manifest.json");
      if (await isReadableFile(manifestPath)) {
        manifestPaths.push(manifestPath);
      }
    }
  }
  return manifestPaths;
}

function dataSourceForManifest(
  payload: Record<string, unknown>,
  manifestPath: string,
): "real" | "synthetic" | "unknown" {
  if (payload.data_source === "real" || payload.data_source === "synthetic") {
    return payload.data_source;
  }
  const segments = manifestPath.split(path.sep);
  if (segments.includes("real")) {
    return "real";
  }
  if (segments.includes("synthetic")) {
    return "synthetic";
  }
  return "unknown";
}

async function firstJsonOutput(outputDir: string): Promise<string | null> {
  try {
    const entries = await readdir(outputDir);
    const match = entries
      .filter((entry) => entry.endsWith(".json") && entry !== "run-summary.json")
      .sort()[0];
    return match ? path.join(outputDir, match) : null;
  } catch {
    return null;
  }
}

async function readDemoRunScenarioResult(
  outputRoot: string,
  scenario: DemoScenarioManifest,
): Promise<DemoRunScenarioResult | null> {
  const scenarioDir = path.join(outputRoot, scenario.dataSource, scenario.scenario);
  const summaryPath = path.join(scenarioDir, "run-summary.json");
  try {
    const payload = JSON.parse(await readFile(summaryPath, "utf8")) as Record<string, unknown>;
    const summary = isPlainObject(payload.summary) ? payload.summary : {};
    return {
      scenario: scenario.scenario,
      dataSource: scenario.dataSource,
      bundleIndexPath: path.join(scenarioDir, "index.html"),
      summaryPath,
      summary: {
        analyzerOutputs: numberValue(summary.analyzer_outputs),
        failedAnalyzers: numberValue(summary.failed_analyzers),
        skippedLines: numberValue(summary.skipped_lines),
        referenceFiles: numberValue(summary.reference_files),
        findingCount: numberValue(summary.finding_count),
        comparisonReports: numberValue(summary.comparison_reports),
      },
      artifacts: artifactsFromRunSummary(payload, scenarioDir),
      referenceFiles: referenceFilesFromRunSummary(payload),
      failedAnalyzers: failedAnalyzersFromRunSummary(payload),
      skippedLineReport: skippedLinesFromRunSummary(payload),
    };
  } catch {
    const fallbackJson = await firstJsonOutput(scenarioDir);
    return {
      scenario: scenario.scenario,
      dataSource: scenario.dataSource,
      bundleIndexPath: path.join(scenarioDir, "index.html"),
      summaryPath,
      summary: {
        analyzerOutputs: fallbackJson ? 1 : 0,
        failedAnalyzers: 0,
        skippedLines: 0,
        referenceFiles: 0,
        findingCount: 0,
        comparisonReports: 0,
      },
      artifacts: fallbackJson
        ? [{ kind: "json", label: path.basename(fallbackJson), path: fallbackJson, exportable: true }]
        : [],
      referenceFiles: [],
      failedAnalyzers: [],
      skippedLineReport: [],
    };
  }
}

function artifactsFromRunSummary(
  payload: Record<string, unknown>,
  scenarioDir: string,
): DemoOutputArtifact[] {
  const artifacts: DemoOutputArtifact[] = [
    {
      kind: "index",
      label: "index.html",
      path: path.join(scenarioDir, "index.html"),
      exportable: false,
    },
    {
      kind: "summary",
      label: "run-summary.json",
      path: path.join(scenarioDir, "run-summary.json"),
      exportable: false,
    },
  ];
  const analyzerRuns = Array.isArray(payload.analyzer_runs) ? payload.analyzer_runs : [];
  for (const item of analyzerRuns) {
    if (!isPlainObject(item)) {
      continue;
    }
    const file = typeof item.file === "string" ? item.file : "analyzer result";
    const analyzerType = typeof item.analyzer_type === "string" ? item.analyzer_type : "";
    for (const [key, kind] of [
      ["json_path", "json"],
      ["html_path", "html"],
      ["pptx_path", "pptx"],
    ] as const) {
      const outputPath = item[key];
      if (typeof outputPath === "string" && outputPath) {
        artifacts.push({
          kind,
          label: `${file} ${analyzerType} ${kind.toUpperCase()}`,
          path: outputPath,
          exportable: kind === "json",
        });
      }
    }
  }
  const comparisonPaths = Array.isArray(payload.comparison_paths) ? payload.comparison_paths : [];
  for (const item of comparisonPaths) {
    if (typeof item !== "string") {
      continue;
    }
    artifacts.push({
      kind: "comparison",
      label: path.basename(item),
      path: item,
      exportable: item.endsWith(".json"),
    });
  }
  return artifacts;
}

function referenceFilesFromRunSummary(payload: Record<string, unknown>): DemoReferenceFile[] {
  const referenceFiles = Array.isArray(payload.reference_files) ? payload.reference_files : [];
  return referenceFiles.flatMap((item): DemoReferenceFile[] => {
    if (!isPlainObject(item) || typeof item.file !== "string" || typeof item.path !== "string") {
      return [];
    }
    return [
      {
        file: item.file,
        path: item.path,
        description: typeof item.description === "string" ? item.description : undefined,
      },
    ];
  });
}

function failedAnalyzersFromRunSummary(
  payload: Record<string, unknown>,
): DemoRunScenarioResult["failedAnalyzers"] {
  const failedAnalyzers = Array.isArray(payload.failed_analyzers)
    ? payload.failed_analyzers
    : [];
  return failedAnalyzers.flatMap((item) => {
    if (!isPlainObject(item) || typeof item.file !== "string") {
      return [];
    }
    return [
      {
        file: item.file,
        analyzerType:
          typeof item.analyzer_type === "string" ? item.analyzer_type : "unknown",
        error: typeof item.error === "string" ? item.error : undefined,
      },
    ];
  });
}

function skippedLinesFromRunSummary(
  payload: Record<string, unknown>,
): DemoRunScenarioResult["skippedLineReport"] {
  const skippedLines = Array.isArray(payload.skipped_line_report)
    ? payload.skipped_line_report
    : [];
  return skippedLines.flatMap((item) => {
    if (!isPlainObject(item) || typeof item.file !== "string") {
      return [];
    }
    return [
      {
        file: item.file,
        analyzerType:
          typeof item.analyzer_type === "string" ? item.analyzer_type : "unknown",
        skippedLines: numberValue(item.skipped_lines),
      },
    ];
  });
}

function numberValue(value: unknown): number {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}

function defaultDemoSiteRoot(): string {
  return path.join(repoRoot, "..", "projects-assets", "test-data", "demo-site");
}

function siblingOutputPath(inputPath: string, extension: "html" | "pptx"): string {
  return path.join(path.dirname(inputPath), `${baseNameWithoutExt(inputPath)}.${extension}`);
}

function baseNameWithoutExt(filePath: string): string {
  const parsed = path.parse(filePath);
  return parsed.name || "archscope-report";
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

function isGcLogAnalysisResult(value: unknown): value is GcLogAnalysisResult {
  if (!isAnalysisResult(value) || value.type !== "gc_log") {
    return false;
  }

  return (
    hasNumber(value.summary, "total_events") &&
    hasNumber(value.summary, "total_pause_ms") &&
    hasNumber(value.summary, "avg_pause_ms") &&
    hasNumber(value.summary, "max_pause_ms") &&
    hasNumber(value.summary, "young_gc_count") &&
    hasNumber(value.summary, "full_gc_count") &&
    isPlainObject(value.metadata.diagnostics)
  );
}

function isThreadDumpAnalysisResult(value: unknown): value is ThreadDumpAnalysisResult {
  if (!isAnalysisResult(value) || value.type !== "thread_dump") {
    return false;
  }

  return (
    hasNumber(value.summary, "total_threads") &&
    hasNumber(value.summary, "runnable_threads") &&
    hasNumber(value.summary, "blocked_threads") &&
    hasNumber(value.summary, "waiting_threads") &&
    hasNumber(value.summary, "threads_with_locks") &&
    isPlainObject(value.metadata.diagnostics)
  );
}

function isExceptionStackAnalysisResult(
  value: unknown,
): value is ExceptionStackAnalysisResult {
  if (!isAnalysisResult(value) || value.type !== "exception_stack") {
    return false;
  }

  return (
    hasNumber(value.summary, "total_exceptions") &&
    hasNumber(value.summary, "unique_exception_types") &&
    hasNumber(value.summary, "unique_signatures") &&
    (typeof value.summary.top_exception_type === "string" ||
      value.summary.top_exception_type === null) &&
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

function exportFailure(
  code: string,
  message: string,
  detail?: string,
): ExportResponse {
  return {
    ok: false,
    error: {
      code,
      message,
      detail,
    },
  };
}

function demoFailure(
  code: string,
  message: string,
  detail?: unknown,
): DemoListResponse & DemoRunResponse {
  return {
    ok: false,
    error: {
      code,
      message,
      detail: detail === undefined ? undefined : errorToDetail(detail),
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
