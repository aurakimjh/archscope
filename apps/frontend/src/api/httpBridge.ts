// ─────────────────────────────────────────────────────────────────────
// [한글] httpBridge.ts — `window.archscope` 를 HTTP fetch 로 구현하는
//   웹 빌드용 브리지(데스크톱이 아닌 환경에서 같은 API 표면을 노출).
//
// 책임/목적:
//   - 데스크톱(Electron) 빌드는 preload 가 `window.archscope` 를 IPC 로
//     채워 줍니다. 웹 빌드는 그게 없으므로, 본 파일이 동일한 형태의
//     ArchScopeRendererApi 를 만들어 `window.archscope` 에 주입합니다.
//   - 모든 호출은 FastAPI(`engines/python/.../web/server.py`) 로 향하는
//     fetch 로 변환됩니다 — 페이지 컴포넌트는 어떤 환경에서도
//     `window.archscope.analyzer.execute(...)` 만 호출하면 됩니다.
//
// 데이터 흐름:
//   페이지 → window.archscope.<group>.<method> → callJson(`/api/...`)
//   → FastAPI → Python/Go 엔진 → JSON 응답.
//
// 핵심 동작 메모:
//   - selectFile: 브라우저는 절대 경로 노출 X → <input type=file> 으로
//     사용자가 고른 파일을 서버로 업로드(uploadFileViaApi) 한 뒤,
//     서버 측 임시 경로를 filePath 로 돌려줍니다.
//   - selectDirectory: 브라우저에서 폴더 절대경로를 얻을 수 없어
//     `window.prompt` 로 사용자가 직접 입력하도록 fallback. Phase 2 에서
//     서버 주도 매니페스트 브라우저로 교체 예정.
//   - openPath(demo 결과 파일 열기): 새 탭에서 `/api/files?path=...` 를
//     열어 백엔드가 직접 파일을 서빙(데스크톱 셸 open 의 웹 대체).
// ─────────────────────────────────────────────────────────────────────
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

// [한글] callJson — fetch 의 boilerplate(JSON 직렬화/Content-Type 헤더/
//   에러 메시지 표준화)를 한 군데로 모은 helper.
//   - body 가 FormData 면 Content-Type 을 설정하지 않음(브라우저가
//     boundary 를 자동 부여하도록).
//   - HTTP 200 외에는 `Error("HTTP <status>: <detail>")` 를 throw → 호출
//     측은 try/catch 한 곳에서 묶어 처리.
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

// [한글] pickFile — 숨겨진 <input type=file> 을 동적으로 만들어 사용자
//   파일 선택을 Promise 로 감싸는 헬퍼. 사용자가 다이얼로그를
//   취소했을 때 브라우저가 이벤트를 안 쏘는 문제를 window 'focus'
//   이벤트 + 250ms 지연으로 우회합니다(관례적 패턴).
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

// [한글] createHttpBridge — ArchScopeRendererApi 모양에 맞춰 모든
//   메서드를 fetch 호출로 채운 객체를 만들어 반환.
//   페이지/컴포넌트 코드에서는 이 객체와 Electron preload 가 만든
//   객체를 구분 없이 사용합니다.
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

// [한글] installHttpBridge — main.tsx 의 부트스트랩 단계에서 1회 호출.
//   `window.archscope` 가 이미 있으면(데스크톱/preload 가 먼저 주입한
//   경우) 덮어쓰지 않습니다. 즉 데스크톱 우선 + 웹 fallback.
export function installHttpBridge(): void {
  if (typeof window === "undefined") return;
  if (window.archscope) return;
  window.archscope = createHttpBridge();
}
