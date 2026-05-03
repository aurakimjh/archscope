import type { PageKey } from "../App";
import { useI18n, type MessageKey } from "../i18n/I18nProvider";

type NavItem = {
  key: PageKey;
  labelKey: MessageKey;
  icon: string;
};

const navItems: NavItem[] = [
  { key: "dashboard", labelKey: "dashboard", icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" },
  { key: "access-log", labelKey: "accessLogAnalyzer", icon: "M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" },
  { key: "gc-log", labelKey: "gcLogAnalyzer", icon: "M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" },
  { key: "profiler", labelKey: "profilerAnalyzer", icon: "M13 10V3L4 14h7v7l9-11h-7z" },
  { key: "thread-dump", labelKey: "threadDumpAnalyzer", icon: "M4 6h16M4 12h16M4 18h7" },
  { key: "exception", labelKey: "exceptionAnalyzer", icon: "M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" },
  { key: "jfr", labelKey: "jfrAnalyzer", icon: "M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3" },
  { key: "chart-studio", labelKey: "chartStudio", icon: "M11 3.055A9.001 9.001 0 1020.945 13H11V3.055z M20.488 9H15V3.512A9.025 9.025 0 0120.488 9z" },
  { key: "demo-data", labelKey: "demoDataCenter", icon: "M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4" },
  { key: "export-center", labelKey: "exportCenter", icon: "M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" },
  { key: "settings", labelKey: "settings", icon: "M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z M15 12a3 3 0 11-6 0 3 3 0 016 0z" },
];

type SidebarProps = {
  activePage: PageKey;
  onNavigate: (page: PageKey) => void;
};

export function Sidebar({ activePage, onNavigate }: SidebarProps): JSX.Element {
  const { t } = useI18n();

  return (
    <aside className="sidebar">
      <div className="brand">
        <div className="brand-mark">AS</div>
        <div>
          <strong>ArchScope</strong>
          <span>{t("brandSubtitle")}</span>
        </div>
      </div>
      <nav className="nav-list" aria-label={t("navLabel")}>
        {navItems.map((item) => (
          <button
            key={item.key}
            type="button"
            data-testid={`nav-${item.key}`}
            className={item.key === activePage ? "nav-item active" : "nav-item"}
            onClick={() => onNavigate(item.key)}
          >
            <span className="nav-icon">
              <svg viewBox="0 0 24 24">
                <path d={item.icon} strokeLinecap="round" strokeLinejoin="round" />
              </svg>
            </span>
            {t(item.labelKey)}
          </button>
        ))}
      </nav>
    </aside>
  );
}
