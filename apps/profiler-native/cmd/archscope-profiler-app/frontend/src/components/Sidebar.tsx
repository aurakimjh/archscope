import { useEffect, useState } from "react";

import { useI18n } from "../i18n/I18nProvider";

const STORAGE_KEY = "archscope.profiler.sidebar.collapsed";

export type NavKey =
  | "profiler"
  | "diff"
  | "access_log"
  | "gc_log"
  | "jfr"
  | "exception"
  | "settings";

const NAV_ICONS: Record<NavKey, string> = {
  profiler: "🔥",
  diff: "Δ",
  access_log: "🌐",
  gc_log: "♻",
  jfr: "🎙",
  exception: "⚠",
  settings: "⚙",
};

export type SidebarProps = {
  active: NavKey;
  onNavigate: (key: NavKey) => void;
};

export function Sidebar({ active, onNavigate }: SidebarProps) {
  const { t } = useI18n();
  const [collapsed, setCollapsed] = useState<boolean>(() => {
    try {
      return window.localStorage.getItem(STORAGE_KEY) === "1";
    } catch {
      return false;
    }
  });

  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, collapsed ? "1" : "0");
    } catch {
      // ignore
    }
  }, [collapsed]);

  const items: { key: NavKey; label: string }[] = [
    { key: "profiler", label: t("navProfiler") },
    { key: "diff", label: t("navDiff") },
    { key: "access_log", label: t("navAccessLog") },
    { key: "gc_log", label: t("navGcLog") },
    { key: "jfr", label: t("navJfr") },
    { key: "exception", label: t("navException") },
    { key: "settings", label: t("navSettings") },
  ];

  return (
    <aside className={collapsed ? "sidebar collapsed" : "sidebar"} aria-label={t("nav")}>
      <div className="sidebar-brand">
        <div className="sidebar-mark" aria-hidden="true">A</div>
        {!collapsed && (
          <div className="sidebar-brand-text">
            <span className="eyebrow">{t("eyebrow")}</span>
            <span className="brand-name">ArchScope</span>
          </div>
        )}
      </div>
      <nav className="sidebar-nav">
        {items.map((item) => (
          <button
            key={item.key}
            type="button"
            className={item.key === active ? "nav-item active" : "nav-item"}
            onClick={() => onNavigate(item.key)}
            aria-current={item.key === active ? "page" : undefined}
            title={collapsed ? item.label : undefined}
          >
            <span className="nav-icon" aria-hidden="true">
              {NAV_ICONS[item.key]}
            </span>
            {!collapsed && <span className="nav-label">{item.label}</span>}
          </button>
        ))}
      </nav>
      <button
        type="button"
        className="sidebar-toggle"
        onClick={() => setCollapsed((value) => !value)}
        aria-label={collapsed ? t("sidebarExpand") : t("sidebarCollapse")}
        title={collapsed ? t("sidebarExpand") : t("sidebarCollapse")}
      >
        {collapsed ? "›" : "‹"}
      </button>
    </aside>
  );
}
