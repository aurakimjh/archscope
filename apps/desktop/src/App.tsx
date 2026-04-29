import { useState } from "react";

import { Layout } from "./components/Layout";
import { AccessLogAnalyzerPage } from "./pages/AccessLogAnalyzerPage";
import { ChartStudioPage } from "./pages/ChartStudioPage";
import { DashboardPage } from "./pages/DashboardPage";
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
  | "export-center"
  | "settings";

const pageComponents: Record<PageKey, () => JSX.Element> = {
  dashboard: DashboardPage,
  "access-log": AccessLogAnalyzerPage,
  "gc-log": GcLogAnalyzerPage,
  profiler: ProfilerAnalyzerPage,
  "thread-dump": ThreadDumpAnalyzerPage,
  exception: ExceptionAnalyzerPage,
  "chart-studio": ChartStudioPage,
  "export-center": ExportCenterPage,
  settings: SettingsPage,
};

export function App(): JSX.Element {
  const [activePage, setActivePage] = useState<PageKey>("dashboard");
  const PageComponent = pageComponents[activePage];

  return (
    <Layout activePage={activePage} onNavigate={setActivePage}>
      <PageComponent />
    </Layout>
  );
}
