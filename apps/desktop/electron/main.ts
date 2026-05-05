/**
 * ArchScope — Electron main process.
 *
 * Responsibilities:
 *  1. Create the BrowserWindow and load the React frontend.
 *  2. Spawn (or connect to) the Python archscope-engine.
 *  3. Expose the ArchScopeRendererApi contract via IPC so the renderer
 *     can call native dialogs, the engine, etc.
 */
import { spawn, execSync, type ChildProcess } from "node:child_process";
import { existsSync } from "node:fs";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import {
  app,
  BrowserWindow,
  dialog,
  ipcMain,
  shell,
  type OpenDialogOptions,
} from "electron";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// ---------------------------------------------------------------------------
// Paths
// ---------------------------------------------------------------------------

const IS_DEV = !app.isPackaged;
const ARCHSCOPE_HOME = path.join(app.getPath("home"), ".archscope");
const SETTINGS_PATH = path.join(ARCHSCOPE_HOME, "settings.json");

function frontendURL(): string {
  if (IS_DEV) {
    return process.env.ARCHSCOPE_DEV_URL ?? "http://127.0.0.1:5173";
  }
  const resourceDir = path.join(process.resourcesPath, "frontend");
  return `file://${path.join(resourceDir, "index.html")}`;
}

// ---------------------------------------------------------------------------
// Engine process management
// ---------------------------------------------------------------------------

let engineProcess: ChildProcess | null = null;
let enginePort = 8765;

function findBundledEngine(): string | null {
  // In a packaged build, the engine binary lives in resources/engine/.
  // PyInstaller onedir: resources/engine/archscope-engine[.exe]
  // PyInstaller onefile: resources/engine/archscope-engine[.exe]
  const ext = process.platform === "win32" ? ".exe" : "";
  const candidates = [
    // Packaged app
    path.join(process.resourcesPath ?? "", "engine", `archscope-engine${ext}`),
    // Onedir layout — binary is inside the directory
    path.join(process.resourcesPath ?? "", "engine", "archscope-engine", `archscope-engine${ext}`),
    // Dev: look relative to the repo root
    path.join(__dirname, "..", "..", "..", "engines", "python", "dist", "archscope-engine", `archscope-engine${ext}`),
    path.join(__dirname, "..", "..", "..", "engines", "python", "dist", `archscope-engine${ext}`),
  ];

  for (const candidate of candidates) {
    if (existsSync(candidate)) {
      console.log(`[archscope] Found bundled engine at: ${candidate}`);
      return candidate;
    }
  }
  return null;
}

function findEngineCommand(): string[] | null {
  // Priority 1: Bundled binary (no Python required)
  const bundled = findBundledEngine();
  if (bundled) {
    return [bundled];
  }

  // Priority 2: archscope-engine on PATH (pip-installed)
  try {
    execSync("archscope-engine --help", { stdio: "ignore" });
    return ["archscope-engine"];
  } catch { /* not found */ }

  // Priority 3: python -m archscope_engine.cli
  for (const pythonCmd of ["python", "python3"]) {
    try {
      execSync(`${pythonCmd} -c "import archscope_engine"`, { stdio: "ignore" });
      return [pythonCmd, "-m", "archscope_engine.cli"];
    } catch { /* not found */ }
  }

  return null;
}

