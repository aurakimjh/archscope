import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";

import type { PageKey } from "@/App";
import { getApiBase } from "@/api/apiBase";
import { AppSidebar } from "@/components/layout/AppSidebar";
import { TopBar } from "@/components/layout/TopBar";
import { useI18n } from "@/i18n/I18nProvider";

const STORAGE_KEY = "archscope.sidebar.collapsed";
const HEALTH_INTERVAL_MS = 8_000;

function readInitialCollapsed(): boolean {
  if (typeof window === "undefined") return false;
  return window.localStorage.getItem(STORAGE_KEY) === "1";
}

type EngineStatus = "checking" | "online" | "offline";

function useEngineHealth(): EngineStatus {
  const [status, setStatus] = useState<EngineStatus>("checking");
  const prevStatus = useRef<EngineStatus>("checking");

  const check = useCallback(async () => {
    try {
      const res = await fetch(`${getApiBase()}/version`, { method: "GET", signal: AbortSignal.timeout(4_000) });
      if (res.ok) {
        setStatus("online");
      } else {
        setStatus("offline");
      }
    } catch {
      setStatus("offline");
    }
  }, []);

  useEffect(() => {
    void check();
    const id = setInterval(() => void check(), HEALTH_INTERVAL_MS);
    return () => clearInterval(id);
  }, [check]);

  useEffect(() => {
    prevStatus.current = status;
  }, [status]);

  return status;
}

type AppShellProps = {
  activePage: PageKey;
  onNavigate: (page: PageKey) => void;
  children: ReactNode;
};

export function AppShell({
  activePage,
  onNavigate,
  children,
}: AppShellProps): JSX.Element {
  const [collapsed, setCollapsed] = useState<boolean>(readInitialCollapsed);
  const engineStatus = useEngineHealth();
  const { t } = useI18n();

  useEffect(() => {
    window.localStorage.setItem(STORAGE_KEY, collapsed ? "1" : "0");
  }, [collapsed]);

  return (
    <div className="flex h-screen w-screen overflow-hidden bg-background text-foreground">
      <AppSidebar
        activePage={activePage}
        onNavigate={onNavigate}
        collapsed={collapsed}
        onToggleCollapsed={() => setCollapsed((prev) => !prev)}
      />
      <div className="flex flex-1 flex-col overflow-hidden">
        <TopBar onNavigate={onNavigate} />
        {engineStatus === "offline" && (
          <div className="flex shrink-0 items-center gap-3 border-b border-destructive/30 bg-destructive/10 px-4 py-2.5 text-sm text-destructive dark:bg-destructive/20">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              viewBox="0 0 20 20"
              fill="currentColor"
              className="h-5 w-5 shrink-0"
            >
              <path
                fillRule="evenodd"
                d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-8-5a.75.75 0 01.75.75v4.5a.75.75 0 01-1.5 0v-4.5A.75.75 0 0110 5zm0 10a1 1 0 100-2 1 1 0 000 2z"
                clipRule="evenodd"
              />
            </svg>
            <div className="min-w-0">
              <p className="font-semibold">{t("engineOfflineTitle")}</p>
              <p className="text-xs opacity-80">
                {t("engineOfflineMessage").replace("{url}", "127.0.0.1:8765")}
              </p>
              <code className="mt-0.5 block text-xs font-mono opacity-70">
                {t("engineOfflineHint")}
              </code>
            </div>
          </div>
        )}
        <main className="flex-1 overflow-y-auto px-7 pb-10 pt-6">{children}</main>
      </div>
    </div>
  );
}
