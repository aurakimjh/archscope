// ─────────────────────────────────────────────────────────────────────
// [한글] Sidebar.tsx — 좌측 분석기 네비게이션 사이드바.
//
// 책임/목적: 분석기 메뉴를 "분석 도구" 그룹으로 묶고 settings 는
// 상위 메뉴로 유지하는 좌측 네비게이션. collapsed(접힘) 여부와 분석
// 도구 그룹 펼침 여부는 localStorage 에 저장해 다음 실행에도 유지됩니다.
// ─────────────────────────────────────────────────────────────────────
import {
  Activity,
  AlertTriangle,
  BarChart3,
  ChartNoAxesCombined,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Clock3,
  Cpu,
  Download,
  FileSearch,
  Gauge,
  GitCompareArrows,
  Info,
  Network,
  PanelsLeftBottom,
  Route,
  Settings,
  Sparkles,
  ClipboardList,
} from "lucide-react";
import { useEffect, useState } from "react";
import type { ComponentType } from "react";

import { APP_VERSION } from "../appInfo";
import { useI18n } from "../i18n/I18nProvider";
import { AboutDialog } from "./AboutDialog";

const STORAGE_KEY = "archscope.profiler.sidebar.collapsed";
const TOOLS_STORAGE_KEY = "archscope.profiler.sidebar.tools.expanded";
const WORKSPACE_STORAGE_KEY = "archscope.profiler.sidebar.workspace.expanded";

export type NavKey =
  | "profiler"
  | "diff"
  | "access_log"
  | "gc_log"
  | "jfr"
  | "exception"
  | "thread_dump"
  | "msa_profile"
  | "trace_import"
  | "analysis_workspace"
  | "export_center"
  | "report_diff"
  | "chart_studio"
  | "incident_timeline"
  | "slo_golden_signals"
  | "service_flow"
  | "evidence_board"
  | "settings";

type NavIcon = ComponentType<{ className?: string }>;

