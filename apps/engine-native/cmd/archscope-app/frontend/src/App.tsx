import { Events } from "@wailsio/runtime";
import * as CaptureService from "../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app/captureservice";
// 네이티브 메뉴가 화면 탐색을 담당하며 메뉴를 여닫아도 현재 도구는 유지한다.
import { lazy, Suspense, useState, useEffect } from "react";

import { useI18n } from "./i18n/I18nProvider";

import { useTheme, type Theme } from "./theme/ThemeProvider";
import { useSortableTables } from "./hooks/useSortableTables";
import { DESKTOP_ROUTES, type NavKey } from "./state/desktopNavigation";
import { AboutDialog } from "./components/AboutDialog";
import { SlideOverPanel } from "./components/SlideOverPanel";
import { getHelpText } from "./help/helpCatalog";



const ProfilerAnalyzerPage = lazy(() =>
  import("./pages/ProfilerAnalyzerPage").then(({ ProfilerAnalyzerPage }) => ({
    default: ProfilerAnalyzerPage,
  })),
);
const DiffPage = lazy(() =>
  import("./pages/DiffPage").then(({ DiffPage }) => ({ default: DiffPage })),
);
const AnalysisWorkspacePage = lazy(() =>
  import("./pages/AnalysisWorkspacePage").then(({ AnalysisWorkspacePage }) => ({
    default: AnalysisWorkspacePage,
  })),
);
const ExportCenterPage = lazy(() =>
  import("./pages/ExportCenterPage").then(({ ExportCenterPage }) => ({
    default: ExportCenterPage,
  })),
);
const ReportDiffPage = lazy(() =>
  import("./pages/ReportDiffPage").then(({ ReportDiffPage }) => ({
    default: ReportDiffPage,
  })),
);
const ChartStudioPage = lazy(() =>
  import("./pages/ChartStudioPage").then(({ ChartStudioPage }) => ({
    default: ChartStudioPage,
  })),
);
const IncidentTimelinePage = lazy(() =>
  import("./pages/IncidentTimelinePage").then(({ IncidentTimelinePage }) => ({
    default: IncidentTimelinePage,
  })),
);
const SloGoldenSignalsPage = lazy(() =>
  import("./pages/SloGoldenSignalsPage").then(({ SloGoldenSignalsPage }) => ({
    default: SloGoldenSignalsPage,
  })),
);
const ServiceFlowPage = lazy(() =>
  import("./pages/ServiceFlowPage").then(({ ServiceFlowPage }) => ({
    default: ServiceFlowPage,
  })),
);
const AccessLogAnalyzerPage = lazy(() =>
  import("./pages/AccessLogAnalyzerPage").then(({ AccessLogAnalyzerPage }) => ({
    default: AccessLogAnalyzerPage,
  })),
);
const GcLogAnalyzerPage = lazy(() =>
  import("./pages/GcLogAnalyzerPage").then(({ GcLogAnalyzerPage }) => ({
    default: GcLogAnalyzerPage,
  })),
);
const JfrAnalyzerPage = lazy(() =>
  import("./pages/JfrAnalyzerPage").then(({ JfrAnalyzerPage }) => ({
    default: JfrAnalyzerPage,
  })),
);
const ExceptionAnalyzerPage = lazy(() =>
  import("./pages/ExceptionAnalyzerPage").then(({ ExceptionAnalyzerPage }) => ({
    default: ExceptionAnalyzerPage,
  })),
);
const ThreadDumpAnalyzerPage = lazy(() =>
  import("./pages/ThreadDumpAnalyzerPage").then(
    ({ ThreadDumpAnalyzerPage }) => ({
      default: ThreadDumpAnalyzerPage,
    }),
  ),
);
const JenniferProfilePage = lazy(() =>
  import("./pages/JenniferProfilePage").then(({ JenniferProfilePage }) => ({
    default: JenniferProfilePage,
  })),
);
const TraceImportPage = lazy(() =>
  import("./pages/TraceImportPage").then(({ TraceImportPage }) => ({
    default: TraceImportPage,
  })),
);
const HttpCapturePage = lazy(() =>
  import("./pages/HttpCapturePage").then(({ HttpCapturePage }) => ({
    default: HttpCapturePage,
  })),
);
const BrowserCpuProfilePage = lazy(() =>
  import("./pages/BrowserCpuProfilePage").then(({ BrowserCpuProfilePage }) => ({
    default: BrowserCpuProfilePage,
  })),
);
const LighthouseAuditPage = lazy(() =>
  import("./pages/LighthouseAuditPage").then(({ LighthouseAuditPage }) => ({
    default: LighthouseAuditPage,
  })),
);
const EvidenceBoardPage = lazy(() =>
  import("./pages/EvidenceBoardPage").then(({ EvidenceBoardPage }) => ({
    default: EvidenceBoardPage,
  })),
);
const SettingsPage = lazy(() =>
  import("./pages/SettingsPage").then(({ SettingsPage }) => ({
    default: SettingsPage,
  })),
);

