import type { ArchScopeRendererApi } from "./api/analyzerContract";

declare global {
  interface Window {
    archscope?: ArchScopeRendererApi;
  }
}

export {};
