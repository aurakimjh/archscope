import { app, BrowserWindow, dialog, ipcMain } from "electron";
import { execFile } from "node:child_process";
import { randomUUID } from "node:crypto";
import { mkdir, readFile, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type {
  AccessLogAnalysisResult,
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
  AnalyzerResponse,
  AnalysisResult,
  ProfilerCollapsedAnalysisResult,
  SelectFileRequest,
  SelectFileResponse,
} from "../src/api/analyzerContract.js";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const repoRoot = path.resolve(__dirname, "../../..");

type EngineInvocation = {
  file: string;
  argsPrefix: string[];
  cwd: string;
};

type EngineProcessResponse =
  | { ok: true }
  | { ok: false; error: { code: string; message: string; detail?: string } };

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
    "analyzer:access-log:analyze",
    async (
      _event,
      request: AnalyzeAccessLogRequest,
    ): Promise<AnalyzerResponse<AccessLogAnalysisResult>> => {
      try {
        return await analyzeAccessLog(request);
      } catch (error) {
        return ipcFailed(error);
      }
    },
  );

  ipcMain.handle(
    "analyzer:profiler:analyze-collapsed",
    async (
      _event,
      request: AnalyzeCollapsedProfileRequest,
    ): Promise<AnalyzerResponse<ProfilerCollapsedAnalysisResult>> => {
      try {
        return await analyzeCollapsedProfile(request);
      } catch (error) {
        return ipcFailed(error);
      }
    },
  );
}

async function analyzeAccessLog(
  request: AnalyzeAccessLogRequest,
): Promise<AnalyzerResponse<AccessLogAnalysisResult>> {
  if (!request.filePath || !request.format) {
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

  return runAnalyzer<AccessLogAnalysisResult>([
    ...[
      "access-log",
      "analyze",
      "--file",
      request.filePath,
      "--format",
      request.format,
    ],
    ...(request.maxLines !== undefined ? ["--max-lines", String(request.maxLines)] : []),
    ...(request.startTime ? ["--start-time", request.startTime] : []),
    ...(request.endTime ? ["--end-time", request.endTime] : []),
  ]);
}

async function analyzeCollapsedProfile(
  request: AnalyzeCollapsedProfileRequest,
): Promise<AnalyzerResponse<ProfilerCollapsedAnalysisResult>> {
  if (!request.wallPath || request.wallIntervalMs <= 0) {
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

  return runAnalyzer<ProfilerCollapsedAnalysisResult>(args);
}

async function runAnalyzer<T extends AnalysisResult>(
  analyzerArgs: string[],
): Promise<AnalyzerResponse<T>> {
  const tempDir = path.join(tmpdir(), `archscope-${randomUUID()}`);
  const outputPath = path.join(tempDir, "analysis-result.json");

  await mkdir(tempDir, { recursive: true });

  try {
    const engineArgs = [...analyzerArgs, "--out", outputPath];
    const result = await execEngine(engineArgs);

    if (!result.ok) {
      return result;
    }

    const rawJson = await readFile(outputPath, "utf8");
    const parsed = JSON.parse(rawJson) as unknown;

    if (!isAnalysisResult(parsed)) {
      return failure(
        "ENGINE_OUTPUT_INVALID",
        "Python engine did not produce valid AnalysisResult JSON.",
        rawJson.slice(0, 1000),
      );
    }

    return {
      ok: true,
      result: parsed as T,
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
    execFile(
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
          resolve(
            failure(
              "ENGINE_EXITED",
              "Python engine failed while analyzing the selected file.",
              compactDetail([errorToDetail(error), stderr, stdout]),
            ),
          );
          return;
        }

        resolve({ ok: true });
      },
    );
  });
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
    Array.isArray(candidate.source_files) &&
    isPlainObject(candidate.summary) &&
    isPlainObject(candidate.series)
  );
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
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
