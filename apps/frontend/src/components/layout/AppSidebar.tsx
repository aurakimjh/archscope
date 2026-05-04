import {
  AlertTriangle,
  BarChart3,
  Database,
  FileText,
  FlameKindling,
  Gauge,
  Layers3,
  PanelLeftClose,
  PanelLeftOpen,
  Settings as SettingsIcon,
  Share2,
  Trash2,
  Upload,
  Zap,
} from "lucide-react";
import type { ComponentType, SVGProps } from "react";

import type { PageKey } from "@/App";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { useI18n, type MessageKey } from "@/i18n/I18nProvider";
import { cn } from "@/lib/utils";

type IconType = ComponentType<SVGProps<SVGSVGElement>>;

type NavItem = {
  key: PageKey;
  labelKey: MessageKey;
  Icon: IconType;
};

type NavSection = {
  labelKey: MessageKey | null;
  items: NavItem[];
};

const sections: NavSection[] = [
  {
    labelKey: null,
    items: [{ key: "dashboard", labelKey: "dashboard", Icon: Gauge }],
  },
  {
    labelKey: "navAnalyzers",
    items: [
      { key: "access-log", labelKey: "accessLogAnalyzer", Icon: FileText },
      { key: "gc-log", labelKey: "gcLogAnalyzer", Icon: Trash2 },
      { key: "profiler", labelKey: "profilerAnalyzer", Icon: Zap },
      { key: "thread-dump", labelKey: "threadDumpAnalyzer", Icon: Layers3 },
      { key: "exception", labelKey: "exceptionAnalyzer", Icon: AlertTriangle },
      { key: "jfr", labelKey: "jfrAnalyzer", Icon: FlameKindling },
    ],
  },
  {
    labelKey: "navWorkspace",
    items: [
      { key: "chart-studio", labelKey: "chartStudio", Icon: BarChart3 },
      { key: "demo-data", labelKey: "demoDataCenter", Icon: Database },
      { key: "export-center", labelKey: "exportCenter", Icon: Upload },
    ],
  },
  {
    labelKey: null,
    items: [{ key: "settings", labelKey: "settings", Icon: SettingsIcon }],
  },
];

type AppSidebarProps = {
  activePage: PageKey;
  onNavigate: (page: PageKey) => void;
  collapsed: boolean;
  onToggleCollapsed: () => void;
};

export function AppSidebar({
  activePage,
  onNavigate,
  collapsed,
  onToggleCollapsed,
}: AppSidebarProps): JSX.Element {
  const { t } = useI18n();

  return (
    <TooltipProvider delayDuration={150}>
      <aside
        data-collapsed={collapsed}
        className={cn(
          "group/sidebar flex h-full shrink-0 flex-col border-r border-sidebar-border bg-sidebar text-sidebar-foreground transition-[width] duration-200",
          collapsed ? "w-14" : "w-60",
        )}
        aria-label={t("navLabel")}
      >
        <div className="flex h-14 items-center justify-between gap-2 border-b border-sidebar-border px-2.5">
          <span
            className={cn(
              "text-xs font-semibold uppercase tracking-wider text-sidebar-foreground/60 transition-opacity",
              collapsed && "pointer-events-none opacity-0",
            )}
          >
            {t("navLabel")}
          </span>
          <Button
            variant="ghost"
            size="icon"
            className="h-7 w-7"
            aria-label={collapsed ? t("sidebarExpand") : t("sidebarCollapse")}
            onClick={onToggleCollapsed}
          >
            {collapsed ? <PanelLeftOpen /> : <PanelLeftClose />}
          </Button>
        </div>

        <nav className="flex-1 overflow-y-auto px-2 py-3">
          {sections.map((section, index) => (
            <div key={index} className={cn("mb-3", index === 0 && "mt-0")}>
              {section.labelKey && !collapsed && (
                <p className="mb-1 px-2 text-[10px] font-semibold uppercase tracking-wider text-sidebar-foreground/50">
                  {t(section.labelKey)}
                </p>
              )}
              {section.labelKey && collapsed && index !== 0 && (
                <Separator className="my-2 bg-sidebar-border" />
              )}
              <ul className="flex flex-col gap-0.5">
                {section.items.map((item) => {
                  const isActive = item.key === activePage;
                  const linkLabel = t(item.labelKey);
                  const button = (
                    <button
                      type="button"
                      data-testid={`nav-${item.key}`}
                      data-active={isActive || undefined}
                      onClick={() => onNavigate(item.key)}
                      className={cn(
                        "group flex w-full items-center gap-3 rounded-md px-2 py-1.5 text-sm transition-colors",
                        isActive
                          ? "bg-sidebar-accent text-sidebar-accent-foreground"
                          : "text-sidebar-foreground/80 hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
                        collapsed && "justify-center px-0",
                      )}
                    >
                      <item.Icon className="h-4 w-4 shrink-0" />
                      <span
                        className={cn(
                          "truncate transition-[opacity,width]",
                          collapsed && "sr-only",
                        )}
                      >
                        {linkLabel}
                      </span>
                    </button>
                  );
                  return (
                    <li key={item.key}>
                      {collapsed ? (
                        <Tooltip>
                          <TooltipTrigger asChild>{button}</TooltipTrigger>
                          <TooltipContent side="right" sideOffset={6}>
                            {linkLabel}
                          </TooltipContent>
                        </Tooltip>
                      ) : (
                        button
                      )}
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
        </nav>

        <div className="border-t border-sidebar-border px-2 py-2">
          <button
            type="button"
            onClick={() => onNavigate("export-center")}
            className={cn(
              "flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-xs text-sidebar-foreground/70 hover:bg-sidebar-accent/60 hover:text-sidebar-accent-foreground",
              collapsed && "justify-center",
            )}
          >
            <Share2 className="h-3.5 w-3.5" />
            <span className={cn("truncate", collapsed && "sr-only")}>
              {t("exportCenter")}
            </span>
          </button>
          <p
            className={cn(
              "mt-2 px-2 text-[10px] text-sidebar-foreground/50",
              collapsed && "hidden",
            )}
          >
            v0.2.0-alpha · web
          </p>
        </div>
      </aside>
    </TooltipProvider>
  );
}
