// ─────────────────────────────────────────────────────────────────────
// [한글] vite-env.d.ts — Vite 클라이언트 타입 참조.
//
// 책임/목적: import.meta.env / 정적 자산 import 등 Vite 가
// 주입하는 전역 타입을 TypeScript 가 인식하도록 reference 선언.
// 이 파일은 build/lint 단계에서만 필요하고, 런타임 코드는 없음.
// ─────────────────────────────────────────────────────────────────────
/// <reference types="vite/client" />

declare const __APP_VERSION__: string;
declare const __APP_BUILD__: string;
