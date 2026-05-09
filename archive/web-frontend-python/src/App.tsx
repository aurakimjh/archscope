// ─────────────────────────────────────────────────────────────────────
// [한글] App.tsx — archscope 프론트엔드 최상위 라우팅 컴포넌트.
//
// 책임/목적:
//   - 사이드바에서 선택된 페이지 키(PageKey)에 따라 본문에 렌더할
//     페이지 컴포넌트를 결정합니다(클라이언트 측 단순 스위치 라우팅).
//   - react-router 같은 라이브러리 없이 useState 하나로 활성 페이지를
//     관리합니다(데스크톱/싱글 윈도우 앱이라 URL 라우팅이 필요 없음).
//   - "DemoDataCenter → ExportCenter" 흐름에서 분석 결과의 input 경로
//     를 부모(App)로 끌어올려 두 페이지 간 상태를 전달합니다.
//
// 데이터 흐름:
//   AppShell(좌측 사이드바) → onNavigate(setActivePage) → App
//   → pageComponents[activePage] 또는 DemoDataCenter/ExportCenter 분기.
//
// 주요 컴포넌트:
//   - AppShell: 사이드바 + 헤더 + main slot. shadcn/ui Sheet/Sidebar 기반.
//   - 각 *AnalyzerPage / DashboardPage / ChartStudioPage / SettingsPage:
//     pages/* 폴더에 분리된 페이지 컴포넌트들.
//
// 의존성/주의:
//   - shadcn 테마 토큰과 Tailwind v4 를 가정. ThemeProvider 는 main.tsx
//     에서 감싸므로 여기서는 다루지 않습니다.
//   - JSX.Element 반환은 React 18 타입(global JSX namespace) 의존.
// ─────────────────────────────────────────────────────────────────────
import { useState } from "react";

import { AppShell } from "./components/layout/AppShell";
import { AccessLogAnalyzerPage } from "./pages/AccessLogAnalyzerPage";
import { ChartStudioPage } from "./pages/ChartStudioPage";
import { DashboardPage } from "./pages/DashboardPage";
import { DemoDataCenterPage } from "./pages/DemoDataCenterPage";
import { ExceptionAnalyzerPage } from "./pages/ExceptionAnalyzerPage";
import { ExportCenterPage } from "./pages/ExportCenterPage";
import { GcLogAnalyzerPage } from "./pages/GcLogAnalyzerPage";
import { JfrAnalyzerPage } from "./pages/JfrAnalyzerPage";
import { ProfilerAnalyzerPage } from "./pages/ProfilerAnalyzerPage";
import { SettingsPage } from "./pages/SettingsPage";
import { ThreadDumpAnalyzerPage } from "./pages/ThreadDumpAnalyzerPage";

// [한글] PageKey — 사이드바가 보낼 수 있는 모든 활성 페이지 식별자.
//   슬러그(kebab-case) 그대로 라우트 키로 사용. AppSidebar 내부의
//   네비게이션 링크 정의와 1:1 로 동기화되어야 합니다.
export type PageKey =
  | "dashboard"
  | "access-log"
  | "gc-log"
  | "profiler"
  | "thread-dump"
  | "exception"
  | "jfr"
  | "chart-studio"
  | "demo-data"
  | "export-center"
  | "settings";

// [한글] SimplePageKey — 별도 props 가 필요 없는 페이지(테이블 룩업으로
//   한 줄 렌더 가능한 페이지)만 골라낸 좁힌 타입.
//   demo-data 와 export-center 는 부모로부터 props 를 받아야 하므로
//   pageComponents 매핑에 들어가지 않고 아래 분기에서 처리됩니다.
type SimplePageKey = Exclude<PageKey, "demo-data" | "export-center">;

// [한글] pageComponents — 단순 페이지(props 가 없는 페이지) 의 라우팅
//   테이블. activePage 키로 컴포넌트 함수를 직접 조회하여 렌더합니다.
const pageComponents: Record<SimplePageKey, () => JSX.Element> = {
  dashboard: DashboardPage,
  "access-log": AccessLogAnalyzerPage,
  "gc-log": GcLogAnalyzerPage,
  profiler: ProfilerAnalyzerPage,
  "thread-dump": ThreadDumpAnalyzerPage,
  exception: ExceptionAnalyzerPage,
  jfr: JfrAnalyzerPage,
  "chart-studio": ChartStudioPage,
  settings: SettingsPage,
};

// [한글] App — 최상위 컴포넌트. activePage state 로 어떤 페이지를 보여줄지
//   결정하고, exportInputPath state 로 DemoDataCenter → ExportCenter 간
//   분석 결과 경로를 전달합니다.
//   - state: activePage(PageKey), exportInputPath(string)
//   - effects: 없음(순수 분기 렌더)
//   - children: AppShell 안에 페이지 컴포넌트 1개를 렌더
export function App(): JSX.Element {
  const [activePage, setActivePage] = useState<PageKey>("dashboard");
  const [exportInputPath, setExportInputPath] = useState("");

  // [한글] openExportCenter — DemoDataCenter 페이지에서 "Export" 버튼을
  //   눌렀을 때 호출. 입력 파일 경로를 보존하면서 페이지를 전환합니다.
  function openExportCenter(inputPath: string): void {
    setExportInputPath(inputPath);
    setActivePage("export-center");
  }

  const pageContent =
    activePage === "demo-data" ? (
      <DemoDataCenterPage onOpenExportCenter={openExportCenter} />
    ) : activePage === "export-center" ? (
      <ExportCenterPage initialInputPath={exportInputPath} />
    ) : (
      (() => {
        const PageComponent = pageComponents[activePage];
        return <PageComponent />;
      })()
    );

  return (
    <AppShell activePage={activePage} onNavigate={setActivePage}>
      {pageContent}
    </AppShell>
  );
}
