import { contextBridge, ipcRenderer } from "electron";

import type {
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
  AnalyzerExecuteRequest,
  SelectFileRequest,
} from "../src/api/analyzerContract.js";

contextBridge.exposeInMainWorld("archscope", {
  platform: process.platform,
  selectFile: (request?: SelectFileRequest) =>
    ipcRenderer.invoke("dialog:select-file", request),
  analyzer: {
    execute: (request: AnalyzerExecuteRequest) =>
      ipcRenderer.invoke("analyzer:execute", request),
    analyzeAccessLog: (request: AnalyzeAccessLogRequest) =>
      ipcRenderer.invoke("analyzer:execute", {
        type: "access_log",
        params: request,
      }),
    analyzeCollapsedProfile: (request: AnalyzeCollapsedProfileRequest) =>
      ipcRenderer.invoke("analyzer:execute", {
        type: "profiler_collapsed",
        params: request,
      }),
  },
});