async function startEngine(): Promise<boolean> {
  const cmd = findEngineCommand();
  if (!cmd) {
    console.warn("[archscope] Engine not found — running in UI-only mode.");
    return false;
  }

  const args = [...cmd.slice(1), "serve", "--port", String(enginePort)];
  const bin = cmd[0];

  console.log(`[archscope] Starting engine: ${bin} ${args.join(" ")}`);

  engineProcess = spawn(bin, args, {
    stdio: ["ignore", "pipe", "pipe"],
    env: { ...process.env },
  });

  engineProcess.stdout?.on("data", (chunk: Buffer) => {
    process.stdout.write(`[engine] ${chunk.toString()}`);
  });
  engineProcess.stderr?.on("data", (chunk: Buffer) => {
    process.stderr.write(`[engine] ${chunk.toString()}`);
  });
  engineProcess.on("exit", (code) => {
    console.log(`[archscope] Engine exited with code ${code}`);
    engineProcess = null;
  });

  // Wait until the engine is reachable.
  for (let i = 0; i < 30; i++) {
    await new Promise((r) => setTimeout(r, 500));
    try {
      const res = await fetch(`http://127.0.0.1:${enginePort}/api/version`);
      if (res.ok) return true;
    } catch { /* retry */ }
  }
  console.warn("[archscope] Engine did not become ready within 15 s.");
  return false;
}

function stopEngine(): void {
  if (engineProcess) {
    engineProcess.kill();
    engineProcess = null;
  }
}

// ---------------------------------------------------------------------------
// Engine HTTP helper
// ---------------------------------------------------------------------------

async function engineJson<T>(urlPath: string, init?: RequestInit): Promise<T> {
  const base = `http://127.0.0.1:${enginePort}`;
  const headers: Record<string, string> = { Accept: "application/json" };
  if (init?.body && typeof init.body === "string") {
    headers["Content-Type"] = "application/json";
  }
  const res = await fetch(`${base}${urlPath}`, { ...init, headers: { ...headers, ...(init?.headers as Record<string, string> ?? {}) } });
  return res.json() as Promise<T>;
}

// ---------------------------------------------------------------------------
// Settings helpers
// ---------------------------------------------------------------------------

const DEFAULT_SETTINGS = {
  enginePath: "",
  chartTheme: "light",
  locale: "en",
};

async function loadSettings(): Promise<Record<string, unknown>> {
  try {
    const raw = await readFile(SETTINGS_PATH, "utf-8");
    return { ...DEFAULT_SETTINGS, ...JSON.parse(raw) };
  } catch {
    return { ...DEFAULT_SETTINGS };
  }
}

async function saveSettings(settings: Record<string, unknown>): Promise<void> {
  await mkdir(ARCHSCOPE_HOME, { recursive: true });
  await writeFile(SETTINGS_PATH, JSON.stringify(settings, null, 2), "utf-8");
}

// ---------------------------------------------------------------------------
// IPC handlers — these mirror the ArchScopeRendererApi contract
// ---------------------------------------------------------------------------

