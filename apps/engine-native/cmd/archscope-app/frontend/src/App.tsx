// ─────────────────────────────────────────────────────────────────────
// [한글] App.tsx — Wails 데스크톱 프로파일러의 최상위 셸 컴포넌트.
//
// 책임/목적:
//   - 좌측 Sidebar 의 NavKey 상태(active) 를 보유하고, 그에 맞는
//     분석기 페이지(*.AnalyzerPage) 를 우측 영역에 렌더.
//   - 상단 topbar 에서 테마(light/dark/system), 로케일(en/ko) 토글
//     컨트롤을 노출. 실제 상태는 ThemeProvider / I18nProvider 가 보관.
//   - 전체 화면을 DropZone 으로 감싸 OS-level 파일 드롭을 받지만,
//     현재 구현은 path 를 단순히 `void` 처리하고 각 페이지가 자체
//     Wails 다이얼로그를 통해 파일을 선택. (향후: 드롭 path 를 활성
//     페이지로 브로드캐스트할 수 있는 채널 도입 예정.)
//
// 데이터 흐름:
//   - Go EngineService 와의 통신은 여기서는 일어나지 않음.
//     각 *AnalyzerPage 가 bridge/engine.ts 를 통해 Wails Call.ByName
//     으로 분석을 트리거하고 결과(AnalysisResult) 를 받아 차트로 렌더.
//   - active 상태가 바뀌면 해당 페이지가 새로 마운트되며, 페이지 로컬
//     state(파일 경로, 결과 캐시 등)는 페이지 unmount 시 사라짐.
//
// 주요 페이지: profiler / diff / access_log / gc_log / jfr /
//   exception / thread_dump / msa_profile (Jennifer) / settings.
// ─────────────────────────────────────────────────────────────────────
import { lazy, Suspense, useState } from "react";

import { useI18n } from "./i18n/I18nProvider";
import { localeLabels, locales } from "./i18n/messages";
import { useTheme, type Theme } from "./theme/ThemeProvider";
import { useSortableTables } from "./hooks/useSortableTables";
import { DropZone } from "./components/DropZone";
import { HelpTip } from "./components/HelpTip";
import { Sidebar, type NavKey } from "./components/Sidebar";
import { getHelpText } from "./help/helpCatalog";

const THEME_OPTIONS: Theme[] = ["light", "dark", "system"];

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
  import("./pages/ThreadDumpAnalyzerPage").then(({ ThreadDumpAnalyzerPage }) => ({
    default: ThreadDumpAnalyzerPage,
  })),
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
  import("./pages/HttpCapturePage").then(({ HttpCapturePage }) => ({ default: HttpCapturePage })),
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
      <div className="app-shell with-sidebar">
        <Sidebar active={active} onNavigate={setActive} />
        <div className="app-main">
          <header className="topbar">
            <div className="topbar-controls">
              <div className="locale-switch" role="group" aria-label={t("themeLabel")}>
                <HelpTip
                  text={getHelpText(locale, "themeControl")}
                  className="ml-0.5 mr-1 self-center"
                />
                {THEME_OPTIONS.map((value) => (
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
              <div className="locale-switch" role="group" aria-label="Locale">
                <HelpTip
                  text={getHelpText(locale, "localeControl")}
                  className="ml-0.5 mr-1 self-center"
                />
                {locales.map((value) => (
                  <button
                    key={value}
                    type="button"
                    className={value === locale ? "locale-pill active" : "locale-pill"}
                    onClick={() => setLocale(value)}
                    aria-pressed={value === locale}
                    title={localeLabels[value]}
                  >
                    {value.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>
          </header>

          <Suspense fallback={<PageFallback />}>
            {active === "profiler" && <ProfilerAnalyzerPage />}
            {active === "diff" && <DiffPage />}
            {active === "access_log" && <AccessLogAnalyzerPage />}
            {active === "gc_log" && <GcLogAnalyzerPage />}
            {active === "jfr" && <JfrAnalyzerPage />}
            {active === "exception" && <ExceptionAnalyzerPage />}
            {active === "thread_dump" && <ThreadDumpAnalyzerPage />}
            {active === "msa_profile" && <JenniferProfilePage />}
            {active === "trace_import" && <TraceImportPage />}
            {active === "http_capture" && <HttpCapturePage />}
            {active === "analysis_workspace" && <AnalysisWorkspacePage />}
            {active === "export_center" && <ExportCenterPage />}
            {active === "report_diff" && <ReportDiffPage />}
            {active === "chart_studio" && <ChartStudioPage />}
            {active === "incident_timeline" && <IncidentTimelinePage />}
            {active === "slo_golden_signals" && <SloGoldenSignalsPage />}
            {active === "service_flow" && <ServiceFlowPage />}
            {active === "evidence_board" && <EvidenceBoardPage />}
            {active === "settings" && <SettingsPage />}
          </Suspense>
        </div>
      </div>
    </DropZone>
  );
}

function PageFallback(): JSX.Element {
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
