import type { PageKey } from "../App";
import { useI18n, type MessageKey } from "../i18n/I18nProvider";

type NavItem = {
  key: PageKey;
  labelKey: MessageKey;
};

const navItems: NavItem[] = [
  { key: "dashboard", labelKey: "dashboard" },
  { key: "access-log", labelKey: "accessLogAnalyzer" },
  { key: "gc-log", labelKey: "gcLogAnalyzer" },
  { key: "profiler", labelKey: "profilerAnalyzer" },
  { key: "thread-dump", labelKey: "threadDumpAnalyzer" },
  { key: "exception", labelKey: "exceptionAnalyzer" },
  { key: "chart-studio", labelKey: "chartStudio" },
  { key: "export-center", labelKey: "exportCenter" },
  { key: "settings", labelKey: "settings" },
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
            className={item.key === activePage ? "nav-item active" : "nav-item"}
            onClick={() => onNavigate(item.key)}
          >
            {t(item.labelKey)}
          </button>
        ))}
      </nav>
    </aside>
  );
}
