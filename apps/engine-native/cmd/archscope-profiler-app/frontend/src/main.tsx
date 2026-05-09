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
import App from "./App";
import { I18nProvider } from "./i18n/I18nProvider";
import { ThemeProvider } from "./theme/ThemeProvider";
import "./styles/tailwind.css";

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ThemeProvider>
      <I18nProvider>
        <App />
      </I18nProvider>
    </ThemeProvider>
  </React.StrictMode>,
);
