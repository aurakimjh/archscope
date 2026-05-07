import { useState } from "react";

import { useI18n } from "./i18n/I18nProvider";
import { localeLabels, locales } from "./i18n/messages";
import { useTheme, type Theme } from "./theme/ThemeProvider";
import { DropZone } from "./components/DropZone";
import { Sidebar, type NavKey } from "./components/Sidebar";
import { DiffPage } from "./pages/DiffPage";
import { SettingsPage } from "./pages/SettingsPage";
import { AccessLogAnalyzerPage } from "./pages/AccessLogAnalyzerPage";
import { GcLogAnalyzerPage } from "./pages/GcLogAnalyzerPage";
import { JfrAnalyzerPage } from "./pages/JfrAnalyzerPage";
import { ExceptionAnalyzerPage } from "./pages/ExceptionAnalyzerPage";
import { ThreadDumpAnalyzerPage } from "./pages/ThreadDumpAnalyzerPage";
import { ProfilerAnalyzerPage } from "./pages/ProfilerAnalyzerPage";
import { JenniferProfilePage } from "./pages/JenniferProfilePage";

const THEME_OPTIONS: Theme[] = ["light", "dark", "system"];

function App() {
  const { locale, setLocale, t } = useI18n();
  const { theme, setTheme } = useTheme();
  const [active, setActive] = useState<NavKey>("profiler");

  const handleDrop = (path: string) => {
    // Drop forwards into whichever page is active. Each page owns its own
    // file state — the global drop is just an attention-grabber for the
    // primary surface (profiler) where users paste path strings most often.
    if (active === "profiler") {
      // ProfilerAnalyzerPage subscribes to its own Wails dialog; we don't
      // currently broadcast the path through. Future enhancement.
      void path;
    }
  };

  return (
    <DropZone onDrop={handleDrop}>
      <div className="app-shell with-sidebar">
        <Sidebar active={active} onNavigate={setActive} />
        <div className="app-main">
          <header className="topbar">
            <div className="brand">
              <span className="eyebrow">{t("eyebrow")}</span>
              <h1>{t("appTitle")}</h1>
            </div>
            <div className="topbar-controls">
              <div className="locale-switch" role="group" aria-label={t("themeLabel")}>
                {THEME_OPTIONS.map((value) => (
                  <button
                    key={value}
                    type="button"
                    className={value === theme ? "locale-pill active" : "locale-pill"}
                    onClick={() => setTheme(value)}
                    aria-pressed={value === theme}
                  >
                    {value === "light"
                      ? t("themeLight")
                      : value === "dark"
                      ? t("themeDark")
                      : t("themeSystem")}
                  </button>
                ))}
              </div>
              <div className="locale-switch" role="group" aria-label="Locale">
                {locales.map((value) => (
                  <button
                    key={value}
                    type="button"
                    className={value === locale ? "locale-pill active" : "locale-pill"}
                    onClick={() => setLocale(value)}
                    aria-pressed={value === locale}
                    title={localeLabels[value]}
                  >
                    {value.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>
          </header>

          {active === "profiler" && <ProfilerAnalyzerPage />}
          {active === "diff" && <DiffPage />}
          {active === "access_log" && <AccessLogAnalyzerPage />}
          {active === "gc_log" && <GcLogAnalyzerPage />}
          {active === "jfr" && <JfrAnalyzerPage />}
          {active === "exception" && <ExceptionAnalyzerPage />}
          {active === "thread_dump" && <ThreadDumpAnalyzerPage />}
          {active === "msa_profile" && <JenniferProfilePage />}
          {active === "settings" && <SettingsPage />}
        </div>
      </div>
    </DropZone>
  );
}

export default App;
