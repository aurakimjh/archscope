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
