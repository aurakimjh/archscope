import React from "react";
import ReactDOM from "react-dom/client";

import { App } from "./App";
import { registerArchScopeTheme } from "./charts/echartsTheme";
import { I18nProvider } from "./i18n/I18nProvider";
import "./styles/global.css";

registerArchScopeTheme();

ReactDOM.createRoot(document.getElementById("root") as HTMLElement).render(
  <React.StrictMode>
    <I18nProvider>
      <App />
    </I18nProvider>
  </React.StrictMode>,
);
