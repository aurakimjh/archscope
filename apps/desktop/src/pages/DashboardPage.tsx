import { useEffect, useMemo, useState } from "react";

import { loadSampleAnalysisResult, type DashboardSampleResult } from "../api/analyzerClient";
import { createChartOption } from "../charts/chartFactory";
import { dashboardChartTemplateIds, getChartTemplate } from "../charts/chartTemplates";
import { registerArchScopeTheme } from "../charts/echartsTheme";
import { ChartPanel } from "../components/ChartPanel";
import { MetricCard } from "../components/MetricCard";
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
        {chartOptions.map((chart) => (
          <ChartPanel key={chart.id} title={chart.title} option={chart.option} />
        ))}
      </section>
    </div>
  );
}
