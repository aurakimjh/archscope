import { useEffect, useMemo, useState } from "react";

import { loadSampleAnalysisResult, type DashboardSampleResult } from "../api/analyzerClient";
import { ChartPanel } from "../components/ChartPanel";
import { MetricCard } from "../components/MetricCard";
import {
  p95TrendOption,
  profilerBreakdownOption,
  requestCountTrendOption,
  statusCodeDistributionOption,
} from "../charts/chartOptions";
import { registerArchScopeTheme } from "../charts/echartsTheme";
import { useI18n } from "../i18n/I18nProvider";

export function DashboardPage(): JSX.Element {
  const [data, setData] = useState<DashboardSampleResult | null>(null);
  const { t } = useI18n();

  useEffect(() => {
    registerArchScopeTheme();
    void loadSampleAnalysisResult().then(setData);
  }, []);

  const chartOptions = useMemo(() => {
    if (!data) {
      return null;
    }
    const labels = {
      requestsAxis: t("requestsAxis"),
      millisecondsAxis: t("millisecondsAxis"),
      statusSeries: t("statusSeries"),
      samplesAxis: t("samplesAxis"),
    };
    return {
      requestCount: requestCountTrendOption(data, labels),
      p95: p95TrendOption(data, labels),
      status: statusCodeDistributionOption(data, labels),
      profiler: profilerBreakdownOption(data, labels),
    };
  }, [data, t]);

  if (!data || !chartOptions) {
    return <div className="page">{t("loadingSample")}</div>;
  }

  return (
    <div className="page">
      <section className="summary-grid">
        <MetricCard label={t("totalRequests")} value={data.summary.total_requests} />
        <MetricCard
          label={t("averageResponseTime")}
          value={`${data.summary.avg_response_ms} ms`}
        />
        <MetricCard
          label={t("p95ResponseTime")}
          value={`${data.summary.p95_response_ms} ms`}
        />
        <MetricCard label={t("errorRate")} value={`${data.summary.error_rate}%`} />
      </section>
      <section className="chart-grid">
        <ChartPanel
          title={t("requestCountTrend")}
          option={chartOptions.requestCount}
        />
        <ChartPanel
          title={t("responseTimeP95Trend")}
          option={chartOptions.p95}
        />
        <ChartPanel
          title={t("statusCodeDistribution")}
          option={chartOptions.status}
        />
        <ChartPanel
          title={t("profilerComponentBreakdown")}
          option={chartOptions.profiler}
        />
      </section>
    </div>
  );
}
