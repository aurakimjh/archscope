// ─────────────────────────────────────────────────────────────────────
// [한글] vite.config.ts — 개발/빌드 설정. 다음 키 포인트가 있습니다.
//
//   1) base: "./"
//      Electron 패키지에서 file:// 프로토콜로 로드해도 자산 경로가
//      깨지지 않도록 상대 경로로 빌드.
//   2) @tailwindcss/vite
//      Tailwind v4 의 first-party Vite 플러그인 — postcss 단계 없이도
//      JIT 작동.
//   3) resolve.alias["@"] → src
//      모든 import 가 "@/components/..." 같은 절대 경로 alias 사용.
//   4) server.proxy["/api"] → ARCHSCOPE_API_URL(기본 127.0.0.1:8765)
//      개발 모드에서 same-origin /api 호출이 FastAPI 엔진으로 전달.
//   5) test.exclude: e2e 디렉터리(playwright)는 vitest 에서 제외.
// ─────────────────────────────────────────────────────────────────────
import path from "node:path";
import { fileURLToPath } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vitest/config";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

const API_TARGET = process.env.ARCHSCOPE_API_URL ?? "http://127.0.0.1:8765";

export default defineConfig({
  base: "./",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    chunkSizeWarningLimit: 1500,
  },
  test: {
    exclude: ["e2e/**", "node_modules/**", "dist/**"],
  },
  server: {
    host: "127.0.0.1",
    port: 5173,
    proxy: {
      "/api": {
        target: API_TARGET,
        changeOrigin: true,
      },
    },
  },
});
