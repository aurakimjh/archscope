// ─────────────────────────────────────────────────────────────────────
// [한글] playwright.config.ts — E2E 테스트 설정.
//   testDir: "./e2e" 디렉토리에서 *.spec.ts 자동 발견.
//   fullyParallel + workers=1 — 데스크톱/엔진 단일 인스턴스 가정,
//   포트 충돌과 race 를 피하려고 직렬 실행.
// ─────────────────────────────────────────────────────────────────────
import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  timeout: 60_000,
  fullyParallel: false,
  workers: 1,
});
