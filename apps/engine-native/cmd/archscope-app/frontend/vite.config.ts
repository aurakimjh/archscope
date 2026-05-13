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
import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import wails from "@wailsio/runtime/plugins/vite";

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

function readPackageVersion(): string {
  try {
    const pkg = JSON.parse(readFileSync(path.resolve(__dirname, "package.json"), "utf8"));
    return typeof pkg.version === "string" && pkg.version.trim() ? pkg.version.trim() : "0.3.1";
  } catch {
    return "0.3.1";
  }
}

function readGitBuild(): string {
  try {
    return execSync("git describe --tags --always --dirty", {
      cwd: __dirname,
      stdio: ["ignore", "pipe", "ignore"],
    })
      .toString()
      .trim();
  } catch {
    return `v${readPackageVersion()}`;
  }
}

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss(), wails("./bindings")],
  define: {
    __APP_VERSION__: JSON.stringify(process.env.VITE_APP_VERSION || readPackageVersion()),
    __APP_BUILD__: JSON.stringify(process.env.VITE_APP_BUILD || readGitBuild()),
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "src"),
    },
  },
  build: {
    // Page-level lazy loading keeps the startup shell small. The largest
    // remaining chunk is the shared, lazy ECharts runtime used only after a
    // chart page is opened, so keep the release budget explicit instead of
    // accepting Vite's generic 500 KB default.
    chunkSizeWarningLimit: 700,
  },
});