const PAGES = {
  profiler: ProfilerAnalyzerPage,
  diff: DiffPage,
  access_log: AccessLogAnalyzerPage,
  gc_log: GcLogAnalyzerPage,
  jfr: JfrAnalyzerPage,
  exception: ExceptionAnalyzerPage,
  thread_dump: ThreadDumpAnalyzerPage,
  msa_profile: JenniferProfilePage,
  trace_import: TraceImportPage,
  http_capture: HttpCapturePage,
  browser_cpu: BrowserCpuProfilePage,
  browser_audit: LighthouseAuditPage,
  analysis_workspace: AnalysisWorkspacePage,
  export_center: ExportCenterPage,
  report_diff: ReportDiffPage,
  chart_studio: ChartStudioPage,
  incident_timeline: IncidentTimelinePage,
  slo_golden_signals: SloGoldenSignalsPage,
  service_flow: ServiceFlowPage,
  evidence_board: EvidenceBoardPage,
  settings: SettingsPage,
} satisfies Record<NavKey, unknown>;

// 현재 도구만 마운트해 큰 분석 결과 여러 개가 메모리에 남지 않게 한다.
function App() {
  const { locale, setLocale, t } = useI18n();
  const { theme, setTheme } = useTheme();
  const [active, setActive] = useState<NavKey>("profiler");
  const [about, setAbout] = useState(false);
  const [help, setHelp] = useState(false);
  const [canOpen, setCanOpen] = useState(false);
  const [canRecent, setCanRecent] = useState(false);
  const [launchError, setLaunchError] = useState("");
  useEffect(() => {
    let disposed = false;
    void CaptureService.ApplyDesktopLaunch()
      .then((state) => {
        if (disposed) return;
        if (state.showHttp) setActive("http_capture");
        if (state.error) setLaunchError(state.error);
      })
      .catch((caught) => {
        if (!disposed) setLaunchError(String(caught));
      });
    const show = () => setActive("http_capture");
    const off = Events.On("capture:show", show);
    const open = Events.On("capture:open-file", show);
    const errors = Events.On("capture:automation-error", (event) =>
      setLaunchError((event.data as { message?: string })?.message ?? ""),
    );
    return () => {
      disposed = true;
      off();
      open();
      errors();
    };
  }, []);
  const routes = DESKTOP_ROUTES.map(route => {
    let label: string;
    if ("labelKey" in route) label = t(route.labelKey);
    else if (route.key === "http_capture") label = locale === "ko" ? "HTTP 캡처" : "HTTP Capture";
    else label = locale === "ko" ? "브라우저 CPU" : "Browser CPU";
    return {...route, label};
  });
  useEffect(() => {
    const update = () => {
      setCanOpen(!!document.querySelector(`[data-route="${active}"] [data-open-file]:not([disabled])`));
      setCanRecent(!!document.querySelector(`[data-route="${active}"] .recent-files`));
    };
    const observer = new MutationObserver(update);
    observer.observe(document.getElementById("root")!, {
      childList: true, subtree: true, attributes: true, attributeFilter: ["disabled"],
    });
    update();
    return () => observer.disconnect();
  }, [active]);
  useEffect(() => {
    void Events.Emit("desktop:state", {locale, theme, active, canOpen, canRecent, routes});
  }, [locale, theme, active, canOpen, canRecent]);
  useEffect(() => Events.On("desktop:command", event => {
    const command = (event.data as {command?: string})?.command ?? "";
    const [action, value] = command.split(":");
    const modalOpen = document.querySelector(
      '[role="dialog"][aria-modal="true"]:not([aria-hidden="true"])',
    );
    if (["open", "navigate"].includes(action) && modalOpen) return;
    if (action === "navigate" && DESKTOP_ROUTES.some(route => route.key === value)) {
      setActive(value as NavKey);
    } else if (action === "locale" && (value === "en" || value === "ko")) {
      setLocale(value);
    } else if (action === "theme" && ["light", "dark", "system"].includes(value)) {
      setTheme(value as Theme);
    } else if (action === "about") {
      setAbout(true);
    } else if (action === "open") {
      document.querySelector<HTMLButtonElement>(
        `[data-route="${active}"] [data-open-file]:not([disabled])`,
      )?.click();
    } else if (action === "recent") {
      const recent = document.querySelector<HTMLDetailsElement>(`[data-route="${active}"] .recent-files`);
      if (recent) { recent.open = true;recent.scrollIntoView({block:"nearest"}); }
    } else if (action === "help") {
      if (active === "http_capture") {
        void Events.Emit("capture:command", {command: "help"});
      } else {
        const trigger = document.querySelector<HTMLButtonElement>(
          `[data-route="${active}"] [data-help-trigger]`,
        );
        if (trigger) trigger.click();
        else setHelp(true);
      }
    }
  }), [active, setLocale, setTheme]);
  useEffect(() => {
    const navigate = (event: Event) => {
      const key = (event as CustomEvent<NavKey>).detail;
      if (DESKTOP_ROUTES.some(route => route.key===key)) setActive(key);
    };
    window.addEventListener("desktop:navigate",navigate);
    return () => window.removeEventListener("desktop:navigate",navigate);
  }, []);
  useSortableTables();

  const ActivePage = PAGES[active];

  return (
    <>
      <div className="app-shell desktop-shell" data-active-route={active}>
        <div className="app-main">
          <header className="desktop-titlebar"><span>{routes.find(route => route.key===active)?.label}</span></header>
          <Suspense fallback={<PageFallback />}>
            {launchError && (
              <p
                role="alert"
                className="m-3 rounded border border-red-500 p-2 text-sm"
              >
                {launchError}
              </p>
            )}
            <div className="tool-pane" data-route={active} key={active}>
              <ActivePage />
            </div>
          </Suspense>
        </div>
      </div>
      <AboutDialog open={about} onClose={() => setAbout(false)} />
      <SlideOverPanel open={help} onClose={() => setHelp(false)} title={routes.find(route => route.key===active)?.label ?? "ArchScope"}>
        <p>{getHelpText(locale,"fileDock")}</p>
      </SlideOverPanel>
    </>
  );
}

function PageFallback(): React.JSX.Element {
  return (
    <main className="content" aria-busy="true">
      <section className="rounded-md border border-border bg-muted/40 p-4">
        <div className="h-4 w-36 animate-pulse rounded bg-muted" />
        <div className="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div className="h-20 animate-pulse rounded bg-muted" />
          <div className="h-20 animate-pulse rounded bg-muted" />
          <div className="h-20 animate-pulse rounded bg-muted" />
          <div className="h-20 animate-pulse rounded bg-muted" />
        </div>
      </section>
    </main>
  );
}

export default App;
