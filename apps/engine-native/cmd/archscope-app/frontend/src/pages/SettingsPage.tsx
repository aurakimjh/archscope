// ─────────────────────────────────────────────────────────────────────
// [한글] pages/SettingsPage.tsx — 앱 설정 페이지.
//
// 책임/목적: locale(en/ko), theme(light/dark/system), 프로파일러
// 기본값(intervalMs/topN/profileKind), 최근 파일 목록(LRU) 을 표시하고
// 변경/초기화 가능. 모든 설정은 localStorage 에 영속 저장.
//
// 의존성 주의: 본 페이지에서는 EngineService / ProfilerService 호출
// 없음 — 클라이언트 사이드 설정만 다룹니다.
// ─────────────────────────────────────────────────────────────────────
import { useEffect, useState } from "react";

import { HelpedLabel, HelpedTitle } from "../components/HelpTip";
import { getHelpText } from "@/help/helpCatalog";
import { useI18n } from "../i18n/I18nProvider";
import { localeLabels, locales, type Locale } from "../i18n/messages";
import { useTheme, type Theme } from "../theme/ThemeProvider";
import { useRecentFiles } from "../hooks/useRecentFiles";
import {
  loadDefaults,
  saveDefaults,
  type ProfilerDefaults,
} from "../state/defaults";

const PROFILE_KINDS = ["wall", "cpu", "lock"] as const;

export function SettingsPage() {
  const { t, setLocale, locale } = useI18n();
  const { theme, setTheme } = useTheme();
  const { entries, clear } = useRecentFiles();
  const [defaults, setDefaults] = useState<ProfilerDefaults>(loadDefaults);
  const [saved, setSaved] = useState(false);

  useEffect(() => {
    if (!saved) return;
    const id = window.setTimeout(() => setSaved(false), 1500);
    return () => window.clearTimeout(id);
  }, [saved]);

  const handleSave = () => {
    saveDefaults(defaults);
    setSaved(true);
  };

  return (
    <main className="content settings-content">
      <section className="card">
        <h2>
          <HelpedTitle help={getHelpText(locale, "pageSettings")}>
            {t("settingsTitle")}
          </HelpedTitle>
        </h2>
        <div className="settings-grid">
          <div className="settings-row">
            <HelpedLabel help={getHelpText(locale, "localeControl")}>
              {t("settingsLanguage")}
            </HelpedLabel>
            <div className="locale-switch">
              {locales.map((value) => (
                <button
                  key={value}
                  type="button"
                  className={value === locale ? "locale-pill active" : "locale-pill"}
                  onClick={() => setLocale(value as Locale)}
                  aria-pressed={value === locale}
                >
                  {localeLabels[value]}
                </button>
              ))}
            </div>
          </div>
          <div className="settings-row">
            <HelpedLabel help={getHelpText(locale, "themeControl")}>
              {t("settingsTheme")}
            </HelpedLabel>
            <div className="locale-switch">
              {(["light", "dark", "system"] as Theme[]).map((value) => (
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
          </div>
          <div className="settings-row">
            <HelpedLabel help={getHelpText(locale, "optionInterval")}>
              {t("settingsDefaultInterval")}
            </HelpedLabel>
            <input
              type="number"
              min={1}
              value={defaults.intervalMs}
              onChange={(e) =>
                setDefaults({ ...defaults, intervalMs: Number(e.target.value) || 100 })
              }
            />
          </div>
          <div className="settings-row">
            <HelpedLabel help={getHelpText(locale, "optionTopN")}>
              {t("settingsDefaultTopN")}
            </HelpedLabel>
            <input
              type="number"
              min={1}
              max={100}
              value={defaults.topN}
              onChange={(e) =>
                setDefaults({ ...defaults, topN: Number(e.target.value) || 20 })
              }
            />
          </div>
          <div className="settings-row">
            <HelpedLabel help={getHelpText(locale, "optionProfileKind")}>
              {t("settingsDefaultProfileKind")}
            </HelpedLabel>
            <select
              value={defaults.profileKind}
              onChange={(e) =>
                setDefaults({
                  ...defaults,
                  profileKind: e.target.value as ProfilerDefaults["profileKind"],
                })
              }
            >
              {PROFILE_KINDS.map((kind) => (
                <option key={kind} value={kind}>
                  {kind}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="settings-actions">
          <button type="button" className="primary" onClick={handleSave}>
            {saved ? t("saved") : t("save")}
          </button>
        </div>
      </section>

      <section className="card">
        <h2>
          <HelpedTitle help={getHelpText(locale, "settingsRecent")}>
            {t("settingsRecentFiles")}
          </HelpedTitle>
        </h2>
        {entries.length === 0 ? (
          <p className="muted">{t("recentFilesNone")}</p>
        ) : (
          <>
            <ul className="recent-list">
              {entries.map((entry) => (
                <li key={entry.path} title={entry.path}>
                  <code>{entry.path}</code>
                </li>
              ))}
            </ul>
            <button type="button" className="ghost" onClick={clear}>
              {t("settingsResetRecent")}
            </button>
          </>
        )}
      </section>

      <section className="card">
        <h2>
          <HelpedTitle help={getHelpText(locale, "settingsAbout")}>
            {t("settingsAbout")}
          </HelpedTitle>
        </h2>
        <p className="muted">{t("settingsAboutBody")}</p>
      </section>
    </main>
  );
}
