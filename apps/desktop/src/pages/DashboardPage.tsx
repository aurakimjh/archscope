import { useEffect, useMemo, useState } from "react";

import { loadSampleAnalysisResult, type SampleAnalysisResult } from "../api/analyzerClient";
import { ChartPanel } from "../components/ChartPanel";
import {
  p95TrendOption,
  profilerBreakdownOption,
  requestCountTrendOption,
  statusCodeDistributionOption,
} from "../charts/chartOptions";
import { registerArchScopeTheme } from "../charts/echartsTheme";

export function DashboardPage(): JSX.Element {
  const [data, setData] = useState<SampleAnalysisResult | null>(null);

  useEffect(() => {
    registerArchScopeTheme();
    void loadSampleAnalysisResult().then(setData);
  }, []);

  const chartOptions = useMemo(() => {
    if (!data) {
      return null;
    }
    return {
      requestCount: requestCountTrendOption(data),
      p95: p95TrendOption(data),
      status: statusCodeDistributionOption(data),
      profiler: profilerBreakdownOption(data),
    };
  }, [data]);

  if (!data || !chartOptions) {
    return <div className="page">Loading sample analysis result...</div>;
  }

  return (
    <div className="page">
      <section className="summary-grid">
        <MetricCard label="Total Requests" value={data.summary.total_requests} />
        <MetricCard
          label="Average Response Time"
          value={`${data.summary.avg_response_ms} ms`}
        />
        <MetricCard
          label="p95 Response Time"
          value={`${data.summary.p95_response_ms} ms`}
        />
        <MetricCard label="Error Rate" value={`${data.summary.error_rate}%`} />
      </section>
      <section className="chart-grid">
        <ChartPanel
          title="Request Count Trend"
          option={chartOptions.requestCount}
        />
        <ChartPanel
          title="Response Time p95 Trend"
          option={chartOptions.p95}
        />
        <ChartPanel
          title="Status Code Distribution"
          option={chartOptions.status}
        />
        <ChartPanel
          title="Profiler Component Breakdown"
          option={chartOptions.profiler}
        />
      </section>
    </div>
  );
}

function MetricCard({
  label,
  value,
}: {
  label: string;
  value: string | number;
}): JSX.Element {
  return (
    <div className="metric-card">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
