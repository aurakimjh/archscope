import { useState } from "react";

import { Layout } from "./components/Layout";
import { AccessLogAnalyzerPage } from "./pages/AccessLogAnalyzerPage";
import { ChartStudioPage } from "./pages/ChartStudioPage";
import { DashboardPage } from "./pages/DashboardPage";
import { DemoDataCenterPage } from "./pages/DemoDataCenterPage";
import { ExceptionAnalyzerPage } from "./pages/ExceptionAnalyzerPage";
import { ExportCenterPage } from "./pages/ExportCenterPage";
import { GcLogAnalyzerPage } from "./pages/GcLogAnalyzerPage";
import { ProfilerAnalyzerPage } from "./pages/ProfilerAnalyzerPage";
import { SettingsPage } from "./pages/SettingsPage";
import { ThreadDumpAnalyzerPage } from "./pages/ThreadDumpAnalyzerPage";

export type PageKey =
  | "dashboard"
  | "access-log"
  | "gc-log"
  | "profiler"
  | "thread-dump"
  | "exception"
  | "chart-studio"
  | "demo-data"
  | "export-center"
  | "settings";

type SimplePageKey = Exclude<PageKey, "demo-data" | "export-center">;

const pageComponents: Record<SimplePageKey, () => JSX.Element> = {
  dashboard: DashboardPage,
  "access-log": AccessLogAnalyzerPage,
  "gc-log": GcLogAnalyzerPage,
  profiler: ProfilerAnalyzerPage,
  "thread-dump": ThreadDumpAnalyzerPage,
  exception: ExceptionAnalyzerPage,
  "chart-studio": ChartStudioPage,
  settings: SettingsPage,
};

export function App(): JSX.Element {
  const [activePage, setActivePage] = useState<PageKey>("dashboard");
  const [exportInputPath, setExportInputPath] = useState("");

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
    <Layout activePage={activePage} onNavigate={setActivePage}>
      {pageContent}
    </Layout>
  );
}
