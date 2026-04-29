import { describe, expect, it } from "vitest";

import type {
  AccessLogAnalysisResult,
  ProfilerCollapsedAnalysisResult,
} from "../api/analyzerClient";
import { sampleAnalysisResult } from "./sampleCharts";
import { createChartOption } from "./chartFactory";
import { chartTemplates, dashboardChartTemplateIds } from "./chartTemplates";
import type { ChartLabels } from "./chartOptions";

const labels: ChartLabels = {
  requestsAxis: "Requests",
  millisecondsAxis: "ms",
  statusSeries: "Status",
  samplesAxis: "Samples",
  p95Series: "p95",
};

describe("chart factory", () => {
  it("creates dashboard chart options for registered templates", () => {
    for (const templateId of dashboardChartTemplateIds) {
      const option = createChartOption(templateId, sampleAnalysisResult, labels);
      expect(option.series).toBeDefined();
    }
  });

  it("creates access-log request chart options from real analyzer data", () => {
    const option = createChartOption(
      "AccessLog.RequestCountTrend",
      accessLogResult,
      labels,
    );

    expect(option.xAxis).toMatchObject({ type: "category" });
    expect(option.series).toEqual([
      expect.objectContaining({
        name: "Requests",
        data: [2],
      }),
    ]);
  });

  it("creates profiler component chart options from real analyzer data", () => {
    const option = createChartOption(
      "Profiler.ComponentBreakdown",
      profilerResult,
      labels,
    );

    expect(option.yAxis).toMatchObject({
      type: "category",
      data: ["Application"],
    });
    expect(option.series).toEqual([
      expect.objectContaining({
        name: "Samples",
        data: [10],
      }),
    ]);
  });

  it("keeps every template connected to a dashboard catalog entry or factory id", () => {
    expect(chartTemplates.map((template) => template.id)).toEqual(
      expect.arrayContaining(dashboardChartTemplateIds),
    );
  });
});

const accessLogResult: AccessLogAnalysisResult = {
  type: "access_log",
  source_files: ["access.log"],
  created_at: "2026-04-30T00:00:00Z",
  summary: {
    total_requests: 2,
    avg_response_ms: 120,
    p95_response_ms: 140,
    p99_response_ms: 150,
    error_rate: 0,
  },
  series: {
    requests_per_minute: [{ time: "10:00", value: 2 }],
    avg_response_time_per_minute: [{ time: "10:00", value: 120 }],
    p95_response_time_per_minute: [{ time: "10:00", value: 140 }],
    status_code_distribution: [{ status: "2xx", count: 2 }],
    top_urls_by_count: [{ uri: "/", count: 2 }],
    top_urls_by_avg_response_time: [{ uri: "/", avg_response_ms: 120, count: 2 }],
  },
  tables: { sample_records: [] },
  charts: {},
  metadata: {
    format: "nginx",
    parser: "nginx_combined_with_response_time",
    schema_version: "0.1.0",
    diagnostics: {
      total_lines: 2,
      parsed_records: 2,
      skipped_lines: 0,
      skipped_by_reason: {},
      samples: [],
    },
    analysis_options: {
      max_lines: null,
      start_time: null,
      end_time: null,
    },
    findings: [],
  },
};

const profilerResult: ProfilerCollapsedAnalysisResult = {
  type: "profiler_collapsed",
  source_files: ["wall.collapsed"],
  created_at: "2026-04-30T00:00:00Z",
  summary: {
    profile_kind: "wall",
    total_samples: 10,
    interval_ms: 100,
    estimated_seconds: 1,
    elapsed_seconds: null,
  },
  series: {
    top_stacks: [],
    component_breakdown: [{ component: "Application", samples: 10 }],
  },
  tables: { top_stacks: [] },
  charts: {},
  metadata: {
    parser: "async_profiler_collapsed",
    schema_version: "0.1.0",
    diagnostics: {
      total_lines: 1,
      parsed_records: 1,
      skipped_lines: 0,
      skipped_by_reason: {},
      samples: [],
    },
  },
};
