/**
 * HTTP-backed implementation of the ArchScope renderer bridge.
 *
 * The Electron build mounted the same surface on `window.archscope` via IPC.
 * The web build uploads files through the FastAPI server (see
 * `engines/python/archscope_engine/web/server.py`) and forwards every other
 * call through HTTP. The renderer code keeps using `window.archscope.*`
 * unchanged.
 */
import { uploadFile as uploadFileViaApi } from "@/lib/uploadFile";

import type {
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
  AnalyzerCancelResponse,
  AnalyzerExecuteRequest,
  AnalyzerExecutionResult,
  AnalyzerResponse,
  AppSettings,
  ArchScopeRendererApi,
  DemoListResponse,
  DemoRunRequest,
  DemoRunResponse,
  ExportExecuteRequest,
  ExportResponse,
  SelectDirectoryResponse,
  SelectFileRequest,
  SelectFileResponse,
} from "./analyzerContract";

const API_BASE = "/api";

type FetchInit = Omit<RequestInit, "body"> & { body?: unknown };

async function callJson<T>(path: string, init: FetchInit = {}): Promise<T> {
  const headers: HeadersInit = { Accept: "application/json", ...(init.headers ?? {}) };
  const requestInit: RequestInit = { ...init, headers } as RequestInit;
  if (init.body !== undefined && !(init.body instanceof FormData)) {
    (headers as Record<string, string>)["Content-Type"] = "application/json";
    requestInit.body = JSON.stringify(init.body);
  } else if (init.body instanceof FormData) {
    requestInit.body = init.body;
  }
  const response = await fetch(`${API_BASE}${path}`, requestInit);
  if (!response.ok) {
    const detail = await response.text().catch(() => "");
    throw new Error(`HTTP ${response.status}: ${detail || response.statusText}`);
  }
  return (await response.json()) as T;
}

function extensionsFromFilters(filters?: SelectFileRequest["filters"]): string {
  if (!filters || filters.length === 0) return "";
  const exts = new Set<string>();
  for (const filter of filters) {
    for (const ext of filter.extensions ?? []) {
      if (ext === "*" || ext === "") continue;
      exts.add(`.${ext.replace(/^\./, "")}`);
    }
  }
  return Array.from(exts).join(",");
}

function pickFile(accept: string): Promise<File | null> {
  return new Promise((resolve) => {
    const input = document.createElement("input");
    input.type = "file";
    if (accept) input.accept = accept;
    input.style.display = "none";
    let settled = false;
    const finish = (file: File | null) => {
      if (settled) return;
      settled = true;
      input.remove();
      resolve(file);
    };
    input.addEventListener("change", () => {
      const file = input.files?.[0] ?? null;
      finish(file);
    });
    // The browser does not fire an event when the user cancels the dialog,
    // but the focus event on window is the conventional fallback.
    const onFocus = () => {
      window.removeEventListener("focus", onFocus);
      window.setTimeout(() => {
        if (!settled && (!input.files || input.files.length === 0)) finish(null);
      }, 250);
    };
    window.addEventListener("focus", onFocus);
    document.body.appendChild(input);
    input.click();
  });
}

async function selectFile(request?: SelectFileRequest): Promise<SelectFileResponse> {
  const file = await pickFile(extensionsFromFilters(request?.filters));
  if (!file) return { canceled: true };
  try {
    const uploaded = await uploadFileViaApi(file);
    return { canceled: false, filePath: uploaded.filePath };
  } catch (error) {
    console.error("[archscope] selectFile upload failed", error);
    return { canceled: true };
  }
}

async function selectDirectory(_request?: { title?: string }): Promise<SelectDirectoryResponse> {
  // Browsers cannot expose a real filesystem path for a folder. For dev/web
  // mode we accept a typed absolute path on the server. Phase 2 will replace
  // this with a server-driven manifest browser.
  const initial = window.prompt(
    "Enter the absolute server-side path to use as the manifest/output root.",
  );
  if (!initial) return { canceled: true };
  return { canceled: false, directoryPath: initial };
}

function buildExecuteBody(request: AnalyzerExecuteRequest): unknown {
  return request;
}

export function createHttpBridge(): ArchScopeRendererApi {
  return {
    platform: "web",
    selectFile,
    selectDirectory,
    analyzer: {
      execute: (request: AnalyzerExecuteRequest) =>
        callJson<AnalyzerResponse<AnalyzerExecutionResult>>("/analyzer/execute", {
          method: "POST",
          body: buildExecuteBody(request),
        }),
      cancel: (requestId: string) =>
        callJson<AnalyzerCancelResponse>("/analyzer/cancel", {
          method: "POST",
          body: { requestId },
        }),
      analyzeAccessLog: (request: AnalyzeAccessLogRequest) =>
        callJson("/analyzer/execute", {
          method: "POST",
          body: { type: "access_log", requestId: request.requestId, params: request },
        }),
      analyzeCollapsedProfile: (request: AnalyzeCollapsedProfileRequest) =>
        callJson("/analyzer/execute", {
          method: "POST",
          body: { type: "profiler_collapsed", requestId: request.requestId, params: request },
        }),
    },
    exporter: {
      execute: (request: ExportExecuteRequest) =>
        callJson<ExportResponse>("/export/execute", {
          method: "POST",
          body: request,
        }),
    },
    demo: {
      list: (manifestRoot?: string) => {
        const query = manifestRoot
          ? `?manifestRoot=${encodeURIComponent(manifestRoot)}`
          : "";
        return callJson<DemoListResponse>(`/demo/list${query}`, { method: "GET" });
      },
      run: (request: DemoRunRequest) =>
        callJson<DemoRunResponse>("/demo/run", {
          method: "POST",
          body: request,
        }),
      openPath: async (path: string) => {
        try {
          const url = `${API_BASE}/files?path=${encodeURIComponent(path)}`;
          window.open(url, "_blank", "noopener,noreferrer");
          return { ok: true } as const;
        } catch (error) {
          return {
            ok: false,
            error: { code: "OPEN_FAILED", message: String(error) },
          } as const;
        }
      },
    },
    settings: {
      get: () => callJson<AppSettings>("/settings", { method: "GET" }),
      set: (settings: AppSettings) =>
        callJson<{ ok: boolean }>("/settings", {
          method: "PUT",
          body: settings,
        }),
    },
  };
}

export function installHttpBridge(): void {
  if (typeof window === "undefined") return;
  if (window.archscope) return;
  window.archscope = createHttpBridge();
}