function registerIpcHandlers(): void {
  // --- File dialogs ---
  ipcMain.handle("archscope:selectFile", async (_event, request?) => {
    const filters: Electron.FileFilter[] = [];
    if (request?.filters) {
      for (const f of request.filters) {
        filters.push({ name: f.name ?? "Files", extensions: f.extensions ?? ["*"] });
      }
    }
    const opts: OpenDialogOptions = {
      properties: ["openFile"],
      filters: filters.length > 0 ? filters : undefined,
    };
    const result = await dialog.showOpenDialog(opts);
    if (result.canceled || result.filePaths.length === 0) {
      return { canceled: true };
    }
    return { canceled: false, filePath: result.filePaths[0] };
  });

  ipcMain.handle("archscope:selectDirectory", async (_event, request?) => {
    const opts: OpenDialogOptions = {
      properties: ["openDirectory"],
      title: request?.title,
    };
    const result = await dialog.showOpenDialog(opts);
    if (result.canceled || result.filePaths.length === 0) {
      return { canceled: true };
    }
    return { canceled: false, directoryPath: result.filePaths[0] };
  });

  // --- Analyzer ---
  ipcMain.handle("archscope:analyzer:execute", async (_event, request) => {
    return engineJson("/api/analyzer/execute", {
      method: "POST",
      body: JSON.stringify(request),
    });
  });

  ipcMain.handle("archscope:analyzer:cancel", async (_event, body) => {
    return engineJson("/api/analyzer/cancel", {
      method: "POST",
      body: JSON.stringify(body),
    });
  });

  ipcMain.handle("archscope:analyzer:analyzeAccessLog", async (_event, request) => {
    return engineJson("/api/analyzer/execute", {
      method: "POST",
      body: JSON.stringify({ type: "access_log", requestId: request.requestId, params: request }),
    });
  });

  ipcMain.handle("archscope:analyzer:analyzeCollapsedProfile", async (_event, request) => {
    return engineJson("/api/analyzer/execute", {
      method: "POST",
      body: JSON.stringify({ type: "profiler_collapsed", requestId: request.requestId, params: request }),
    });
  });

  // --- Exporter ---
  ipcMain.handle("archscope:exporter:execute", async (_event, request) => {
    return engineJson("/api/export/execute", {
      method: "POST",
      body: JSON.stringify(request),
    });
  });

  // --- Demo ---
  ipcMain.handle("archscope:demo:list", async (_event, manifestRoot?) => {
    const query = manifestRoot ? `?manifestRoot=${encodeURIComponent(manifestRoot)}` : "";
    return engineJson(`/api/demo/list${query}`);
  });

  ipcMain.handle("archscope:demo:run", async (_event, request) => {
    return engineJson("/api/demo/run", {
      method: "POST",
      body: JSON.stringify(request),
    });
  });

  ipcMain.handle("archscope:demo:openPath", async (_event, filePath: string) => {
    try {
      await shell.openPath(filePath);
      return { ok: true };
    } catch (error) {
      return { ok: false, error: { code: "OPEN_FAILED", message: String(error) } };
    }
  });

  // --- Settings ---
  ipcMain.handle("archscope:settings:get", async () => {
    return loadSettings();
  });

  ipcMain.handle("archscope:settings:set", async (_event, settings) => {
    const merged = { ...(await loadSettings()), ...settings };
    await saveSettings(merged);
    return { ok: true };
  });
}

// ---------------------------------------------------------------------------
// Window creation
// ---------------------------------------------------------------------------

function createMainWindow(): BrowserWindow {
  const win = new BrowserWindow({
    width: 1440,
    height: 900,
    minWidth: 1024,
    minHeight: 600,
    title: "ArchScope",
    webPreferences: {
      preload: path.join(__dirname, "preload.js"),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: false,
    },
  });

  const url = frontendURL();
  if (url.startsWith("http")) {
    win.loadURL(url);
  } else {
    win.loadFile(url.replace("file://", ""));
  }

  // Lock the page zoom factor to 1.0. Without this Electron's default
  // Ctrl+(+/-/0) and Ctrl/Cmd+wheel handlers would scale the entire window —
  // including chart axis labels — and conflict with the chart's own zoom.
  win.webContents.on("did-finish-load", () => {
    win.webContents.setZoomFactor(1.0);
  });
  win.webContents.on("zoom-changed", () => {
    win.webContents.setZoomFactor(1.0);
  });
  win.webContents.on("before-input-event", (event, input) => {
    const isAccel = input.control || input.meta;
    if (!isAccel) return;
    if (input.key === "+" || input.key === "-" || input.key === "=" || input.key === "0") {
      event.preventDefault();
    }
  });

  if (IS_DEV || process.env.ARCHSCOPE_DEVTOOLS === "1") {
    win.webContents.openDevTools({ mode: "detach" });
  }

  return win;
}

// ---------------------------------------------------------------------------
// App lifecycle
// ---------------------------------------------------------------------------

app.whenReady().then(async () => {
  registerIpcHandlers();
  await startEngine();
  createMainWindow();

  app.on("activate", () => {
    if (BrowserWindow.getAllWindows().length === 0) {
      createMainWindow();
    }
  });
});

app.on("window-all-closed", () => {
  stopEngine();
  if (process.platform !== "darwin") {
    app.quit();
  }
});

app.on("before-quit", () => {
  stopEngine();
});
