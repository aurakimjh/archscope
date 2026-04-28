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

export function App(): JSX.Element {
  const [activePage, setActivePage] = useState<PageKey>("dashboard");

  return (
    <Layout activePage={activePage} onNavigate={setActivePage}>
      {activePage === "dashboard" && <DashboardPage />}
      {activePage === "access-log" && <AccessLogAnalyzerPage />}
      {activePage === "gc-log" && <GcLogAnalyzerPage />}
      {activePage === "profiler" && <ProfilerAnalyzerPage />}
      {activePage === "thread-dump" && <ThreadDumpAnalyzerPage />}
      {activePage === "exception" && <ExceptionAnalyzerPage />}
      {activePage === "chart-studio" && <ChartStudioPage />}
      {activePage === "export-center" && <ExportCenterPage />}
      {activePage === "settings" && <SettingsPage />}
    </Layout>
  );
}
