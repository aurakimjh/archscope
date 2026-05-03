import { useEffect, useMemo, useState } from "react";

import type { DashboardSampleResult } from "../api/analyzerClient";
import { createChartOption } from "../charts/chartFactory";
import { dashboardChartTemplateIds, getChartTemplate } from "../charts/chartTemplates";
import { ChartPanel } from "../components/ChartPanel";
import { EmptyState } from "../components/LoadingStates";
import { MetricCard } from "../components/MetricCard";
import { useI18n } from "../i18n/I18nProvider";

const DASHBOARD_STORAGE_KEY = "archscope.dashboard.lastResult";

export function DashboardPage(): JSX.Element {
  const [data, setData] = useState<DashboardSampleResult | null>(null);
  const { t } = useI18n();

  useEffect(() => {
    const saved = window.localStorage.getItem(DASHBOARD_STORAGE_KEY);
    if (saved) {
      try {
        setData(JSON.parse(saved) as DashboardSampleResult);
      } catch {
        // ignore
      }
    }
  }, []);

  const chartOptions = useMemo(() => {
    if (!data) return null;
    const labels = {
      requestsAxis: t("requestsAxis"),
      millisecondsAxis: t("millisecondsAxis"),
      statusSeries: t("statusSeries"),
      samplesAxis: t("samplesAxis"),
      p95Series: t("p95Series"),
    };
    return dashboardChartTemplateIds.map((templateId) => {
      const template = getChartTemplate(templateId);
      return {
        id: template.id,
        title: t(template.titleKey),
        option: createChartOption(template.id, data, labels),
      };
    });
  }, [data, t]);

  if (!data || !chartOptions) {
    return (
      <div className="page">
        <EmptyState
          icon="chart"
          message={t("dashboardEmpty")}
        />
      </div>
    );
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
        {chartOptions.map((chart) => (
          <ChartPanel key={chart.id} title={chart.title} option={chart.option} />
        ))}
      </section>
    </div>
  );
}

export function saveDashboardResult(result: DashboardSampleResult): void {
  try {
    window.localStorage.setItem(DASHBOARD_STORAGE_KEY, JSON.stringify(result));
  } catch {
    // storage full, ignore
  }
}