const NAV_ICONS: Record<NavKey, NavIcon> = {
  profiler: Activity,
  diff: GitCompareArrows,
  access_log: FileSearch,
  gc_log: ChartNoAxesCombined,
  jfr: Cpu,
  exception: AlertTriangle,
  thread_dump: PanelsLeftBottom,
  msa_profile: Network,
  trace_import: Route,
  analysis_workspace: ClipboardList,
  export_center: Download,
  report_diff: GitCompareArrows,
  chart_studio: BarChart3,
  incident_timeline: Clock3,
  slo_golden_signals: Gauge,
  service_flow: Network,
  evidence_board: ClipboardList,
  settings: Settings,
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
  const [toolsExpanded, setToolsExpanded] = useState<boolean>(() => {
    try {
      return window.localStorage.getItem(TOOLS_STORAGE_KEY) !== "0";
    } catch {
      return true;
    }
  });
  const [workspaceExpanded, setWorkspaceExpanded] = useState<boolean>(() => {
    try {
      return window.localStorage.getItem(WORKSPACE_STORAGE_KEY) !== "0";
    } catch {
      return true;
    }
  });
  const [aboutOpen, setAboutOpen] = useState(false);

  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, collapsed ? "1" : "0");
    } catch {
      // ignore
    }
  }, [collapsed]);

  useEffect(() => {
    try {
      window.localStorage.setItem(TOOLS_STORAGE_KEY, toolsExpanded ? "1" : "0");
    } catch {
      // ignore
    }
  }, [toolsExpanded]);

  useEffect(() => {
    try {
      window.localStorage.setItem(
        WORKSPACE_STORAGE_KEY,
        workspaceExpanded ? "1" : "0",
      );
    } catch {
      // ignore
    }
  }, [workspaceExpanded]);

  const analysisItems: { key: NavKey; label: string }[] = [
    { key: "profiler", label: t("navProfiler") },
    { key: "diff", label: t("navDiff") },
    { key: "access_log", label: t("navAccessLog") },
    { key: "gc_log", label: t("navGcLog") },
    { key: "jfr", label: t("navJfr") },
    { key: "exception", label: t("navException") },
    { key: "thread_dump", label: t("navThreadDump") },
    { key: "msa_profile", label: t("navMsaProfile") },
    { key: "trace_import", label: t("navTraceImport") },
  ];
  const workspaceItems: { key: NavKey; label: string }[] = [
    { key: "analysis_workspace", label: t("navAnalysisWorkspace") },
    { key: "export_center", label: t("navExportCenter") },
    { key: "report_diff", label: t("navReportDiff") },
    { key: "chart_studio", label: t("navChartStudio") },
    { key: "incident_timeline", label: t("navIncidentTimeline") },
    { key: "slo_golden_signals", label: t("navSloGoldenSignals") },
    { key: "service_flow", label: t("navServiceFlow") },
    { key: "evidence_board", label: t("navEvidenceBoard") },
  ];
  const topLevelItems: { key: NavKey; label: string }[] = [
    { key: "settings", label: t("navSettings") },
  ];

  return (
    <aside className={collapsed ? "sidebar collapsed" : "sidebar"} aria-label={t("nav")}>
      <div className="sidebar-brand">
        <div className="sidebar-mark" aria-hidden="true">
          <Sparkles className="sidebar-mark-icon" />
        </div>
        {!collapsed && (
          <div className="sidebar-brand-text">
            <span className="brand-name">{t("appTitle")}</span>
            <span className="brand-subtitle">{t("eyebrow")}</span>
          </div>
        )}
      </div>
      <nav className="sidebar-nav">
        <button
          type="button"
          className="nav-group-toggle"
          onClick={() => setToolsExpanded((value) => !value)}
          aria-expanded={toolsExpanded}
          title={collapsed ? t("navAnalysisTools") : undefined}
        >
          <span className="nav-icon nav-group-icon" aria-hidden="true">
            <Sparkles className="nav-lucide" />
          </span>
          {!collapsed && <span className="nav-label">{t("navAnalysisTools")}</span>}
          {!collapsed && (
            <span className="nav-group-chevron" aria-hidden="true">
              {toolsExpanded ? (
                <ChevronDown className="nav-lucide-sm" />
              ) : (
                <ChevronRight className="nav-lucide-sm" />
              )}
            </span>
          )}
        </button>
        {toolsExpanded && (
          <NavGroupItems
            active={active}
            collapsed={collapsed}
            items={analysisItems}
            onNavigate={onNavigate}
          />
        )}
        <button
          type="button"
          className="nav-group-toggle"
          onClick={() => setWorkspaceExpanded((value) => !value)}
          aria-expanded={workspaceExpanded}
          title={collapsed ? t("navWorkspaceTools") : undefined}
        >
          <span className="nav-icon nav-group-icon" aria-hidden="true">
            <ClipboardList className="nav-lucide" />
          </span>
          {!collapsed && <span className="nav-label">{t("navWorkspaceTools")}</span>}
          {!collapsed && (
            <span className="nav-group-chevron" aria-hidden="true">
              {workspaceExpanded ? (
                <ChevronDown className="nav-lucide-sm" />
              ) : (
                <ChevronRight className="nav-lucide-sm" />
              )}
            </span>
          )}
        </button>
        {workspaceExpanded && (
          <NavGroupItems
            active={active}
            collapsed={collapsed}
            items={workspaceItems}
            onNavigate={onNavigate}
          />
        )}
        <div className="nav-separator" />
        {topLevelItems.map((item) => {
          const Icon = NAV_ICONS[item.key];
          return (
            <button
              key={item.key}
              type="button"
              className={item.key === active ? "nav-item active" : "nav-item"}
              onClick={() => onNavigate(item.key)}
              aria-current={item.key === active ? "page" : undefined}
              title={collapsed ? item.label : undefined}
            >
              <span className="nav-icon" aria-hidden="true">
                <Icon className="nav-lucide" />
              </span>
              {!collapsed && <span className="nav-label">{item.label}</span>}
            </button>
          );
        })}
      </nav>
      <div className="sidebar-footer">
        <button
          type="button"
          className="sidebar-version-button"
          onClick={() => setAboutOpen(true)}
          title={collapsed ? t("aboutOpen") : undefined}
          aria-label={t("aboutOpen")}
        >
          <span className="nav-icon" aria-hidden="true">
            <Info className="nav-lucide" />
          </span>
          {!collapsed && <span className="sidebar-version-text">v{APP_VERSION}</span>}
        </button>
      </div>
      <button
        type="button"
        className="sidebar-toggle"
        onClick={() => setCollapsed((value) => !value)}
        aria-label={collapsed ? t("sidebarExpand") : t("sidebarCollapse")}
        title={collapsed ? t("sidebarExpand") : t("sidebarCollapse")}
      >
        {collapsed ? (
          <ChevronRight className="nav-lucide-sm" />
        ) : (
          <ChevronLeft className="nav-lucide-sm" />
        )}
      </button>
      <AboutDialog open={aboutOpen} onClose={() => setAboutOpen(false)} />
    </aside>
  );
}

function NavGroupItems({
  active,
  collapsed,
  items,
  onNavigate,
}: {
  active: NavKey;
  collapsed: boolean;
  items: { key: NavKey; label: string }[];
  onNavigate: (key: NavKey) => void;
}): JSX.Element {
  return (
    <div className="nav-group-items">
      {items.map((item) => {
        const Icon = NAV_ICONS[item.key];
        return (
          <button
            key={item.key}
            type="button"
            className={item.key === active ? "nav-item active" : "nav-item"}
            onClick={() => onNavigate(item.key)}
            aria-current={item.key === active ? "page" : undefined}
            title={collapsed ? item.label : undefined}
          >
            <span className="nav-icon" aria-hidden="true">
              <Icon className="nav-lucide" />
            </span>
            {!collapsed && <span className="nav-label">{item.label}</span>}
          </button>
        );
      })}
    </div>
  );
}
