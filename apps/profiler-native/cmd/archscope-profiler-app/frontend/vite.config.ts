// ─────────────────────────────────────────────────────────────────────
// [한글] vite.config.ts — Wails 데스크톱 셸의 Vite 빌드 설정.
//
// 책임/목적:
//   - react / tailwindcss / wails 3 개 플러그인 활성화.
//   - wails("./bindings") 플러그인은 Wails CLI 가 생성한 Go bindings
//     모듈(`./bindings/...`) 을 ESM 으로 번들에 포함시킵니다 — 이
//     경로가 없으면 ProfilerService 등의 import 가 실패합니다.
//   - alias "@" → src/ 로 두어 페이지/컴포넌트가 절대-스타일 import
//     (예: "@/bridge/engine") 를 사용 가능.
//
// 의존성 주의: tailwindcss v4 vite plugin 사용 (기존 PostCSS 설정 불필요).
// ─────────────────────────────────────────────────────────────────────
import path from "node:path";
import { fileURLToPath } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), wails("./bindings")],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
});
