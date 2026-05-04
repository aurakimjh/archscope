/**
 * ArchScope — Electron preload script.
 *
 * Exposes the ArchScopeRendererApi contract on `window.archscope` via
 * contextBridge so the React frontend can call native file dialogs and
 * the engine through IPC without any code changes.
 */
import { contextBridge, ipcRenderer } from "electron";

/**
 * The shape exposed to the renderer matches ArchScopeRendererApi from
 * apps/frontend/src/api/analyzerContract.ts.
 */
const archscope = {
  platform: "electron" as const,

  selectFile: (request?: unknown) =>
    ipcRenderer.invoke("archscope:selectFile", request),

  selectDirectory: (request?: unknown) =>
    ipcRenderer.invoke("archscope:selectDirectory", request),

  analyzer: {
    execute: (request: unknown) =>
      ipcRenderer.invoke("archscope:analyzer:execute", request),
    cancel: (requestId: string) =>
      ipcRenderer.invoke("archscope:analyzer:cancel", { requestId }),
    analyzeAccessLog: (request: unknown) =>
      ipcRenderer.invoke("archscope:analyzer:analyzeAccessLog", request),
    analyzeCollapsedProfile: (request: unknown) =>
      ipcRenderer.invoke("archscope:analyzer:analyzeCollapsedProfile", request),
  },

  exporter: {
    execute: (request: unknown) =>
      ipcRenderer.invoke("archscope:exporter:execute", request),
  },

  demo: {
    list: (manifestRoot?: string) =>
      ipcRenderer.invoke("archscope:demo:list", manifestRoot),
    run: (request: unknown) =>
      ipcRenderer.invoke("archscope:demo:run", request),
    openPath: (filePath: string) =>
      ipcRenderer.invoke("archscope:demo:openPath", filePath),
  },

  settings: {
    get: () => ipcRenderer.invoke("archscope:settings:get"),
    set: (settings: unknown) =>
      ipcRenderer.invoke("archscope:settings:set", settings),
  },
};

contextBridge.exposeInMainWorld("archscope", archscope);
