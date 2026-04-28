import { contextBridge, ipcRenderer } from "electron";

import type {
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
  SelectFileRequest,
} from "../src/api/analyzerContract.js";

contextBridge.exposeInMainWorld("archscope", {
  platform: process.platform,
  selectFile: (request?: SelectFileRequest) =>
    ipcRenderer.invoke("dialog:select-file", request),
  analyzer: {
    analyzeAccessLog: (request: AnalyzeAccessLogRequest) =>
      ipcRenderer.invoke("analyzer:access-log:analyze", request),
    analyzeCollapsedProfile: (request: AnalyzeCollapsedProfileRequest) =>
      ipcRenderer.invoke("analyzer:profiler:analyze-collapsed", request),
  },
});
