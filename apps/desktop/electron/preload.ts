import { contextBridge, ipcRenderer } from "electron";

import type {
  AnalyzeAccessLogRequest,
  AnalyzeCollapsedProfileRequest,
} from "../src/api/analyzerContract.js";

contextBridge.exposeInMainWorld("archscope", {
  platform: process.platform,
  analyzer: {
    analyzeAccessLog: (request: AnalyzeAccessLogRequest) =>
      ipcRenderer.invoke("analyzer:access-log:analyze", request),
    analyzeCollapsedProfile: (request: AnalyzeCollapsedProfileRequest) =>
      ipcRenderer.invoke("analyzer:profiler:analyze-collapsed", request),
  },
});
