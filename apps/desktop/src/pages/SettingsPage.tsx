import { useCallback, useEffect, useState } from "react";

import type { AppSettings } from "../api/analyzerContract";
import { useI18n, localeLabels, locales, type Locale } from "../i18n/I18nProvider";

type SaveStatus = "idle" | "saved" | "error";

export function SettingsPage(): JSX.Element {
  const { t, locale, setLocale } = useI18n();
  const [enginePath, setEnginePath] = useState("");
  const [chartTheme, setChartTheme] = useState<"light" | "dark">("light");
  const [saveStatus, setSaveStatus] = useState<SaveStatus>("idle");
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    void loadCurrentSettings();
  }, []);

  async function loadCurrentSettings(): Promise<void> {
    const settings = await window.archscope?.settings?.get();
    if (settings) {
      setEnginePath(settings.enginePath);
      setChartTheme(settings.chartTheme);
    }
    setLoaded(true);
  }

  const handleSave = useCallback(async () => {
    const settings: AppSettings = {
      enginePath,
      chartTheme,
      locale,
    };
    const response = await window.archscope?.settings?.set(settings);
    if (response?.ok) {
      setSaveStatus("saved");
    } else {
      setSaveStatus("error");
    }
    setTimeout(() => setSaveStatus("idle"), 2000);
  }, [enginePath, chartTheme, locale]);

  async function handleBrowseEngine(): Promise<void> {
    const response = await window.archscope?.selectFile?.({
      title: t("enginePath"),
      filters: [{ name: t("allFilesFilter"), extensions: ["*"] }],
    });
    if (response && !response.canceled && response.filePath) {
      setEnginePath(response.filePath);
    }
  }

  function handleReset(): void {
    setEnginePath("");
    setChartTheme("light");
    setLocale("en");
  }

  if (!loaded) {
    return <div className="page"><p>Loading...</p></div>;
  }

  return (
    <div className="page">
      <section className="settings-panel">
        <h2>{t("settings")}</h2>

        <div className="settings-group">
          <label className="settings-label">{t("enginePath")}</label>
          <p className="settings-description">{t("enginePathDescription")}</p>
          <div className="settings-row">
            <input
              type="text"
              className="settings-input"
              value={enginePath}
              placeholder={t("bundledEngine")}
              onChange={(e) => setEnginePath(e.target.value)}
            />
            <button
              type="button"
              className="secondary-button"
              onClick={() => void handleBrowseEngine()}
            >
              {t("browseEngine")}
            </button>
          </div>
        </div>

        <div className="settings-group">
          <label className="settings-label">{t("defaultChartTheme")}</label>
          <div className="settings-row">
            <select
              className="settings-select"
              value={chartTheme}
              onChange={(e) => setChartTheme(e.target.value as "light" | "dark")}
            >
              <option value="light">{t("lightTheme")}</option>
              <option value="dark">{t("darkTheme")}</option>
            </select>
          </div>
        </div>

        <div className="settings-group">
          <label className="settings-label">{t("localeAndReportLanguage")}</label>
          <div className="settings-row">
            <select
              className="settings-select"
              value={locale}
              onChange={(e) => setLocale(e.target.value as Locale)}
            >
              {locales.map((loc) => (
                <option key={loc} value={loc}>
                  {localeLabels[loc]}
                </option>
              ))}
            </select>
          </div>
        </div>

        <div className="settings-actions">
          <button
            type="button"
            className="primary-button"
            onClick={() => void handleSave()}
          >
            {t("saveSettings")}
          </button>
          <button
            type="button"
            className="secondary-button"
            onClick={handleReset}
          >
            {t("resetToDefault")}
          </button>
          {saveStatus === "saved" && (
            <span className="settings-status success">{t("settingsSaved")}</span>
          )}
          {saveStatus === "error" && (
            <span className="settings-status error">{t("settingsSaveError")}</span>
          )}
        </div>
      </section>
    </div>
  );
}
