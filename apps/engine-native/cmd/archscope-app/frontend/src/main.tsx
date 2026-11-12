// ─────────────────────────────────────────────────────────────────────
// [한글] main.tsx — Wails v3 데스크톱 셸의 React 진입점.
//
// 책임/목적:
//   - createRoot 로 #root DOM 에 React 트리를 마운트.
//   - 전역 Provider 순서: ThemeProvider → I18nProvider → App.
//     테마(라이트/다크/시스템) 상태가 i18n 보다 바깥에 있는 이유는,
//     초기 페인트 시 system color-scheme 적용이 더 빠르게 적용되어야
//     플래시(깜빡임)가 적기 때문입니다.
//   - StrictMode 는 개발 시 effect 이중 호출로 잘못된 부수효과를
//     조기에 잡기 위해 활성화 — Wails 런타임 콜은 idempotent 해야
//     하므로 EngineService 호출도 두 번 호출되어도 안전해야 합니다.
//
// 데이터 흐름: 여기서는 Wails IPC 와 직접 상호작용하지 않습니다.
// 실제 `window.go.engineservice.*` (또는 Call.ByName) 호출은
// bridge/engine.ts 와 각 페이지에서 발생합니다.
//
// 의존성 주의: tailwind.css 는 반드시 이 진입점에서 import 해야
// build 결과물이 단일 CSS bundle 로 묶여 Wails 정적 리소스에 포함됩니다.
// ─────────────────────────────────────────────────────────────────────
import React from "react";
import ReactDOM from "react-dom/client";
import { hasNativeDesktopBridge } from "./state/desktopRuntime";
import { I18nProvider } from "./i18n/I18nProvider";
import { ThemeProvider } from "./theme/ThemeProvider";
import "./styles/tailwind.css";

const root = document.getElementById("root") as HTMLElement;
if (!hasNativeDesktopBridge(window)) {
  root.textContent = navigator.language.startsWith("ko")
    ? "ArchScope 데스크톱 앱을 실행하세요. 브라우저 미리보기는 지원하지 않습니다."
    : "Open the ArchScope desktop app. Browser preview is not supported.";
} else if (new URLSearchParams(location.search).has("visual-render")) {
  const token = new URLSearchParams(location.search).get("visual-render") || "";
  void import("./visualRenderSurface").then(module => module.runVisualRenderSurface(token));
} else {
  document.addEventListener("securitypolicyviolation", () => {
    void import("./bridge/engine").then(bridge => bridge.recordOfflineRendererViolation()).catch(() => {});
  });
  // 브리지 확인 전에 App을 import하면 Wails 호출과 페이지 초기화가 먼저 실행된다.
  void import("./bridge/engine").then(async bridge => {
    const status = await bridge.getOfflineStatus();
    if (status.offline) {
      const deny = (): void => { void bridge.recordOfflineRendererViolation().catch(() => {}); };
      document.addEventListener("click", event => {
        const anchor = (event.target as Element | null)?.closest?.("a");
        if (anchor && new URL(anchor.href, location.href).origin !== location.origin) { event.preventDefault(); event.stopImmediatePropagation(); deny(); }
      }, true);
      const originalOpen = window.open.bind(window);
      window.open = ((url?: string | URL, target?: string, features?: string) => {
        if (url && new URL(String(url), location.href).origin !== location.origin) { deny(); return null; }
        return originalOpen(url, target, features);
      }) as typeof window.open;
    }
    return import("./App");
  }).then(({ default: App }) => {
    ReactDOM.createRoot(root).render(
      <React.StrictMode>
        <ThemeProvider>
          <I18nProvider>
            <App />
          </I18nProvider>
        </ThemeProvider>
      </React.StrictMode>,
    );
  });
}
