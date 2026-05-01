import { contextBridge, ipcRenderer } from "electron";

import type {
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
  AnalyzerExecuteRequest,
  DemoRunRequest,
  ExportExecuteRequest,
  SelectFileRequest,
} from "../src/api/analyzerContract.js";

contextBridge.exposeInMainWorld("archscope", {
  platform: process.platform,
  selectFile: (request?: SelectFileRequest) =>
    ipcRenderer.invoke("dialog:select-file", request),
  selectDirectory: (request?: { title?: string }) =>
    ipcRenderer.invoke("dialog:select-directory", request),
  analyzer: {
    execute: (request: AnalyzerExecuteRequest) =>
      ipcRenderer.invoke("analyzer:execute", request),
    cancel: (requestId: string) => ipcRenderer.invoke("analyzer:cancel", requestId),
    analyzeAccessLog: (request: AnalyzeAccessLogRequest) =>
      ipcRenderer.invoke("analyzer:execute", {
        type: "access_log",
        requestId: request.requestId,
        params: request,
      }),
    analyzeCollapsedProfile: (request: AnalyzeCollapsedProfileRequest) =>
      ipcRenderer.invoke("analyzer:execute", {
        type: "profiler_collapsed",
        requestId: request.requestId,
        params: request,
      }),
  },
  exporter: {
    execute: (request: ExportExecuteRequest) =>
      ipcRenderer.invoke("export:execute", request),
  },
  demo: {
    list: (manifestRoot?: string) => ipcRenderer.invoke("demo:list", manifestRoot),
    run: (request: DemoRunRequest) => ipcRenderer.invoke("demo:run", request),
    openPath: (targetPath: string) => ipcRenderer.invoke("file:open", targetPath),
  },
});
