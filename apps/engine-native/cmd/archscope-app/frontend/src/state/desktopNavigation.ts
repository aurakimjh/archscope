export type NavKey =
  | "profiler"
  | "diff"
  | "access_log"
  | "gc_log"
  | "jfr"
  | "exception"
  | "thread_dump"
  | "msa_profile"
  | "trace_import"
  | "http_capture"
	  | "browser_cpu"
  | "browser_audit"
  | "analysis_workspace"
  | "export_center"
  | "report_diff"
  | "chart_studio"
  | "incident_timeline"
  | "slo_golden_signals"
  | "service_flow"
  | "evidence_board"
  | "settings";


export const DESKTOP_ROUTES = [
  {key: "profiler", labelKey: "navProfiler", group: "analyze"},
  {key: "diff", labelKey: "navDiff", group: "analyze"},
  {key: "access_log", labelKey: "navAccessLog", group: "analyze"},
  {key: "gc_log", labelKey: "navGcLog", group: "analyze"},
  {key: "jfr", labelKey: "navJfr", group: "analyze"},
  {key: "exception", labelKey: "navException", group: "analyze"},
  {key: "thread_dump", labelKey: "navThreadDump", group: "analyze"},
  {key: "msa_profile", labelKey: "navMsaProfile", group: "analyze"},
  {key: "trace_import", labelKey: "navTraceImport", group: "analyze"},
  {key: "http_capture", label: "HTTP capture", group: "analyze"},
  {key: "browser_cpu", label: "Browser CPU", group: "analyze"},
  {key: "browser_audit", labelKey: "navLighthouse", group: "analyze"},
  {key: "analysis_workspace", labelKey: "navAnalysisWorkspace", group: "workspace"},
  {key: "export_center", labelKey: "navExportCenter", group: "workspace"},
  {key: "report_diff", labelKey: "navReportDiff", group: "workspace"},
  {key: "chart_studio", labelKey: "navChartStudio", group: "workspace"},
  {key: "incident_timeline", labelKey: "navIncidentTimeline", group: "workspace"},
  {key: "slo_golden_signals", labelKey: "navSloGoldenSignals", group: "workspace"},
  {key: "service_flow", labelKey: "navServiceFlow", group: "workspace"},
  {key: "evidence_board", labelKey: "navEvidenceBoard", group: "workspace"},
  {key: "settings", labelKey: "navSettings", group: "settings"},
] as const;
