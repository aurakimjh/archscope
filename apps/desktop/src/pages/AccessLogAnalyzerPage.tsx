import { ChartPanel } from "../components/ChartPanel";
import { FileDropZone } from "../components/FileDropZone";
import { useI18n } from "../i18n/I18nProvider";

export function AccessLogAnalyzerPage(): JSX.Element {
  const { t } = useI18n();

  return (
    <div className="page">
      <section className="workspace-grid">
        <div className="tool-panel">
          <h2>{t("accessLogAnalyzer")}</h2>
          <FileDropZone
            label={t("selectAccessLogFile")}
            description={t("accessLogFileDescription")}
          />
          <label className="field">
            <span>{t("logFormat")}</span>
            <select defaultValue="nginx">
              <option value="nginx">NGINX</option>
              <option value="apache">Apache</option>
              <option value="ohs">OHS</option>
              <option value="weblogic">WebLogic</option>
              <option value="tomcat">Tomcat</option>
              <option value="custom-regex">Custom Regex</option>
            </select>
          </label>
          <button className="primary-button" type="button">
            {t("analyze")}
          </button>
        </div>
        <div>
          <section className="summary-grid compact">
            <MetricCard label={t("totalRequests")} value="-" />
            <MetricCard label={t("averageResponseTime")} value="-" />
            <MetricCard label={t("p95ResponseTime")} value="-" />
            <MetricCard label={t("errorRate")} value="-" />
          </section>
          <ChartPanel
            title={t("accessLogChartArea")}
            option={{
              xAxis: { type: "category", data: ["10:00", "10:01", "10:02"] },
              yAxis: { type: "value" },
              series: [{ type: "bar", data: [3, 2, 1] }],
            }}
          />
        </div>
      </section>
    </div>
  );
}

function MetricCard({ label, value }: { label: string; value: string }): JSX.Element {
  return (
    <div className="metric-card">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}
