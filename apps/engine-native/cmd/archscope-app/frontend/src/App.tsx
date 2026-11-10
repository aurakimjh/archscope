import { Events } from "@wailsio/runtime";
import * as CaptureService from "../bindings/github.com/aurakimjh/archscope/apps/engine-native/cmd/archscope-app/captureservice";
// 네이티브 메뉴가 화면 탐색을 담당하고, 방문한 도구의 상태는 숨긴 패널에 유지한다.
import { lazy, Suspense, useState, useEffect } from "react";

import { useI18n } from "./i18n/I18nProvider";

import { useTheme, type Theme } from "./theme/ThemeProvider";
import { useSortableTables } from "./hooks/useSortableTables";
import { DropZone } from "./components/DropZone";
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

// [한글] App: 사이드바 선택(active) 에 따라 9개 분석기/유틸 페이지 중
// 하나를 마운트하는 컨테이너. 페이지 전환 시 이전 페이지의 분석 결과는
// (의도적으로) 폐기되어 메모리/렌더 부하를 가볍게 유지합니다.
function App() {
  const { locale, setLocale, t } = useI18n();
  const { theme, setTheme } = useTheme();
  const [active, setActive] = useState<NavKey>("profiler");
  const [visited, setVisited] = useState<NavKey[]>(["profiler"]);
  const [about, setAbout] = useState(false);
  const [help, setHelp] = useState(false);
  const [canOpen, setCanOpen] = useState(false);
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
  const routes = DESKTOP_ROUTES.map(route => ({...route, label: "labelKey" in route ? t(route.labelKey) : route.key === "http_capture" ? (locale === "ko" ? "HTTP 캡처" : "HTTP Capture") : (locale === "ko" ? "브라우저 CPU" : "Browser CPU")}));
  useEffect(() => {setVisited(current => current.includes(active) ? current : [...current, active]);}, [active]);
  useEffect(() => {
    const update = () => setCanOpen(!!document.querySelector(`[data-route="${active}"] [data-open-file]:not([disabled])`));
    const observer = new MutationObserver(update);
    observer.observe(document.getElementById("root")!, {childList:true,subtree:true,attributes:true,attributeFilter:["disabled"]});
    update();return () => observer.disconnect();
  }, [active]);
  useEffect(() => {
    void Events.Emit("desktop:state", {locale,theme,active,canOpen,routes});
  }, [locale,theme,active,canOpen]);
  useEffect(() => Events.On("desktop:command", event => {
    const command = (event.data as {command?:string})?.command ?? "";
    const [action,value] = command.split(":");
    if (action === "navigate" && DESKTOP_ROUTES.some(route => route.key === value)) setActive(value as NavKey);
    else if (action === "locale" && (value === "en" || value === "ko")) setLocale(value);
    else if (action === "theme" && ["light","dark","system"].includes(value)) setTheme(value as Theme);
    else if (action === "about") setAbout(true);
    else if (action === "open") document.querySelector<HTMLButtonElement>(`[data-route="${active}"] [data-open-file]:not([disabled])`)?.click();
    else if (action === "help") {
      if (active === "http_capture") void Events.Emit("capture:command",{command:"help"});
      else { const trigger = document.querySelector<HTMLButtonElement>(`[data-route="${active}"] [data-help-trigger]`); if(trigger) trigger.click();else setHelp(true); }
    }
  }), [active,setLocale,setTheme]);
  useEffect(() => {
    const navigate = (event: Event) => setActive((event as CustomEvent<NavKey>).detail);
    window.addEventListener("desktop:navigate",navigate);
    return () => window.removeEventListener("desktop:navigate",navigate);
  }, []);
  useSortableTables();

  const handleDrop = (path: string) => {
    // Drop forwards into whichever page is active. Each page owns its own
    // file state — the global drop is just an attention-grabber for the
    // primary surface (profiler) where users paste path strings most often.
    if (active === "profiler") {
      // ProfilerAnalyzerPage subscribes to its own Wails dialog; we don't
      // currently broadcast the path through. Future enhancement.
      void path;
    }
  };

  return (
    <DropZone onDrop={handleDrop}>
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
            {(visited.includes("profiler") || active === "profiler") && <div className="tool-pane" data-route="profiler" hidden={active!=="profiler"} inert={active!=="profiler"}><ProfilerAnalyzerPage /></div>}
            {(visited.includes("diff") || active === "diff") && <div className="tool-pane" data-route="diff" hidden={active!=="diff"} inert={active!=="diff"}><DiffPage /></div>}
            {(visited.includes("access_log") || active === "access_log") && <div className="tool-pane" data-route="access_log" hidden={active!=="access_log"} inert={active!=="access_log"}><AccessLogAnalyzerPage /></div>}
            {(visited.includes("gc_log") || active === "gc_log") && <div className="tool-pane" data-route="gc_log" hidden={active!=="gc_log"} inert={active!=="gc_log"}><GcLogAnalyzerPage /></div>}
            {(visited.includes("jfr") || active === "jfr") && <div className="tool-pane" data-route="jfr" hidden={active!=="jfr"} inert={active!=="jfr"}><JfrAnalyzerPage /></div>}
            {(visited.includes("exception") || active === "exception") && <div className="tool-pane" data-route="exception" hidden={active!=="exception"} inert={active!=="exception"}><ExceptionAnalyzerPage /></div>}
            {(visited.includes("thread_dump") || active === "thread_dump") && <div className="tool-pane" data-route="thread_dump" hidden={active!=="thread_dump"} inert={active!=="thread_dump"}><ThreadDumpAnalyzerPage /></div>}
            {(visited.includes("msa_profile") || active === "msa_profile") && <div className="tool-pane" data-route="msa_profile" hidden={active!=="msa_profile"} inert={active!=="msa_profile"}><JenniferProfilePage /></div>}
            {(visited.includes("trace_import") || active === "trace_import") && <div className="tool-pane" data-route="trace_import" hidden={active!=="trace_import"} inert={active!=="trace_import"}><TraceImportPage /></div>}
            {(visited.includes("http_capture") || active === "http_capture") && <div className="tool-pane" data-route="http_capture" hidden={active!=="http_capture"} inert={active!=="http_capture"}><HttpCapturePage /></div>}
            {(visited.includes("browser_cpu") || active === "browser_cpu") && <div className="tool-pane" data-route="browser_cpu" hidden={active!=="browser_cpu"} inert={active!=="browser_cpu"}><BrowserCpuProfilePage /></div>}
            {(visited.includes("browser_audit") || active === "browser_audit") && <div className="tool-pane" data-route="browser_audit" hidden={active!=="browser_audit"} inert={active!=="browser_audit"}><LighthouseAuditPage /></div>}
            {(visited.includes("analysis_workspace") || active === "analysis_workspace") && <div className="tool-pane" data-route="analysis_workspace" hidden={active!=="analysis_workspace"} inert={active!=="analysis_workspace"}><AnalysisWorkspacePage /></div>}
            {(visited.includes("export_center") || active === "export_center") && <div className="tool-pane" data-route="export_center" hidden={active!=="export_center"} inert={active!=="export_center"}><ExportCenterPage /></div>}
            {(visited.includes("report_diff") || active === "report_diff") && <div className="tool-pane" data-route="report_diff" hidden={active!=="report_diff"} inert={active!=="report_diff"}><ReportDiffPage /></div>}
            {(visited.includes("chart_studio") || active === "chart_studio") && <div className="tool-pane" data-route="chart_studio" hidden={active!=="chart_studio"} inert={active!=="chart_studio"}><ChartStudioPage /></div>}
            {(visited.includes("incident_timeline") || active === "incident_timeline") && <div className="tool-pane" data-route="incident_timeline" hidden={active!=="incident_timeline"} inert={active!=="incident_timeline"}><IncidentTimelinePage /></div>}
            {(visited.includes("slo_golden_signals") || active === "slo_golden_signals") && <div className="tool-pane" data-route="slo_golden_signals" hidden={active!=="slo_golden_signals"} inert={active!=="slo_golden_signals"}><SloGoldenSignalsPage /></div>}
            {(visited.includes("service_flow") || active === "service_flow") && <div className="tool-pane" data-route="service_flow" hidden={active!=="service_flow"} inert={active!=="service_flow"}><ServiceFlowPage /></div>}
            {(visited.includes("evidence_board") || active === "evidence_board") && <div className="tool-pane" data-route="evidence_board" hidden={active!=="evidence_board"} inert={active!=="evidence_board"}><EvidenceBoardPage /></div>}
            {(visited.includes("settings") || active === "settings") && <div className="tool-pane" data-route="settings" hidden={active!=="settings"} inert={active!=="settings"}><SettingsPage /></div>}
          </Suspense>
        </div>
      </div>
      <AboutDialog open={about} onClose={() => setAbout(false)} />
      <SlideOverPanel open={help} onClose={() => setHelp(false)} title={routes.find(route => route.key===active)?.label ?? "ArchScope"}>
        <p>{getHelpText(locale,"fileDock")}</p>
      </SlideOverPanel>
    </DropZone>
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
