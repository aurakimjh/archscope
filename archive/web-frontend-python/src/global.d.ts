// ─────────────────────────────────────────────────────────────────────
// [한글] global.d.ts — Electron preload 가 노출하는 window.archscope
//   객체의 타입을 React/TypeScript 가 인식하도록 전역 선언.
//
//   - 데스크톱(Electron) 모드: preload.js 가 contextBridge 로
//     window.archscope = { http, files, ... } 를 주입.
//   - 브라우저(개발) 모드: window.archscope 가 undefined → 모든 API
//     호출은 fetch 로 fallback(httpBridge.ts 가 이를 추상화).
//   - export {} 로 모듈 스코프를 강제 — 그렇지 않으면 declare global
//     이 ambient script 모드로 동작해 의도치 않은 머지가 발생합니다.
// ─────────────────────────────────────────────────────────────────────
import type { ArchScopeRendererApi } from "./api/analyzerContract";

declare global {
  interface Window {
    archscope?: ArchScopeRendererApi;
  }
}

export {};
