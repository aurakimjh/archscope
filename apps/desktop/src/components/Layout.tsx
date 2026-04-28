import type { PageKey } from "../App";
import type { ReactNode } from "react";
import {
  localeLabels,
  locales,
  useI18n,
  type Locale,
} from "../i18n/I18nProvider";
import { Sidebar } from "./Sidebar";

type LayoutProps = {
  activePage: PageKey;
  onNavigate: (page: PageKey) => void;
  children: ReactNode;
};

export function Layout({
  activePage,
  onNavigate,
  children,
}: LayoutProps): JSX.Element {
  const { locale, setLocale, t } = useI18n();

  return (
    <div className="app-shell">
      <Sidebar activePage={activePage} onNavigate={onNavigate} />
      <main className="main-panel">
        <header className="topbar">
          <div>
            <p className="eyebrow">{t("appTagline")}</p>
            <h1>ArchScope</h1>
          </div>
          <label className="locale-switcher">
            <span>{t("language")}</span>
            <select
              value={locale}
              onChange={(event) => setLocale(event.target.value as Locale)}
            >
              {locales.map((item) => (
                <option key={item} value={item}>
                  {localeLabels[item]}
                </option>
              ))}
            </select>
          </label>
          <div className="pipeline">
            <span>{t("pipelineRawData")}</span>
            <span>{t("pipelineParsing")}</span>
            <span>{t("pipelineAnalysis")}</span>
            <span>{t("pipelineVisualization")}</span>
            <span>{t("pipelineExport")}</span>
          </div>
        </header>
        {children}
      </main>
    </div>
  );
}
