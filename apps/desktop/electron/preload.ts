import { contextBridge } from "electron";

contextBridge.exposeInMainWorld("archscope", {
  platform: process.platform,
});
