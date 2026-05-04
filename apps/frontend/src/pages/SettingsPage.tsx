import { Check, RotateCcw, Save, Settings2, Sun, Moon, Monitor } from "lucide-react";
import { useCallback, useEffect, useState } from "react";

import type { AppSettings } from "@/api/analyzerContract";
import { useTheme, type Theme } from "@/components/theme-provider";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import {
  localeLabels,
  locales,
  useI18n,
  type Locale,
} from "@/i18n/I18nProvider";
import { cn } from "@/lib/utils";

type SaveStatus = "idle" | "saved" | "error";

export function SettingsPage(): JSX.Element {
  const { t, locale, setLocale } = useI18n();
  const { theme, setTheme } = useTheme();
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
    const settings: AppSettings = { enginePath, chartTheme, locale };
    const response = await window.archscope?.settings?.set(settings);
    setSaveStatus(response?.ok ? "saved" : "error");
    setTimeout(() => setSaveStatus("idle"), 2000);
  }, [enginePath, chartTheme, locale]);

  const handleBrowseEngine = useCallback(async () => {
    const response = await window.archscope?.selectFile?.({
      title: t("enginePath"),
      filters: [{ name: t("allFilesFilter"), extensions: ["*"] }],
    });
    if (response && !response.canceled && response.filePath) {
      setEnginePath(response.filePath);
    }
  }, [t]);

  function handleReset(): void {
    setEnginePath("");
    setChartTheme("light");
    setLocale("en");
  }

  if (!loaded) {
    return (
      <Card className="border-dashed">
        <CardContent className="flex items-center justify-center py-16 text-sm text-muted-foreground">
          Loading...
        </CardContent>
      </Card>
    );
  }

  const themeButtons: Array<{ id: Theme; label: string; icon: JSX.Element }> = [
    { id: "light", label: t("themeLight"), icon: <Sun className="h-3.5 w-3.5" /> },
    { id: "dark", label: t("themeDark"), icon: <Moon className="h-3.5 w-3.5" /> },
    { id: "system", label: t("themeSystem"), icon: <Monitor className="h-3.5 w-3.5" /> },
  ];

  return (
    <div className="flex max-w-3xl flex-col gap-5">
      <div className="flex items-center gap-2">
        <Settings2 className="h-5 w-5 text-muted-foreground" />
        <h1 className="text-xl font-semibold tracking-tight">{t("settings")}</h1>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">{t("enginePath")}</CardTitle>
          <CardDescription>{t("enginePathDescription")}</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-center gap-2">
            <Input
              type="text"
              className="flex-1 min-w-[240px]"
              placeholder={t("bundledEngine")}
              value={enginePath}
              onChange={(event) => setEnginePath(event.target.value)}
            />
            <Button
              type="button"
              variant="secondary"
              size="sm"
              onClick={() => void handleBrowseEngine()}
            >
              {t("browseEngine")}
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">{t("themeLabel")}</CardTitle>
          <CardDescription>
            {t("themeSystem")} / {t("themeLight")} / {t("themeDark")}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="inline-flex rounded-md border border-border bg-muted/30 p-1">
            {themeButtons.map((option) => (
              <button
                key={option.id}
                type="button"
                onClick={() => setTheme(option.id)}
                className={cn(
                  "inline-flex items-center gap-1.5 rounded px-3 py-1 text-xs font-medium transition-colors",
                  theme === option.id
                    ? "bg-background text-foreground shadow-sm"
                    : "text-muted-foreground hover:text-foreground",
                )}
              >
                {option.icon}
                {option.label}
              </button>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">{t("defaultChartTheme")}</CardTitle>
        </CardHeader>
        <CardContent>
          <select
            className="h-9 w-full max-w-xs rounded-md border border-input bg-transparent px-3 text-sm"
            value={chartTheme}
            onChange={(event) => setChartTheme(event.target.value as "light" | "dark")}
          >
            <option value="light">{t("lightTheme")}</option>
            <option value="dark">{t("darkTheme")}</option>
          </select>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle className="text-sm">{t("localeAndReportLanguage")}</CardTitle>
        </CardHeader>
        <CardContent>
          <select
            className="h-9 w-full max-w-xs rounded-md border border-input bg-transparent px-3 text-sm"
            value={locale}
            onChange={(event) => setLocale(event.target.value as Locale)}
          >
            {locales.map((loc) => (
              <option key={loc} value={loc}>
                {localeLabels[loc]}
              </option>
            ))}
          </select>
        </CardContent>
      </Card>

      <div className="flex flex-wrap items-center gap-2">
        <Button type="button" onClick={() => void handleSave()}>
          <Save className="h-3.5 w-3.5" />
          {t("saveSettings")}
        </Button>
        <Button type="button" variant="outline" onClick={handleReset}>
          <RotateCcw className="h-3.5 w-3.5" />
          {t("resetToDefault")}
        </Button>
        {saveStatus === "saved" && (
          <span className="inline-flex items-center gap-1 text-sm text-emerald-600 dark:text-emerald-400">
            <Check className="h-3.5 w-3.5" />
            {t("settingsSaved")}
          </span>
        )}
        {saveStatus === "error" && (
          <span className="text-sm text-destructive">{t("settingsSaveError")}</span>
        )}
      </div>
    </div>
  );
}
