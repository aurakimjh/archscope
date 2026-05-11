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
import { useState } from "react";

import { useI18n } from "./i18n/I18nProvider";
import { localeLabels, locales } from "./i18n/messages";
import { useTheme, type Theme } from "./theme/ThemeProvider";
import { useSortableTables } from "./hooks/useSortableTables";
import { DropZone } from "./components/DropZone";
import { Sidebar, type NavKey } from "./components/Sidebar";
import { DiffPage } from "./pages/DiffPage";
import { SettingsPage } from "./pages/SettingsPage";
import { AccessLogAnalyzerPage } from "./pages/AccessLogAnalyzerPage";
import { GcLogAnalyzerPage } from "./pages/GcLogAnalyzerPage";
import { JfrAnalyzerPage } from "./pages/JfrAnalyzerPage";
import { ExceptionAnalyzerPage } from "./pages/ExceptionAnalyzerPage";
import { ThreadDumpAnalyzerPage } from "./pages/ThreadDumpAnalyzerPage";
import { ProfilerAnalyzerPage } from "./pages/ProfilerAnalyzerPage";
import { JenniferProfilePage } from "./pages/JenniferProfilePage";

const THEME_OPTIONS: Theme[] = ["light", "dark", "system"];

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

          {active === "profiler" && <ProfilerAnalyzerPage />}
          {active === "diff" && <DiffPage />}
          {active === "access_log" && <AccessLogAnalyzerPage />}
          {active === "gc_log" && <GcLogAnalyzerPage />}
          {active === "jfr" && <JfrAnalyzerPage />}
          {active === "exception" && <ExceptionAnalyzerPage />}
          {active === "thread_dump" && <ThreadDumpAnalyzerPage />}
          {active === "msa_profile" && <JenniferProfilePage />}
          {active === "settings" && <SettingsPage />}
        </div>
      </div>
    </DropZone>
  );
}

export default App;
