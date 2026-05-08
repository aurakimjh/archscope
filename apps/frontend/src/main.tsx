// ─────────────────────────────────────────────────────────────────────
// [한글] main.tsx — 프론트엔드 진입점(Vite 가 index.html 에서 모듈 import).
//
// 책임/목적:
//   - React 트리(ThemeProvider → I18nProvider → App)를 #root 에 마운트.
//   - HTTP bridge 설치(installHttpBridge): Electron desktop 모드에서는
//     window.archscope.http 를, 브라우저 모드에서는 fetch 를 그대로 사용.
//   - ECharts 커스텀 테마 등록(registerArchScopeTheme): 사이트 색상/폰트
//     토큰을 ECharts 기본 옵션과 결합.
//   - Ctrl/Cmd + wheel 페이지 줌 차단: d3-zoom 으로 차트를 스케일하는데
//     그 위에 브라우저 줌까지 겹치면 축 라벨이 두 번 확대되어 보이기에.
//
// 데이터 흐름:
//   index.html → main.tsx → installHttpBridge() / registerArchScopeTheme()
//   → ReactDOM.createRoot().render(<App/>).
//
// 의존성:
//   - pretendard 가변 폰트(한글) + Tailwind v4 + global.css.
//   - React.StrictMode 사용. 개발 모드에서 effect 가 두 번 호출될 수 있음.
// ─────────────────────────────────────────────────────────────────────
import React from "react";
import ReactDOM from "react-dom/client";

import { App } from "./App";
import { installHttpBridge } from "./api/httpBridge";
import { registerArchScopeTheme } from "./charts/echartsTheme";
import { ThemeProvider } from "./components/theme-provider";
import { I18nProvider } from "./i18n/I18nProvider";
import "pretendard/dist/web/variable/pretendardvariable.css";
import "./styles/tailwind.css";
import "./styles/global.css";

installHttpBridge();
registerArchScopeTheme();

// Block Ctrl/Cmd + wheel page-zoom that would otherwise rescale the entire
// chart (axis labels included) on top of d3-zoom.
window.addEventListener(
  "wheel",
  (event) => {
    if (event.ctrlKey || event.metaKey) event.preventDefault();
  },
  { passive: false },
);

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <ThemeProvider>
      <I18nProvider>
        <App />
      </I18nProvider>
    </ThemeProvider>
  </React.StrictMode>,
);
